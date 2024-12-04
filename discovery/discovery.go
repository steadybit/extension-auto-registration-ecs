package discovery

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	extensionconfig "github.com/steadybit/extension-discovery-ecs/config"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
)

var (
	hostIpCache map[string]string
)

func UpdateAgentExtensions(httpClient *resty.Client, ecsClient *ecs.Client, ec2Client *ec2.Client) {
	currentRegistrations, err := getCurrentRegistrations(httpClient)
	if err == nil {
		discoveredExtensions := discoverExtensions(ecsClient, ec2Client)
		syncRegistrations(httpClient, &currentRegistrations, &discoveredExtensions)
	}
}

func getCurrentRegistrations(httpClient *resty.Client) ([]extensionConfigAO, error) {
	var currentRegistrations *[]extensionConfigAO
	resp, err := httpClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&currentRegistrations).
		Get("/extensions")

	if err != nil {
		log.Error().Err(err).Msg("Failed to get extension registrations from the agent. Skip Discovery.")
		return nil, err
	}
	if resp.IsError() {
		log.Error().Msgf("Failed to get extension registrations from the agent: %s. Skip Discovery.", resp.Status())
		return nil, fmt.Errorf("failed to get extension registrations from the agent: %s", resp.Status())
	}
	if resp.IsSuccess() {
		log.Debug().Int("count", len(*currentRegistrations)).Msg("Got extension registrations from the agent")
	}
	return *currentRegistrations, nil
}

