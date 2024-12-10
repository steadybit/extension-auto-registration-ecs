# Extension Discovery for AWS ECS

The image provided by this repository can be used to discover the extensions that are installed in an AWS ECS cluster.

The image needs to be added as an additional container in the agent task definition. It will then use the aws sdk to
discover the extensions that are installed in the cluster and will sync/register them with the steadybit agent.


## Configuration

| Environment Variable                             | Meaning                                                                | required | default                                                                                                                     |
|--------------------------------------------------|------------------------------------------------------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------|
| `STEADYBIT_EXTENSION_DISCOVERY_ECS_CLUSTER_NAME` | The name of the ecs cluster.                                           | yes      |                                                                                                                             |
| `STEADYBIT_EXTENSION_AGENT_KEY`                  | The agent key (used to authenticate at the agent api).                 | yes      |                                                                                                                             |
| `STEADYBIT_EXTENSION_DISCOVERY_INTERVAL`         | The interval of the sync in seconds.                                   | no       | 30                                                                                                                          |
| `STEADYBIT_EXTENSION_DISCOVERY_TASK_FAMILIES`    | The task families that should be used to filter fetching running tasks | no       | steadybit-extension-host,<br/>steadybit-extension-container,<br/>steadybit-extension-http,<br/>steadybit-extension-aws<br/> |


## Pre-requisites

- The task role needs to have the following permissions:
  - `ecs:ListTasks`
  - `ecs:DescribeTasks`
  - `ecs:DescribeContainerInstances`
  - `ec2:DescribeInstances`
- Each extension task definition should have the following tags:
  - `steadybit_extension_port` - the port on which the extension is running
  - `steadybit_extension_types` - the types of the extensions, separated by a `:`, e.g. `ACTION:DISCOVERY`
  - `steadybit_extension_daemon` - if the extension is a daemon, the value should be `true`, can be omitted otherwise
- The tags need to be propagated to the tasks: `aws ecs create-service ...  --propagate-tags TASK_DEFINITION ....`

- More details can be found in the [docs](https://docs.steadybit.com/install-and-configure/install-agent/aws-ecs-ec2)
