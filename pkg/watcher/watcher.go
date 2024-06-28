package watcher

import (
	"context"
	"strings"
	"sync"

	"github.com/gobuffalo/flect"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/clusterreader"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
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

	// Options can be provided when creating a new StatusWatcher to customize the
	// behavior.
	Options *Options

	// informerRefs tracks which informers have been started and stopped by the ObjectStatusReporter
	informerRefs map[GroupKindNamespace]*informerReference
}

func NewDefaultStatusWatcher(dynamicClient dynamic.Interface, mapper meta.RESTMapper, options *Options) kwatcher.StatusWatcher {
	var informerRefs map[GroupKindNamespace]*informerReference
	if options != nil && options.UseInformerRefCache {
		informerRefs = make(map[GroupKindNamespace]*informerReference)
	}

	return &DefaultStatusWatcher{
		DynamicClient: dynamicClient,
		Mapper:        mapper,
		Options:       options,
		informerRefs:  informerRefs,
	}
}

// Watch the cluster for changes made to the specified objects.
// Returns an event channel on which these updates (and errors) will be reported.
// Each update event includes the computed status of the object.
func (w *DefaultStatusWatcher) Watch(ctx context.Context, ids object.ObjMetadataSet, _ kwatcher.Options) <-chan event.Event {
	targetMap := map[string]GroupKindNamespace{}
	for _, id := range ids {
		targetMap[id.GroupKind.String()] = GroupKindNamespace{
			Group:     id.GroupKind.Group,
			Kind:      id.GroupKind.Kind,
			Namespace: "",
		}
	}
	var objectFilter ObjectFilter = &AllowListObjectFilter{AllowList: ids}
	if w.Options != nil && w.Options.UseCustomObjectFilter {
		objectFilter = w.Options.ObjectFilter
	}

	var labelSelector labels.Selector
	if w.Options != nil && w.Options.Filters != nil {
		labelSelector = w.Options.Filters.Labels
	}

	reporter := &ObjectStatusReporter{
		ObjectFilter:  objectFilter,
		Targets:       maps.Values(targetMap),
		lock:          sync.Mutex{},
		context:       ctx,
		LabelSelector: labelSelector,
		DynamicClient: w.DynamicClient,
		Mapper:        w.Mapper,
		StatusReader:  statusreaders.NewDefaultStatusReader(w.Mapper),
		ClusterReader: &clusterreader.DynamicClusterReader{
			DynamicClient: w.DynamicClient,
			Mapper:        w.Mapper,
		},
		informerRefs: w.informerRefs,
	}

	return reporter.Start(ctx)
}

func gvrFromGvk(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: flect.Pluralize(strings.ToLower(gvk.Kind)),
	}
}

func handleFatalError(err error) <-chan event.Event {
	eventCh := make(chan event.Event)
	go func() {
		defer close(eventCh)
		eventCh <- event.Event{
			Type:  event.ErrorEvent,
			Error: err,
		}
	}()
	return eventCh
}
