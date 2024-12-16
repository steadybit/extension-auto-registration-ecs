package autoregistration

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/steadybit/extension-auto-registration-ecs/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/mock"
	"net/http"
	"reflect"
	"testing"
)

type ecsClientApiMock struct {
	mock.Mock
}

func (m *ecsClientApiMock) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.ListTasksOutput), args.Error(1)
}

func (m *ecsClientApiMock) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeTasksOutput), args.Error(1)
}

func (m *ecsClientApiMock) DescribeContainerInstances(ctx context.Context, params *ecs.DescribeContainerInstancesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeContainerInstancesOutput), args.Error(1)
}

type ec2ClientApiMock struct {
	mock.Mock
}

func (m *ec2ClientApiMock) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeInstancesOutput), args.Error(1)
}

func Test_discoverExtensions(t *testing.T) {
	config.Config.TaskFamilies = []string{"steadybit-extension-test"}
	type args struct {
		ecsClient func() EcsApi
		ec2Client func() Ec2Api
	}
	tests := []struct {
		name string
		args args
		want []extensionConfigAO
	}{
		{
			name: "Should discover daemon extensions",
			args: args{
				ecsClient: func() EcsApi {
					ecsMock := new(ecsClientApiMock)
					ecsMock.On("ListTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
						TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"},
					}, nil)
					ecsMock.On("DescribeTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
						Tasks: []types.Task{
							{
								ContainerInstanceArn: extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:container-instance/12345678901234567890"),
								TaskArn:              extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"),
								Group:                extutil.Ptr("steadybit-extension-test"),
								Tags: []types.Tag{
									{
										Key:   extutil.Ptr("steadybit_extension_port"),
										Value: extutil.Ptr("8080"),
									},
									{
										Key:   extutil.Ptr("steadybit_extension_type"),
										Value: extutil.Ptr("ACTION:DISCOVERY"),
									},
									{
										Key:   extutil.Ptr("steadybit_extension_daemon"),
										Value: extutil.Ptr("true"),
									}},
							},
						},
					}, nil)
					ecsMock.On("DescribeContainerInstances", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.DescribeContainerInstancesOutput{
						ContainerInstances: []types.ContainerInstance{
							{
								Ec2InstanceId: extutil.Ptr("i-1234567890abcdef0"),
							},
						},
					}, nil)
					return ecsMock
				},
				ec2Client: func() Ec2Api {
					ec2Mock := new(ec2ClientApiMock)
					ec2Mock.On("DescribeInstances", mock.Anything, mock.Anything, mock.Anything).Return(&ec2.DescribeInstancesOutput{
						Reservations: []ec2types.Reservation{
							{
								Instances: []ec2types.Instance{
									{
										InstanceId:       extutil.Ptr("i-1234567890abcdef0"),
										PrivateIpAddress: extutil.Ptr("111.222.333.444"),
									},
								},
							},
						},
					}, nil)
					return ec2Mock
				},
			},
			want: []extensionConfigAO{
				{
					Url:   "http://111.222.333.444:8080",
					Types: []string{"ACTION", "DISCOVERY"},
				},
			},
		},
		{
			name: "Should discover replica extensions",
			args: args{
				ecsClient: func() EcsApi {
					ecsMock := new(ecsClientApiMock)
					ecsMock.On("ListTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
						TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"},
					}, nil)
					ecsMock.On("DescribeTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
						Tasks: []types.Task{
							{
								ContainerInstanceArn: extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:container-instance/12345678901234567890"),
								TaskArn:              extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"),
								Group:                extutil.Ptr("steadybit-extension-test"),
								Containers: []types.Container{
									{
										NetworkInterfaces: []types.NetworkInterface{
											{
												PrivateIpv4Address: extutil.Ptr("111.222.333.444"),
											},
										},
									},
								},
								Tags: []types.Tag{
									{
										Key:   extutil.Ptr("steadybit_extension_port"),
										Value: extutil.Ptr("8080"),
									},
									{
										Key:   extutil.Ptr("steadybit_extension_type"),
										Value: extutil.Ptr("ACTION:DISCOVERY"),
									},
								},
							},
						},
					}, nil)
					return ecsMock
				},
				ec2Client: func() Ec2Api {
					ec2Mock := new(ec2ClientApiMock)
					return ec2Mock
				},
			},
			want: []extensionConfigAO{
				{
					Url:   "http://111.222.333.444:8080",
					Types: []string{"ACTION", "DISCOVERY"},
				},
			},
		},
		{
			name: "Should ignore task with missing port tag",
			args: args{
				ecsClient: func() EcsApi {
					ecsMock := new(ecsClientApiMock)
					ecsMock.On("ListTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
						TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"},
					}, nil)
					ecsMock.On("DescribeTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
						Tasks: []types.Task{
							{
								TaskArn: extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"),
								Group:   extutil.Ptr("steadybit-extension-test"),
								Tags: []types.Tag{
									{
										Key:   extutil.Ptr("steadybit_extension_type"),
										Value: extutil.Ptr("8080"),
									},
								},
							},
						},
					}, nil)
					return ecsMock
				},
				ec2Client: func() Ec2Api {
					ec2Mock := new(ec2ClientApiMock)
					return ec2Mock
				},
			},
			want: []extensionConfigAO{},
		},
		{
			name: "Should ignore task with missing type tag",
			args: args{
				ecsClient: func() EcsApi {
					ecsMock := new(ecsClientApiMock)
					ecsMock.On("ListTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.ListTasksOutput{
						TaskArns: []string{"arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"},
					}, nil)
					ecsMock.On("DescribeTasks", mock.Anything, mock.Anything, mock.Anything).Return(&ecs.DescribeTasksOutput{
						Tasks: []types.Task{
							{
								TaskArn: extutil.Ptr("arn:aws:ecs:eu-central-1:123456789012:task/steadybit-extension-test/12345678901234567890"),
								Group:   extutil.Ptr("steadybit-extension-test"),
								Tags: []types.Tag{
									{
										Key:   extutil.Ptr("steadybit_extension_port"),
										Value: extutil.Ptr("8080"),
									},
								},
							},
						},
					}, nil)
					return ecsMock
				},
				ec2Client: func() Ec2Api {
					ec2Mock := new(ec2ClientApiMock)
					return ec2Mock
				},
			},
			want: []extensionConfigAO{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := discoverExtensions(extutil.Ptr(tt.args.ecsClient()), extutil.Ptr(tt.args.ec2Client())); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("discoverExtensions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_syncRegistrations(t *testing.T) {
	type args struct {
		httpClient           func() *resty.Client
		currentRegistrations *[]extensionConfigAO
		discoveredExtensions *[]extensionConfigAO
	}
	tests := []struct {
		name string
		args args
		want map[string]int
	}{
		{
			name: "Should add registrations",
			args: args{
				httpClient: func() *resty.Client {
					client := resty.New()
					client.SetBaseURL("http://localhost:42899")
					httpmock.ActivateNonDefault(client.GetClient())
					httpmock.RegisterMatcherResponder("POST", "http://localhost:42899/extensions",
						httpmock.BodyContainsString(`{"url":"http://111.222.333.444:8080","types":["ACTION","DISCOVERY"]}`).WithName("mock"),
						httpmock.NewStringResponder(200, ""))
					return client
				},
				currentRegistrations: &[]extensionConfigAO{
					{
						Url:   "http://99.99.99.99:9999",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
				discoveredExtensions: &[]extensionConfigAO{
					{
						Url:   "http://111.222.333.444:8080",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
			},
			want: map[string]int{
				"POST http://localhost:42899/extensions <mock>": 1,
			},
		},
		{
			name: "Should remove registrations",
			args: args{
				httpClient: func() *resty.Client {
					client := resty.New()
					client.SetBaseURL("http://localhost:42899")
					httpmock.ActivateNonDefault(client.GetClient())
					httpmock.RegisterMatcherResponder("DELETE", "http://localhost:42899/extensions",
						httpmock.BodyContainsString(`{"url":"http://111.222.333.444:8080","types":["ACTION","DISCOVERY"]}`).WithName("mock"),
						httpmock.NewStringResponder(200, ""))
					return client
				},
				currentRegistrations: &[]extensionConfigAO{
					{
						Url:   "http://111.222.333.444:8080",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
				discoveredExtensions: &[]extensionConfigAO{
					{
						Url:   "http://99.99.99.99:9999",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
			},
			want: map[string]int{
				"DELETE http://localhost:42899/extensions <mock>": 1,
			},
		},
		{
			name: "Should not touch existing registrations",
			args: args{
				httpClient: func() *resty.Client {
					client := resty.New()
					client.SetBaseURL("http://localhost:42899")
					httpmock.ActivateNonDefault(client.GetClient())
					return client
				},
				currentRegistrations: &[]extensionConfigAO{
					{
						Url:   "http://99.99.99.99:9999",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
				discoveredExtensions: &[]extensionConfigAO{
					{
						Url:   "http://99.99.99.99:9999",
						Types: []string{"ACTION", "DISCOVERY"},
					},
				},
			},
			want: map[string]int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncRegistrations(tt.args.httpClient(), tt.args.currentRegistrations, tt.args.discoveredExtensions)
			if got := httpmock.GetCallCountInfo(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("httpmock.GetCallCountInfo() = %v, want %v", got, tt.want)
			}
			httpmock.Reset()
		})
	}
}

func Test_getCurrentRegistrations(t *testing.T) {
	type args struct {
		httpClient func() *resty.Client
	}
	tests := []struct {
		name string
		args args
		want []extensionConfigAO
	}{
		{
			name: "Should return registrations",
			args: args{
				httpClient: func() *resty.Client {
					client := resty.New()
					client.SetBaseURL("http://localhost:42899")
					header := http.Header{}
					header.Add("Content-Type", "application/json")
					httpmock.ActivateNonDefault(client.GetClient())
					httpmock.RegisterResponder("GET", "http://localhost:42899/extensions",
						httpmock.NewStringResponder(200, "[{\"url\":\"http://111.222.333.444:8080\",\"types\":[\"ACTION\",\"DISCOVERY\"]}]").HeaderAdd(header))
					return client
				},
			},
			want: []extensionConfigAO{
				{
					Url:   "http://111.222.333.444:8080",
					Types: []string{"ACTION", "DISCOVERY"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := getCurrentRegistrations(tt.args.httpClient())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCurrentRegistrations() got = %v, want %v", got, tt.want)
			}
			httpmock.Reset()
		})
	}
}
