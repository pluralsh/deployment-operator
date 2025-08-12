package applier

import (
	"context"
	"errors"
	"fmt"

	"github.com/pluralsh/deployment-operator/pkg/cache/db"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/apply/cache"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/apply/info"

	"github.com/pluralsh/deployment-operator/pkg/applier/filters"
	rccache "github.com/pluralsh/deployment-operator/pkg/cache"
	"sigs.k8s.io/cli-utils/pkg/apply/filter"
	"sigs.k8s.io/cli-utils/pkg/apply/mutator"
	"sigs.k8s.io/cli-utils/pkg/apply/prune"
	"sigs.k8s.io/cli-utils/pkg/apply/solver"
	"sigs.k8s.io/cli-utils/pkg/apply/taskrunner"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/object/validation"
)

type Applier struct {
	pruner        *prune.Pruner
	statusWatcher watcher.StatusWatcher
	invClient     inventory.Client
	client        dynamic.Interface
	openAPIGetter discovery.OpenAPISchemaInterface
	mapper        meta.RESTMapper
	infoHelper    info.Helper
}

func (a *Applier) Run(ctx context.Context, invInfo inventory.Info, objects []unstructured.Unstructured, options apply.ApplierOptions) <-chan event.Event {
	eventChannel := make(chan event.Event)
	go func() {
		defer close(eventChannel)

		pointers := lo.ToSlicePtr(objects)

		// Validate the resources to make sure we catch those problems early
		// before anything has been updated in the cluster.
		vCollector := &validation.Collector{}
		validator := &validation.Validator{
			Collector: vCollector,
			Mapper:    a.mapper,
		}
		validator.Validate(pointers)

		// Decide which objects to apply and which to prune
		applyObjs, pruneObjs, err := a.prepareObjects(invInfo, pointers)
		if err != nil {
			handleError(eventChannel, err)
			return
		}

		// Build a TaskContext for passing info between tasks
		resourceCache := cache.NewResourceCacheMap()
		taskContext := taskrunner.NewTaskContext(eventChannel, resourceCache)

		// Fetch the queue (channel) of tasks that should be executed.
		// Build list of apply validation filters.
		applyFilters := []filter.ValidationFilter{
			filters.CacheFilter{},
			filters.DependencyFilter{
				TaskContext:       taskContext,
				ActuationStrategy: actuation.ActuationStrategyApply,
				DryRunStrategy:    options.DryRunStrategy,
			},
		}
		// Build list of prune validation filters.
		pruneFilters := []filter.ValidationFilter{
			filter.PreventRemoveFilter{},
			filter.LocalNamespacesFilter{
				LocalNamespaces: localNamespaces(invInfo, object.UnstructuredSetToObjMetadataSet(pointers)),
			},
			filter.DependencyFilter{
				TaskContext:       taskContext,
				ActuationStrategy: actuation.ActuationStrategyDelete,
				DryRunStrategy:    options.DryRunStrategy,
			},
		}
		// Build list of apply mutators.
		applyMutators := []mutator.Interface{
			&mutator.ApplyTimeMutator{
				Client:        a.client,
				Mapper:        a.mapper,
				ResourceCache: resourceCache,
			},
		}
		taskBuilder := &solver.TaskQueueBuilder{
			Pruner:        a.pruner,
			DynamicClient: a.client,
			OpenAPIGetter: a.openAPIGetter,
			InfoHelper:    a.infoHelper,
			Mapper:        a.mapper,
			InvClient:     a.invClient,
			Collector:     vCollector,
			ApplyFilters:  applyFilters,
			ApplyMutators: applyMutators,
			PruneFilters:  pruneFilters,
		}
		opts := solver.Options{
			ServerSideOptions:      options.ServerSideOptions,
			ReconcileTimeout:       options.ReconcileTimeout,
			Destroy:                false,
			Prune:                  !options.NoPrune,
			DryRunStrategy:         options.DryRunStrategy,
			PrunePropagationPolicy: options.PrunePropagationPolicy,
			PruneTimeout:           options.PruneTimeout,
			InventoryPolicy:        options.InventoryPolicy,
		}

		// Build the ordered set of tasks to execute.
		taskQueue := taskBuilder.
			WithApplyObjects(applyObjs).
			WithPruneObjects(pruneObjs).
			WithInventory(invInfo).
			Build(taskContext, opts)

		// Handle validation errors
		switch options.ValidationPolicy {
		case validation.ExitEarly:
			err = vCollector.ToError()
			if err != nil {
				handleError(eventChannel, err)
				return
			}
		case validation.SkipInvalid:
			for _, err := range vCollector.Errors {
				handleValidationError(eventChannel, err)
			}
		default:
			handleError(eventChannel, fmt.Errorf("invalid ValidationPolicy: %q", options.ValidationPolicy))
			return
		}

		// Register invalid objects to be retained in the inventory, if present.
		for _, id := range vCollector.InvalidIds {
			taskContext.AddInvalidObject(id)
		}

		// Send event to inform the caller about the resources that
		// will be applied/pruned.
		eventChannel <- event.Event{
			Type: event.InitType,
			InitEvent: event.InitEvent{
				ActionGroups: taskQueue.ToActionGroups(),
			},
		}
		// Create a new TaskStatusRunner to execute the taskQueue.
		allIds := object.UnstructuredSetToObjMetadataSet(append(applyObjs, pruneObjs...))
		statusWatcher := a.statusWatcher
		// Don't disable watcher for dry runs
		// if opts.DryRunStrategy.ClientOrServerDryRun() {
		// 	statusWatcher = watcher.BlindStatusWatcher{}
		// }
		runner := taskrunner.NewTaskStatusRunner(allIds, statusWatcher)
		err = runner.Run(ctx, taskContext, taskQueue.ToChannel(), taskrunner.Options{
			EmitStatusEvents:         options.EmitStatusEvents,
			WatcherRESTScopeStrategy: options.WatcherRESTScopeStrategy,
		})
		if err != nil {
			handleError(eventChannel, err)
			return
		}
	}()
	return eventChannel
}