func discoverExtensions(ecsClient *ecs.Client, ec2Client *ec2.Client) []extensionConfigAO {
	discoveredExtensions := make([]extensionConfigAO, 0)
	for _, taskFamily := range extensionconfig.Config.TaskFamilies {
		listTasksOutput, err := ecsClient.ListTasks(context.TODO(), &ecs.ListTasksInput{
			Cluster:       &extensionconfig.Config.EcsClusterName,
			DesiredStatus: types.DesiredStatusRunning,
			Family:        &taskFamily,
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list tasks. No extensions discovered.")
			return discoveredExtensions
		}
		if len(listTasksOutput.TaskArns) > 0 {
			describeTasksOutput, err := ecsClient.DescribeTasks(context.TODO(), &ecs.DescribeTasksInput{
				Cluster: &extensionconfig.Config.EcsClusterName,
				Tasks:   listTasksOutput.TaskArns,
				Include: []types.TaskField{types.TaskFieldTags},
			})
			if err != nil {
				log.Warn().Err(err).Msg("Failed to describe tasks. No extensions discovered.")
				return discoveredExtensions
			}
			for _, task := range describeTasksOutput.Tasks {
				portTag := getTagValue(task.Tags, "steadybit_extension_port")
				if portTag == nil {
					log.Warn().Msgf("Task: %s %s - Tag 'steadybit_extension_port' not found. Ignore.", *task.Group, *task.TaskArn)
					continue
				}
				typesTag := getTagValue(task.Tags, "steadybit_extension_type")
				if typesTag == nil {
					log.Warn().Msgf("Task: %s %s - Tag 'steadybit_extension_type' not found. Ignore.", *task.Group, *task.TaskArn)
					continue
				}
				daemonTag := getTagValue(task.Tags, "steadybit_extension_daemon")

				var ip *string
				if daemonTag != nil && *daemonTag == "true" {
					ip = getHostIp(*task.ContainerInstanceArn, ecsClient, ec2Client)
				} else if len(task.Containers[0].NetworkInterfaces) > 0 {
					ip = task.Containers[0].NetworkInterfaces[0].PrivateIpv4Address
				}
				if ip != nil {
					typesArray := strings.Split(*typesTag, ":")
					discoveredExtensions = append(discoveredExtensions, extensionConfigAO{
						Url:   "http://" + *ip + ":" + *portTag,
						Types: typesArray,
					})
					log.Debug().Msgf("Discovered Task: %s - %s:%s - %v", *task.Group, *ip, *portTag, typesArray)
				} else {
					log.Warn().Msgf("Task: %s %s - No IP/Port found. Ignore.", *task.Group, *task.TaskArn)
				}
			}
		}
	}
	return discoveredExtensions
}

func syncRegistrations(httpClient *resty.Client, currentRegistrations *[]extensionConfigAO, discoveredExtensions *[]extensionConfigAO) {
	removeMissingRegistrations(httpClient, currentRegistrations, discoveredExtensions)
	addNewRegistrations(httpClient, currentRegistrations, discoveredExtensions)
}

func removeMissingRegistrations(httpClient *resty.Client, currentRegistrations *[]extensionConfigAO, discoveredExtensions *[]extensionConfigAO) {
	for _, currentRegistration := range *currentRegistrations {
		found := false
		for _, discoveredExtension := range *discoveredExtensions {
			if currentRegistration.Url == discoveredExtension.Url {
				found = true
				break
			}
		}
		if !found {
			resp, err := httpClient.R().
				SetHeader("Content-Type", "application/json").
				SetBasicAuth("_", extensionconfig.Config.AgentKey).
				SetBody(currentRegistration).
				Delete("/extensions")
			if err != nil {
				log.Error().Err(err).Msgf("Failed to remove extension: %s", currentRegistration.Url)
			}
			if resp.IsError() {
				log.Error().Msgf("Failed to remove extension: %s. Status: %s", currentRegistration.Url, resp.Status())
			}
			if resp.IsSuccess() {
				log.Info().Msgf("Removed extension: %s", currentRegistration.Url)
			}
		}
	}
}

func addNewRegistrations(httpClient *resty.Client, currentRegistrations *[]extensionConfigAO, discoveredExtensions *[]extensionConfigAO) {
	for _, discoveredExtension := range *discoveredExtensions {
		found := false
		for _, currentRegistration := range *currentRegistrations {
			if currentRegistration.Url == discoveredExtension.Url {
				found = true
				break
			}
		}
		if !found {
			resp, err := httpClient.R().
				SetHeader("Content-Type", "application/json").
				SetBasicAuth("_", extensionconfig.Config.AgentKey).
				SetBody(discoveredExtension).
				Post("/extensions")
			if err != nil {
				log.Error().Err(err).Msgf("Failed to add extension: %s", discoveredExtension.Url)
			}
			if resp.IsError() {
				log.Error().Msgf("Failed to add extension: %s. Status: %s", discoveredExtension.Url, resp.Status())
			}
			if resp.IsSuccess() {
				log.Info().Msgf("Added extension: %s", discoveredExtension.Url)
			}
		}
	}
}

func getTagValue(tags []types.Tag, key string) *string {
	for _, tag := range tags {
		if *tag.Key == key {
			return tag.Value
		}
	}
	return nil
}

func getHostIp(containerInstanceArn string, ecsClient *ecs.Client, ec2Client *ec2.Client) *string {
	if hostIpCache == nil {
		hostIpCache = make(map[string]string)
	}
	ip, ok := hostIpCache[containerInstanceArn]
	if !ok {
		containerInstance, err := ecsClient.DescribeContainerInstances(context.TODO(), &ecs.DescribeContainerInstancesInput{
			Cluster:            &extensionconfig.Config.EcsClusterName,
			ContainerInstances: []string{containerInstanceArn},
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to describe container instances.")
			return nil
		}
		instanceId := containerInstance.ContainerInstances[0].Ec2InstanceId
		describeInstancesOutput, err := ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			InstanceIds: []string{*instanceId},
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to describe ec2 instance.")
			return nil
		}
		if (len(describeInstancesOutput.Reservations) == 0) || (len(describeInstancesOutput.Reservations[0].Instances) == 0) {
			return nil
		}
		ip = *describeInstancesOutput.Reservations[0].Instances[0].PrivateIpAddress
		hostIpCache[containerInstanceArn] = ip
		return extutil.Ptr(ip)
	} else {
		return extutil.Ptr(ip)
	}
}
