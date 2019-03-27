# App Mesh Deployment

At the moment, due to the complicated nature of routing to host instances, when using dynamic task placement, the recommended way to run the Smart Agent in App Mesh services is as a sidecar container.

The example here is for ECS, but other deployment models will be very similar.

## Create task definition

The provided [example task definition](./app_mesh_with_agent_task.json) shows how the Smart Agent container should be configured with an App Mesh service.

In order to gather metrics about the host on which the service runs, the container needs access to the host filesystem. Define the following volumes in the task definition:

```json
     "volumes": [
        {
            "name": "hostfs",
            "host": {
                "sourcePath": "/"
            },
            "dockerVolumeConfiguration": null
        },
        {
            "name": "docker-socket",
            "host": {
                "sourcePath": "/var/run/docker.sock"
            },
            "dockerVolumeConfiguration": null
        }
    ]
``` 

Add the Smart Agent container to the container definitions:

```json
    "containerDefinitions": [
        {
            "name": "agent",
            "image": "quay.io/signalfx/signalfx-agent:4.3.0",
            "essential": true,
            "portMappings": [
                {
                    "containerPort": 9080,
                    "hostPort": 9080,
                    "protocol": "tcp"
                }
            ],
            "environment": [
                {
                    "name": "ACCESS_TOKEN",
                    "value": "MY_ACCESS_TOKEN"
                },
                {
                    "name": "INGEST_URL",
                    "value": "MY_INGEST_URL"
                }
            ],
            "mountPoints": [
                {
                    "sourceVolume": "hostfs",
                    "containerPath": "/hostfs",
                    "readOnly": true
                },
                {
                    "sourceVolume": "docker-socket",
                    "containerPath": "/var/run/docker.sock",
                    "readOnly": true
                }
            ],
            "volumesFrom": [
                {
                    "sourceContainer": "agent-config"
                }
            ],
            "entryPoint": [
                "bash",
                "-c"
            ],
            "command": [
                "/bin/signalfx-agent -config /agent_config/agent.yaml"
            ],
            "dockerLabels": {
                "app": "signalfx-agent"
            },
            "dnsSearchDomains": null,
            "logConfiguration": null,
            "linuxParameters": null,
            "ulimits": null,
            "dnsServers": null,
            "cpu": 0,
            "workingDirectory": null,
            "dockerSecurityOptions": null,
            "memory": null,
            "memoryReservation": null,
            "disableNetworking": null,
            "healthCheck": null,
            "links": null,
            "hostname": null,
            "extraHosts": null,
            "user": null,
            "readonlyRootFilesystem": null,
            "privileged": null
        }
    ]
```

The configuration here assumes a container, named `agent-config`, in the same
task with a static volume mount at `/agent_config/`.

The service's app and Envoy should be configured to send traces through the
Smart Agent's trace forwarder.

Additionally, an endpoint must be defined in the mesh for the Smart Agent to
send traces and metrics, matching the value given for the `INGEST_URL`
environment variable.

An example App Mesh virtual node and virtual service for the SignalFx ingest endpoint is provided in [nodes/signalfx.json](./nodes/signalfx). 

# Configuration

The agent config file must be provided through a volume mount or downloaded as
the agent initializes. The the URL of the config file to be downloaded can be
provided with the `CONFIG_URL` environment variable. As downloading requires
additional configuration of the network, the easiest method may be to put the
config file in a Docker volume mount that is shared with the agent container. 
The file path in the mount can be specified in the starting command for the agent.
The example below assumes a mounted path `/agent_config/` that has a config file,
`agent.yaml`.

```
          EntryPoint:
            - "bash"
            - "-c"
          Command:
            - "/bin/signalfx-agent -config /agent_config/agent.yaml"
```

The provided Dockerfile can be built and pushed to a repo to use as a valid
default configuration.

