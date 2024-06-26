package cachewatcher

import (
	"context"
	"strings"
	"sync"

	"github.com/gobuffalo/flect"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/clusterreader"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// DefaultStatusWatcher reports on status updates to a set of objects.
//
// Use NewDefaultStatusWatcher to build a DefaultStatusWatcher with default settings.
type DefaultStatusWatcher struct {
	// DynamicClient is used to watch of resource objects.
	DynamicClient dynamic.Interface

	// Mapper is used to map from GroupKind to GroupVersionKind.
	Mapper meta.RESTMapper

	LabelSelector labels.Selector
}

func NewDefaultStatusWatcher(dynamicClient dynamic.Interface, mapper meta.RESTMapper, labelSelector labels.Selector) *DefaultStatusWatcher {
	return &DefaultStatusWatcher{
		DynamicClient: dynamicClient,
		Mapper:        mapper,
		LabelSelector: labelSelector,
	}
}

// Watch the cluster for changes made to the specified objects.
// Returns an event channel on which these updates (and errors) will be reported.
// Each update event includes the computed status of the object.
func (w *DefaultStatusWatcher) Watch(ctx context.Context, ids object.ObjMetadataSet, opts watcher.Options) <-chan event.Event {
	targetMap := map[string]GroupKindNamespace{}
	for _, id := range ids {
		targetMap[id.GroupKind.String()] = GroupKindNamespace{
			Group:     id.GroupKind.Group,
			Kind:      id.GroupKind.Kind,
			Namespace: "",
		}
	}

	informer := &ObjectStatusReporter{
		Targets:       maps.Values(targetMap),
		lock:          sync.Mutex{},
		context:       ctx,
		LabelSelector: w.LabelSelector,
		DynamicClient: w.DynamicClient,
		Mapper:        w.Mapper,
		StatusReader:  statusreaders.NewDefaultStatusReader(w.Mapper),
		ClusterReader: &clusterreader.DynamicClusterReader{
			DynamicClient: w.DynamicClient,
			Mapper:        w.Mapper,
		},
	}

	return informer.Start(ctx)
}

func gvrFromGvk(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: flect.Pluralize(strings.ToLower(gvk.Kind)),
	}
}
