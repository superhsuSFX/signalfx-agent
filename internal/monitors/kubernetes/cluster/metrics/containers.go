package metrics

import (
	"strings"
	"time"

	"github.com/signalfx/golib/datapoint"
	atypes "github.com/signalfx/signalfx-agent/internal/monitors/types"
	"github.com/signalfx/signalfx-agent/internal/utils"
	v1 "k8s.io/api/core/v1"
)

func dimPropsForContainer(cs v1.ContainerStatus) *atypes.DimProperties {

	containerProps := make(map[string]string)

	if cs.State.Running != nil {
		containerProps["container_status"] = "running"
	}

	if cs.State.Terminated != nil {
		containerProps["container_status"] = "terminated"
		containerProps["container_status_reason"] = cs.State.Terminated.Reason
	}

	if cs.State.Waiting != nil {
		containerProps["container_status"] = "waiting"
		containerProps["container_status_reason"] = cs.State.Waiting.Reason
	}

	if len(containerProps) > 0 {
		return &atypes.DimProperties{
			Dimension: atypes.Dimension{
				Name:  "container_id",
				Value: stripContainerIDPrefix(cs.ContainerID),
			},
			Properties: containerProps,
		}
	}
	return nil
}

func stripContainerIDPrefix(id string) string {
	out := strings.Replace(id, "docker://", "", 1)
	out = strings.Replace(out, "cri-o://", "", 1)

	return out
}

func datapointsForContainerStatus(cs v1.ContainerStatus, contDims map[string]string) []*datapoint.Datapoint {

	dps := []*datapoint.Datapoint{
		datapoint.New(
			"kubernetes.container_restart_count",
			contDims,
			datapoint.NewIntValue(int64(cs.RestartCount)),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.container_ready",
			contDims,
			datapoint.NewIntValue(int64(utils.BoolToInt(cs.Ready))),
			datapoint.Gauge,
			time.Now()),
	}

	return dps
}

func datapointsForContainerSpec(c v1.Container, contDims map[string]string) []*datapoint.Datapoint {

	dps := []*datapoint.Datapoint{
		datapoint.New(
			"kubernetes.container_cpu.request",
			contDims,
			datapoint.NewIntValue(c.Resources.Limits.Cpu().Value()),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.container_memory.request",
			contDims,
			datapoint.NewIntValue(c.Resources.Limits.Memory().Value()),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.container_cpu.limit",
			contDims,
			datapoint.NewIntValue(c.Resources.Limits.Cpu().Value()),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.container_memory.limit",
			contDims,
			datapoint.NewIntValue(c.Resources.Limits.Memory().Value()),
			datapoint.Gauge,
			time.Now()),
	}

	return dps
}

func getAllContainerDimensions(id string, name string, image string, dims map[string]string) map[string]string {
	out := utils.CloneStringMap(dims)

	out["container_id"] = id
	out["container_spec_name"] = name
	out["container_image"] = image

	return out
}
