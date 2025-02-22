package metrics

import (
	"time"

	"github.com/signalfx/golib/datapoint"
	k8sutil "github.com/signalfx/signalfx-agent/internal/monitors/kubernetes/utils"
	atypes "github.com/signalfx/signalfx-agent/internal/monitors/types"
	"github.com/signalfx/signalfx-agent/internal/utils"
	"k8s.io/api/extensions/v1beta1"
)

func datapointsForDaemonSet(ds *v1beta1.DaemonSet) []*datapoint.Datapoint {
	dimensions := map[string]string{
		"metric_source":        "kubernetes",
		"kubernetes_namespace": ds.Namespace,
		"uid":                  string(ds.UID),
		"kubernetes_name":      ds.Name,
	}

	return []*datapoint.Datapoint{
		datapoint.New(
			"kubernetes.daemon_set.current_scheduled",
			dimensions,
			datapoint.NewIntValue(int64(ds.Status.CurrentNumberScheduled)),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.daemon_set.desired_scheduled",
			dimensions,
			datapoint.NewIntValue(int64(ds.Status.DesiredNumberScheduled)),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.daemon_set.misscheduled",
			dimensions,
			datapoint.NewIntValue(int64(ds.Status.NumberMisscheduled)),
			datapoint.Gauge,
			time.Now()),
		datapoint.New(
			"kubernetes.daemon_set.ready",
			dimensions,
			datapoint.NewIntValue(int64(ds.Status.NumberReady)),
			datapoint.Gauge,
			time.Now()),
	}
}

func dimPropsForDaemonSet(ds *v1beta1.DaemonSet) *atypes.DimProperties {
	props, tags := k8sutil.PropsAndTagsFromLabels(ds.Labels)
	props["kubernetes_workload"] = "DaemonSet"

	for _, or := range ds.OwnerReferences {
		props[utils.LowercaseFirstChar(or.Kind)] = or.Name
		props[utils.LowercaseFirstChar(or.Kind)+"_uid"] = string(or.UID)
	}

	if len(props) == 0 && len(tags) == 0 {
		return nil
	}

	return &atypes.DimProperties{
		Dimension: atypes.Dimension{
			Name:  "kubernetes_uid",
			Value: string(ds.UID),
		},
		Properties: props,
		Tags:       tags,
	}
}
