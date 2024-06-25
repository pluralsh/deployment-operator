package applier

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/apply/cache"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/apply/info"

	"sigs.k8s.io/cli-utils/pkg/apply/filter"
	"sigs.k8s.io/cli-utils/pkg/apply/mutator"
	"sigs.k8s.io/cli-utils/pkg/apply/prune"
	"sigs.k8s.io/cli-utils/pkg/apply/solver"
	"sigs.k8s.io/cli-utils/pkg/apply/taskrunner"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/object/validation"

	"github.com/pluralsh/deployment-operator/pkg/applier/filters"
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

func (a *Applier) Run(ctx context.Context, invInfo inventory.Info, objects object.UnstructuredSet, options apply.ApplierOptions) <-chan event.Event {
	eventChannel := make(chan event.Event)
	go func() {
		defer close(eventChannel)
		// Validate the resources to make sure we catch those problems early
		// before anything has been updated in the cluster.
		vCollector := &validation.Collector{}
		validator := &validation.Validator{
			Collector: vCollector,
			Mapper:    a.mapper,
		}
		validator.Validate(objects)

		// Decide which objects to apply and which to prune
		applyObjs, pruneObjs, err := a.prepareObjects(invInfo, objects, options)
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
			filter.InventoryPolicyApplyFilter{
				Client:    a.client,
				Mapper:    a.mapper,
				Inv:       invInfo,
				InvPolicy: options.InventoryPolicy,
			},
			filters.CrdFilter{
				Client:    a.client,
				Mapper:    a.mapper,
				Inv:       invInfo,
				InvPolicy: options.InventoryPolicy,
			},
			filters.DependencyFilter{
				TaskContext:       taskContext,
				ActuationStrategy: actuation.ActuationStrategyApply,
				DryRunStrategy:    options.DryRunStrategy,
			},
			filters.CacheFilter{},
		}
		// Build list of prune validation filters.
		pruneFilters := []filter.ValidationFilter{
			filter.PreventRemoveFilter{},
			filter.InventoryPolicyPruneFilter{
				Inv:       invInfo,
				InvPolicy: options.InventoryPolicy,
			},
			filter.LocalNamespacesFilter{
				LocalNamespaces: localNamespaces(invInfo, object.UnstructuredSetToObjMetadataSet(objects)),
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
func (a *Applier) prepareObjects(localInv inventory.Info, localObjs object.UnstructuredSet,
	o apply.ApplierOptions) (object.UnstructuredSet, object.UnstructuredSet, error) {
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
	// If the inventory uses the Name strategy and an inventory ID is provided,
	// verify that the existing inventory object (if there is one) has an ID
	// label that matches.
	// TODO(seans): This inventory id validation should happen in destroy and status.
	if localInv.Strategy() == inventory.NameStrategy && localInv.ID() != "" {
		prevInvObjs, err := a.invClient.GetClusterInventoryObjs(localInv)
		if err != nil {
			return nil, nil, err
		}
		if len(prevInvObjs) > 1 {
			panic(fmt.Errorf("found %d inv objects with Name strategy", len(prevInvObjs)))
		}
		if len(prevInvObjs) == 1 {
			invObj := prevInvObjs[0]
			val := invObj.GetLabels()[common.InventoryLabel]
			if val != localInv.ID() {
				return nil, nil, fmt.Errorf("inventory-id of inventory object in cluster doesn't match provided id %q", localInv.ID())
			}
		}
	}
	pruneObjs, err := a.pruner.GetPruneObjs(localInv, localObjs, prune.Options{
		DryRunStrategy: o.DryRunStrategy,
	})
	if err != nil {
		return nil, nil, err
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
