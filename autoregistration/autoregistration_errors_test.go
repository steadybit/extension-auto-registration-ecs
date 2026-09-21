package autoregistration

import (
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/steadybit/extension-auto-registration-ecs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"
)

const testBaseUrl = "http://localhost:42899"

func jsonHeader() http.Header {
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	return header
}

func mockedClient(t *testing.T) *resty.Client {
	t.Helper()
	client := resty.New()
	client.SetBaseURL(testBaseUrl)
	httpmock.ActivateNonDefault(client.GetClient())
	t.Cleanup(httpmock.Reset)
	return client
}

// getHostIp memoizes in the hostIpCache package global, so a test that does not
// clear it can silently pass on a value another test left behind.
func resetHostIpCache(t *testing.T) {
	t.Helper()
	hostIpCache = nil
	t.Cleanup(func() { hostIpCache = nil })
}

func Test_discoverExtensions_failures(t *testing.T) {
	config.Config.TaskFamilies = []string{"steadybit-extension-test"}
	config.Config.EcsClusterName = "test-cluster"

	tests := []struct {
		name      string
		ecsClient func() EcsApi
	}{
		{
			name: "Should discover nothing when listing tasks fails",
			ecsClient: func() EcsApi {
				ecsMock := new(ecsClientApiMock)
				ecsMock.On("ListTasks", mock.Anything, mock.Anything).Return(nil, errors.New("access denied"))
				return ecsMock
			},
		},
		{
			name: "Should discover nothing when describing tasks fails",
			ecsClient: func() EcsApi {
				ecsMock := new(ecsClientApiMock)
				ecsMock.On("ListTasks", mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
					TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/1"},
				}, nil)
				ecsMock.On("DescribeTasks", mock.Anything, mock.Anything).Return(nil, errors.New("throttled"))
				return ecsMock
			},
		},
		{
			name: "Should discover nothing when the family has no running tasks",
			ecsClient: func() EcsApi {
				ecsMock := new(ecsClientApiMock)
				ecsMock.On("ListTasks", mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{TaskArns: []string{}}, nil)
				return ecsMock
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoverExtensions(new(tt.ecsClient()), new(Ec2Api(new(ec2ClientApiMock))))
			assert.Empty(t, got)
		})
	}
}

func Test_discoverExtensions_ignoresTaskWithoutIp(t *testing.T) {
	config.Config.TaskFamilies = []string{"steadybit-extension-test"}
	resetHostIpCache(t)

	ecsMock := new(ecsClientApiMock)
	ecsMock.On("ListTasks", mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
		TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/1"},
	}, nil)
	// Not a daemon task, and its container exposes no network interface, so there
	// is no address to register — the task must be skipped rather than registered
	// with an empty host.
	ecsMock.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
		Tasks: []types.Task{
			{
				TaskArn:    new("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/1"),
				Group:      new("steadybit-extension-test"),
				Containers: []types.Container{{NetworkInterfaces: []types.NetworkInterface{}}},
				Tags: []types.Tag{
					{Key: new("steadybit_extension_port"), Value: new("8080")},
					{Key: new("steadybit_extension_type"), Value: new("ACTION")},
				},
			},
		},
	}, nil)

	got := discoverExtensions(new(EcsApi(ecsMock)), new(Ec2Api(new(ec2ClientApiMock))))

	assert.Empty(t, got)
}

