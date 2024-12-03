package discovery

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	extensionconfig "github.com/steadybit/extension-discovery-ecs/config"
	"github.com/steadybit/extension-kit/extutil"
)

var (
	taskDefinitionsCache map[string][]string
	hostIpCache          map[string]string
)

func UpdateAgentExtensions(httpClient *resty.Client, awsClient *ecs.Client) {
	var currentRegistrations *[]extensionConfigAO
	resp, err := httpClient.R().
		SetHeader("Accept", "application/json").
		SetResult(&currentRegistrations).
		Get("/extensions")

	if err != nil {
		log.Error().Err(err).Msg("Failed to get extension registrations from the agent. Skip Discovery.")
		return
	}
	if resp.IsError() {
		log.Error().Msgf("Failed to get extension registrations from the agent: %s. Skip Discovery.", resp.Status())
		return
	}
	if resp.IsSuccess() {
		log.Debug().Int("count", len(*currentRegistrations)).Msg("Got extension registrations from the agent")
	}

	for _, taskFamily := range extensionconfig.Config.TaskFamilies {
		listTasksOutput, err := awsClient.ListTasks(context.TODO(), &ecs.ListTasksInput{
			Cluster:       &extensionconfig.Config.EcsClusterName,
			DesiredStatus: types.DesiredStatusRunning,
			Family:        &taskFamily,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to get tasks from ECS. Skip Discovery.")
			return
		}
		if len(listTasksOutput.TaskArns) > 0 {
			describeTasksOutput, err := awsClient.DescribeTasks(context.TODO(), &ecs.DescribeTasksInput{
				Cluster: &extensionconfig.Config.EcsClusterName,
				Tasks:   listTasksOutput.TaskArns,
				Include: []types.TaskField{types.TaskFieldTags},
			})
			if err != nil {
				log.Error().Err(err).Msg("Failed to describe tasks from ECS. Skip Discovery.")
				return
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
					ip = getHostIp(awsClient, *task.ContainerInstanceArn)
				} else if len(task.Containers[0].NetworkInterfaces) > 0 {
					ip = task.Containers[0].NetworkInterfaces[0].PrivateIpv4Address
				}
				if ip != nil {
					log.Info().Msgf("Discovered Task: %s - %s:%s - %v", *task.Group, *ip, *portTag, *typesTag)
				} else {
					log.Warn().Msgf("Task: %s %s - No IP/Port found. Ignore.", *task.Group, *task.TaskArn)
				}
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

func getHostIp(awsClient *ecs.Client, containerInstanceArn string) *string {
	if hostIpCache == nil {
		hostIpCache = make(map[string]string)
	}
	ip, ok := hostIpCache[containerInstanceArn]
	if !ok {
		containerInstance, err := awsClient.DescribeContainerInstances(context.TODO(), &ecs.DescribeContainerInstancesInput{
			Cluster:            &extensionconfig.Config.EcsClusterName,
			ContainerInstances: []string{containerInstanceArn},
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to describe container instances from ECS.")
			return nil
		}
		for _, detail := range containerInstance.ContainerInstances[0].Attachments[0].Details {
			if *detail.Name == "privateIPv4Address" {
				ip = *detail.Value
				hostIpCache[containerInstanceArn] = ip
				return extutil.Ptr(ip)
			}
		}
	} else {
		return extutil.Ptr(ip)
	}
	return nil
}
