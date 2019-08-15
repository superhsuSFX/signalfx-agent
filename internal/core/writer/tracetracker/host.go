package tracetracker

import (
	"context"

	"github.com/signalfx/golib/trace"
	"github.com/signalfx/signalfx-agent/internal/core/common/dpmeta"
	"github.com/signalfx/signalfx-agent/internal/core/services"
	"github.com/signalfx/signalfx-agent/internal/monitors/types"
)

var dimsToSyncSource = []string{
	"container_id",
	"kubernetes_pod_uid",
}

type SourceTracker struct {
	dimensionCh chan<- *types.Dimension
}

func NewSourceTracker(dimensionCh chan<- *types.Dimension) *SourceTracker {
	return &SourceTracker{
		dimensionCh: dimensionCh,
	}
}

func (st *SourceTracker) AddSpans(ctx context.Context, spans []*trace.Span) {
	var props []*types.Dimension
	for i := range spans {
		endpoint, ok := spans[i].Meta[dpmeta.EndpointMeta].(services.Endpoint)
		if !ok || endpoint == nil {
			continue
		}

		dims := endpoint.Dimensions()
		for _, dim := range dimsToSyncSource {
			if 
		}
	}
}
