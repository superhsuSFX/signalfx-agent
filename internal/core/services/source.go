package services

// SourceTracker is used to maintain the relationship between an endpoint's IP
// address (host) and the container/pod that it is part of.
type SourceTracker struct {
	hostToContainerID map[string]string
	hostToPodUID      map[string]string
}

func NewSourceTracker() *SourceTracker {
	return &SourceTracker{
		hostToContainerID: make(map[string]string),
		hostToPodUID:      make(map[string]string),
	}
}

func (st *SourceTracker) endpointAdded(service Endpoint) {
	host := service.Core().Host
	if host == "" {
		return
	}

	dims := service.Dimensions()
	if dims == nil {
		return
	}

	if containerID := dims["container_id"]; containerID != "" {
		st.hostToContainerID[host] = containerID
	}
	if podUID := dims["kubernetes_pod_uid"]; podUID != "" {
		st.hostToPodUID[host] = podUID
	}
}

func (st *SourceTracker) endpointRemoved(service Endpoint) {
}
