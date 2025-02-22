package cluster

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/signalfx/signalfx-agent/internal/monitors/kubernetes/cluster/metrics"
	"github.com/signalfx/signalfx-agent/internal/utils/k8sutil"
	log "github.com/sirupsen/logrus"

	quota "github.com/openshift/api/quota/v1"
	quotav1 "github.com/openshift/client-go/quota/clientset/versioned/typed/quota/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// State makes use of the K8s client's "reflector" helper to watch the API
// server for changes and keep the datapoint cache up to date,
type State struct {
	clientset   *k8s.Clientset
	quotaClient *quotav1.QuotaV1Client
	reflectors  map[string]*cache.Reflector
	namespace   string
	cancel      func()

	metricCache *metrics.DatapointCache
}

func newState(flavor KubernetesDistribution, restConfig *rest.Config, metricCache *metrics.DatapointCache,
	namespace string) (*State, error) {
	state := &State{
		reflectors:  make(map[string]*cache.Reflector),
		metricCache: metricCache,
		namespace:   namespace,
	}

	var err error

	if flavor == OpenShift {
		state.quotaClient, err = quotav1.NewForConfig(restConfig)
		if err != nil {
			return nil, fmt.Errorf("could not create API client for %s: %s", quota.SchemeGroupVersion, err)
		}
	}

	state.clientset, err = k8s.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create Kubernetes API client: %s", err)
	}

	return state, nil
}

// Start starts syncing any resource that isn't already being synced
func (cs *State) Start() {
	log.Info("Starting K8s API resource sync")

	var ctx context.Context
	ctx, cs.cancel = context.WithCancel(context.Background())

	coreClient := cs.clientset.CoreV1().RESTClient()
	extV1beta1Client := cs.clientset.ExtensionsV1beta1().RESTClient()
	appsV1Client := cs.clientset.AppsV1().RESTClient()
	batchV1Client := cs.clientset.BatchV1().RESTClient()
	batchBetaV1Client := cs.clientset.BatchV1beta1().RESTClient()
	if cs.quotaClient != nil {
		cs.beginSyncForType(ctx, &quota.ClusterResourceQuota{}, "clusterresourcequotas", v1.NamespaceAll,
			cs.quotaClient.RESTClient())
	}

	cs.beginSyncForType(ctx, &v1.Pod{}, "pods", cs.namespace, coreClient)
	cs.beginSyncForType(ctx, &v1beta1.DaemonSet{}, "daemonsets", cs.namespace, extV1beta1Client)
	cs.beginSyncForType(ctx, &v1beta1.Deployment{}, "deployments", cs.namespace, extV1beta1Client)
	cs.beginSyncForType(ctx, &appsv1.StatefulSet{}, "statefulsets", cs.namespace, appsV1Client)
	cs.beginSyncForType(ctx, &v1.ReplicationController{}, "replicationcontrollers", cs.namespace, coreClient)
	cs.beginSyncForType(ctx, &v1beta1.ReplicaSet{}, "replicasets", cs.namespace, extV1beta1Client)
	cs.beginSyncForType(ctx, &v1.ResourceQuota{}, "resourcequotas", cs.namespace, coreClient)
	cs.beginSyncForType(ctx, &v1.Service{}, "services", cs.namespace, coreClient)
	cs.beginSyncForType(ctx, &batchv1.Job{}, "jobs", cs.namespace, batchV1Client)
	cs.beginSyncForType(ctx, &batchv1beta1.CronJob{}, "cronjobs", cs.namespace, batchBetaV1Client)
	// Node and Namespace are NOT namespaced resources, so we don't need to
	// fetch them if we are scoped to a specific namespace
	if cs.namespace == "" {
		cs.beginSyncForType(ctx, &v1.Node{}, "nodes", "", coreClient)
		cs.beginSyncForType(ctx, &v1.Namespace{}, "namespaces", "", coreClient)
	}
}

func (cs *State) beginSyncForType(ctx context.Context, resType runtime.Object, resName string, namespace string, client cache.Getter) {
	keysSeen := make(map[interface{}]bool)

	store := k8sutil.FixedFakeCustomStore{
		FakeCustomStore: cache.FakeCustomStore{},
	}
	store.AddFunc = func(obj interface{}) error {
		cs.metricCache.Lock()
		defer cs.metricCache.Unlock()

		if key := cs.metricCache.HandleAdd(obj.(runtime.Object)); key != nil {
			keysSeen[key] = true
		}

		return nil
	}
	store.UpdateFunc = store.AddFunc
	store.DeleteFunc = func(obj interface{}) error {
		cs.metricCache.Lock()
		defer cs.metricCache.Unlock()

		if key := cs.metricCache.HandleDelete(obj.(runtime.Object)); key != nil {
			delete(keysSeen, key)
		}

		return nil
	}
	store.ReplaceFunc = func(list []interface{}, resourceVerion string) error {
		cs.metricCache.Lock()
		defer cs.metricCache.Unlock()

		for k := range keysSeen {
			cs.metricCache.DeleteByKey(k)
			delete(keysSeen, k)
		}
		for i := range list {
			if key := cs.metricCache.HandleAdd(list[i].(runtime.Object)); key != nil {
				keysSeen[key] = true
			}
		}
		return nil
	}

	watchList := cache.NewListWatchFromClient(client, resName, namespace, fields.Everything())
	cs.reflectors[resName] = cache.NewReflector(watchList, resType, &store, 0)

	go cs.reflectors[resName].Run(ctx.Done())
}

// Stop all running goroutines. There is a bug/limitation in the k8s go
// client's Controller where goroutines are leaked even when using the stop
// channel properly.
// See https://github.com/kubernetes/client-go/blob/release-8.0/tools/cache/controller.go#L144
func (cs *State) Stop() {
	log.Info("Stopping all K8s API resource sync")
	if cs.cancel != nil {
		cs.cancel()
	}
}
