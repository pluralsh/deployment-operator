package watcher

import (
	"context"
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type DynamicStatusWatcher struct {
	*kwatcher.DefaultStatusWatcher

	// Options can be provided when creating a new StatusWatcher to customize the
	// behavior.
	Options Options

	// discoveryClient is used to ensure if CRD exists on the server.
	discoveryClient discovery.CachedDiscoveryInterface

	// informerRefs tracks which informers have been started and stopped by the ObjectStatusReporter
	informerRefs map[GroupKindNamespace]*watcherReference
}

func (in *DynamicStatusWatcher) Watch(ctx context.Context, ids object.ObjMetadataSet, opts kwatcher.Options) <-chan event.Event {
	var strategy kwatcher.RESTScopeStrategy

	if opts.RESTScopeStrategy != kwatcher.RESTScopeAutomatic {
		strategy = opts.RESTScopeStrategy
	}

	if in.Options.RESTScopeStrategy != nil {
		strategy = *in.Options.RESTScopeStrategy
	}

	if strategy == kwatcher.RESTScopeAutomatic {
		strategy = autoSelectRESTScopeStrategy(ids)
	}

	var scope meta.RESTScope
	var targets []GroupKindNamespace
	switch strategy {
	case kwatcher.RESTScopeRoot:
		scope = meta.RESTScopeRoot
		targets = rootScopeGKNs(ids)
		klog.V(3).Infof("DynamicStatusWatcher starting in root-scoped mode (targets: %d)", len(targets))
	case kwatcher.RESTScopeNamespace:
		scope = meta.RESTScopeNamespace
		targets = namespaceScopeGKNs(ids)
		klog.V(3).Infof("DynamicStatusWatcher starting in namespace-scoped mode (targets: %d)", len(targets))
	default:
		return handleFatalError(fmt.Errorf("invalid RESTScopeStrategy: %v", strategy))
	}

	var objectFilter kwatcher.ObjectFilter = &kwatcher.AllowListObjectFilter{AllowList: ids}
	if in.Options.UseCustomObjectFilter {
		objectFilter = in.Options.ObjectFilter
	}

	var labelSelector labels.Selector
	if in.Options.Filters != nil {
		labelSelector = in.Options.Filters.Labels
	}

	informer := &ObjectStatusReporter{
		Mapper:        in.Mapper,
		StatusReader:  in.StatusReader,
		ClusterReader: in.ClusterReader,
		Targets:       targets,
		RESTScope:     scope,
		ObjectFilter:  objectFilter,
		// Custom options
		LabelSelector:   labelSelector,
		DynamicClient:   in.DynamicClient,
		DiscoveryClient: in.discoveryClient,
		watcherRefs:     in.informerRefs,
		id:              in.Options.ID,
	}

	return informer.Start(ctx)
}

func NewDynamicStatusWatcher(dynamicClient dynamic.Interface, discoveryClient discovery.CachedDiscoveryInterface, mapper meta.RESTMapper, options Options) kwatcher.StatusWatcher {
	var informerRefs map[GroupKindNamespace]*watcherReference
	if options.UseInformerRefCache {
		informerRefs = make(map[GroupKindNamespace]*watcherReference)
	}

	defaultStatusWatcher := kwatcher.NewDefaultStatusWatcher(dynamicClient, mapper)
	defaultStatusWatcher.Filters = options.Filters

	return &DynamicStatusWatcher{
		DefaultStatusWatcher: defaultStatusWatcher,
		// Custom options
		discoveryClient: discoveryClient,
		Options:         options,
		informerRefs:    informerRefs,
	}
}
