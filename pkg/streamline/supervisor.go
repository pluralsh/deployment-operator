package streamline

import (
	"context"
	"fmt"
	"sync"

	cmap "github.com/orcaman/concurrent-map/v2"

	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

var (
	supervisor *Supervisor

	// TODO: read that from discovery client
	coreResources = containers.ToSet([]schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "endpoints"},
		{Group: "", Version: "v1", Resource: "namespaces"},
		{Group: "", Version: "v1", Resource: "nodes"},
		{Group: "", Version: "v1", Resource: "persistentvolumes"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "", Version: "v1", Resource: "resourcequotas"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "events"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},

		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},

		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},

		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},

		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	})
)

type Supervisor struct {
	mu            sync.Mutex
	client        dynamic.Interface
	store         store.Store
	register      chan schema.GroupVersionResource
	synchronizers cmap.ConcurrentMap[schema.GroupVersionResource, Synchronizer]
}

func Run(client dynamic.Interface, store store.Store) {
	if supervisor != nil {
		return
	}

	supervisor = &Supervisor{
		client:        client,
		store:         store,
		register:      make(chan schema.GroupVersionResource),
		synchronizers: cmap.NewStringer[schema.GroupVersionResource, Synchronizer](),
	}

	supervisor.run(context.Background())
}

func GetSupervisor() (*Supervisor, error) {
	return supervisor, lo.Ternary(supervisor == nil, fmt.Errorf("supervisor not initialized"), nil)
}

func (in *Supervisor) run(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				in.stop()
				return
			case gvr := <-in.register:
				if in.synchronizers.Has(gvr) {
					continue
				}

				in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store))
				go func() {
					syn, ok := in.synchronizers.Get(gvr)
					if !ok {
						return
					}
					if err := syn.Start(ctx); err != nil {
						in.synchronizers.Remove(gvr)
						in.register <- gvr
					}
				}()
			}
		}
	}()

	for _, gvr := range coreResources.List() {
		in.Register(gvr)
	}
}

func (in *Supervisor) stop() {
	in.mu.Lock()
	defer in.mu.Unlock()

	for _, s := range in.synchronizers.Items() {
		s.Stop()
	}

	close(in.register)
}

func (in *Supervisor) Register(gvr schema.GroupVersionResource) {
	in.mu.Lock()
	defer in.mu.Unlock()

	in.register <- gvr
}

func (in *Supervisor) Unregister(gvr schema.GroupVersionResource) {
	in.mu.Lock()
	defer in.mu.Unlock()

	if s, ok := in.synchronizers.Get(gvr); ok {
		s.Stop()
		in.synchronizers.Remove(gvr)
	}
}