func Test_getCurrentRegistrations_failures(t *testing.T) {
	t.Run("Should return an error when the agent responds with an error status", func(t *testing.T) {
		client := mockedClient(t)
		httpmock.RegisterResponder("GET", testBaseUrl+"/extensions",
			httpmock.NewStringResponder(500, "boom").HeaderAdd(jsonHeader()))

		got, err := getCurrentRegistrations(client)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.Nil(t, got)
	})

	t.Run("Should treat a null body as no registrations rather than panicking", func(t *testing.T) {
		client := mockedClient(t)
		// A JSON "null" body unmarshals cleanly but leaves the result pointer nil;
		// dereferencing it used to panic with a nil pointer dereference.
		httpmock.RegisterResponder("GET", testBaseUrl+"/extensions",
			httpmock.NewStringResponder(200, "null").HeaderAdd(jsonHeader()))

		var got []extensionConfigAO
		var err error
		assert.NotPanics(t, func() { got, err = getCurrentRegistrations(client) })

		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("Should return an error when the agent is unreachable", func(t *testing.T) {
		// No responder registered, so httpmock fails the transport itself.
		client := mockedClient(t)

		got, err := getCurrentRegistrations(client)

		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

func Test_syncRegistrations_survivesErrorResponses(t *testing.T) {
	config.Config.AgentKey = "test-agent-key"

	t.Run("Should keep going when removing a registration fails", func(t *testing.T) {
		client := mockedClient(t)
		httpmock.RegisterResponder("DELETE", testBaseUrl+"/extensions", httpmock.NewStringResponder(500, "nope"))

		syncRegistrations(client,
			&[]extensionConfigAO{{Url: "http://1.1.1.1:8080", Types: []string{"ACTION"}}},
			&[]extensionConfigAO{})

		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	})

	t.Run("Should keep going when adding a registration fails", func(t *testing.T) {
		client := mockedClient(t)
		httpmock.RegisterResponder("POST", testBaseUrl+"/extensions", httpmock.NewStringResponder(500, "nope"))

		syncRegistrations(client,
			&[]extensionConfigAO{},
			&[]extensionConfigAO{{Url: "http://1.1.1.1:8080", Types: []string{"ACTION"}}})

		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	})
}

func Test_getHostIp(t *testing.T) {
	const containerInstanceArn = "arn:aws:ecs:eu-central-1:123456789012:container-instance/1"

	t.Run("Should return nil when describing the container instance fails", func(t *testing.T) {
		resetHostIpCache(t)
		ecsMock := new(ecsClientApiMock)
		ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything).Return(nil, errors.New("access denied"))

		got := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(new(ec2ClientApiMock))))

		assert.Nil(t, got)
	})

	t.Run("Should return nil when describing the ec2 instance fails", func(t *testing.T) {
		resetHostIpCache(t)
		ecsMock := new(ecsClientApiMock)
		ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything).Return(&ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []types.ContainerInstance{{Ec2InstanceId: new("i-1")}},
		}, nil)
		ec2Mock := new(ec2ClientApiMock)
		ec2Mock.On("DescribeInstances", mock.Anything, mock.Anything).Return(nil, errors.New("throttled"))

		got := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(ec2Mock)))

		assert.Nil(t, got)
	})

	t.Run("Should return nil when the ec2 instance has no reservations", func(t *testing.T) {
		resetHostIpCache(t)
		ecsMock := new(ecsClientApiMock)
		ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything).Return(&ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []types.ContainerInstance{{Ec2InstanceId: new("i-1")}},
		}, nil)
		ec2Mock := new(ec2ClientApiMock)
		ec2Mock.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{},
		}, nil)

		got := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(ec2Mock)))

		assert.Nil(t, got)
	})

	t.Run("Should return nil when the reservation has no instances", func(t *testing.T) {
		resetHostIpCache(t)
		ecsMock := new(ecsClientApiMock)
		ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything).Return(&ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []types.ContainerInstance{{Ec2InstanceId: new("i-1")}},
		}, nil)
		ec2Mock := new(ec2ClientApiMock)
		ec2Mock.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{}}},
		}, nil)

		got := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(ec2Mock)))

		assert.Nil(t, got)
	})

	t.Run("Should serve a repeated lookup from the cache without calling AWS again", func(t *testing.T) {
		resetHostIpCache(t)
		ecsMock := new(ecsClientApiMock)
		ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything).Return(&ecs.DescribeContainerInstancesOutput{
			ContainerInstances: []types.ContainerInstance{{Ec2InstanceId: new("i-1")}},
		}, nil)
		ec2Mock := new(ec2ClientApiMock)
		ec2Mock.On("DescribeInstances", mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{Instances: []ec2types.Instance{{InstanceId: new("i-1"), PrivateIpAddress: new("10.0.0.1")}}},
			},
		}, nil)

		first := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(ec2Mock)))
		second := getHostIp(containerInstanceArn, new(EcsApi(ecsMock)), new(Ec2Api(ec2Mock)))

		assert.Equal(t, "10.0.0.1", *first)
		assert.Equal(t, "10.0.0.1", *second)
		ecsMock.AssertNumberOfCalls(t, "DescribeContainerInstances", 1)
		ec2Mock.AssertNumberOfCalls(t, "DescribeInstances", 1)
	})
}

func Test_UpdateAgentExtensions(t *testing.T) {
	config.Config.TaskFamilies = []string{"steadybit-extension-test"}
	config.Config.EcsClusterName = "test-cluster"
	config.Config.AgentKey = "test-agent-key"

	t.Run("Should replace a stale registration with the discovered one", func(t *testing.T) {
		resetHostIpCache(t)
		client := mockedClient(t)
		httpmock.RegisterResponder("GET", testBaseUrl+"/extensions",
			httpmock.NewStringResponder(200, `[{"url":"http://9.9.9.9:9999","types":["ACTION"]}]`).HeaderAdd(jsonHeader()))
		httpmock.RegisterResponder("DELETE", testBaseUrl+"/extensions", httpmock.NewStringResponder(200, ""))
		httpmock.RegisterResponder("POST", testBaseUrl+"/extensions", httpmock.NewStringResponder(200, ""))

		ecsMock := new(ecsClientApiMock)
		ecsMock.On("ListTasks", mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
			TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/1"},
		}, nil)
		ecsMock.On("DescribeTasks", mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
			Tasks: []types.Task{
				{
					TaskArn: new("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/1"),
					Group:   new("steadybit-extension-test"),
					Containers: []types.Container{
						{NetworkInterfaces: []types.NetworkInterface{{PrivateIpv4Address: new("10.0.0.1")}}},
					},
					Tags: []types.Tag{
						{Key: new("steadybit_extension_port"), Value: new("8080")},
						{Key: new("steadybit_extension_type"), Value: new("ACTION")},
					},
				},
			},
		}, nil)

		ecsApi := EcsApi(ecsMock)
		ec2Api := Ec2Api(new(ec2ClientApiMock))
		UpdateAgentExtensions(client, &ecsApi, &ec2Api)

		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info["GET "+testBaseUrl+"/extensions"])
		assert.Equal(t, 1, info["DELETE "+testBaseUrl+"/extensions"], "the stale registration should be removed")
		assert.Equal(t, 1, info["POST "+testBaseUrl+"/extensions"], "the discovered extension should be added")
	})

	t.Run("Should not touch registrations when the agent cannot be read", func(t *testing.T) {
		resetHostIpCache(t)
		client := mockedClient(t)
		httpmock.RegisterResponder("GET", testBaseUrl+"/extensions", httpmock.NewStringResponder(503, "unavailable"))

		ecsMock := new(ecsClientApiMock)
		ecsApi := EcsApi(ecsMock)
		ec2Api := Ec2Api(new(ec2ClientApiMock))
		UpdateAgentExtensions(client, &ecsApi, &ec2Api)

		// Discovery must not even be attempted, and nothing may be written back.
		ecsMock.AssertNotCalled(t, "ListTasks", mock.Anything, mock.Anything)
		assert.Equal(t, 1, httpmock.GetTotalCallCount(), "only the failed GET should have been issued")
	})
}