// prepareObjects returns the set of objects to apply and to prune or
// an error if one occurred.
func (a *Applier) prepareObjects(localInv inventory.Info, localObjs object.UnstructuredSet) (object.UnstructuredSet, object.UnstructuredSet, error) {
	if localInv == nil {
		return nil, nil, fmt.Errorf("the local inventory can't be nil")
	}

	if err := inventory.ValidateNoInventory(localObjs); err != nil {
		return nil, nil, err
	}
	// Add the inventory annotation to the resources being applied.
	for _, localObj := range localObjs {
		inventory.AddInventoryIDAnnotation(localObj, localInv)
	}

	pruneObjs := object.UnstructuredSet{}
	if localInv.Strategy() == "memory" && localInv.ID() != "" {
		inv := db.GetInventory(localInv.ID())
		invObjs, err := inv.Load()
		if err != nil {
			return nil, nil, err
		}
		ids := object.UnstructuredSetToObjMetadataSet(localObjs)
		ids = invObjs.Diff(ids)

		for _, id := range ids {
			pruneObj, err := a.getObject(id)
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).Infof("skip pruning (object: %q): resource not found", id)
					continue
				}
				return nil, nil, err
			}
			pruneObjs = append(pruneObjs, pruneObj)
		}
	}

	return localObjs, pruneObjs, nil
}

func localNamespaces(localInv inventory.Info, localObjs []object.ObjMetadata) sets.String { // nolint:staticcheck
	namespaces := sets.NewString()
	for _, obj := range localObjs {
		if obj.Namespace != "" {
			namespaces.Insert(obj.Namespace)
		}
	}
	invNamespace := localInv.Namespace()
	if invNamespace != "" {
		namespaces.Insert(invNamespace)
	}
	return namespaces
}

func handleError(eventChannel chan event.Event, err error) {
	eventChannel <- event.Event{
		Type: event.ErrorType,
		ErrorEvent: event.ErrorEvent{
			Err: err,
		},
	}
}

func handleValidationError(eventChannel chan<- event.Event, err error) {
	var tErr *validation.Error
	switch {
	case errors.As(err, &tErr):
		// handle validation error about one or more specific objects
		eventChannel <- event.Event{
			Type: event.ValidationType,
			ValidationEvent: event.ValidationEvent{
				Identifiers: tErr.Identifiers(),
				Error:       tErr,
			},
		}
	default:
		// handle general validation error (no specific object)
		eventChannel <- event.Event{
			Type: event.ValidationType,
			ValidationEvent: event.ValidationEvent{
				Error: err,
			},
		}
	}
}

func (a *Applier) getObject(id object.ObjMetadata) (*unstructured.Unstructured, error) {
	entry, exists := rccache.GetResourceCache().GetCacheEntry(id.String())

	if exists && entry != nil && entry.GetStatus() != nil {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   entry.GetStatus().Group,
			Version: entry.GetStatus().Version,
			Kind:    entry.GetStatus().Kind,
		})
		obj.SetNamespace(entry.GetStatus().Namespace)
		obj.SetName(entry.GetStatus().Name)
		if entry.GetStatus().UID != nil {
			obj.SetUID(types.UID(*entry.GetStatus().UID))
		}

		return obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, id.Name)
}
