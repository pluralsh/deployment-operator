package sync

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

const (
	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

func (engine *Engine) ControlLoop() {
	if engine.deathChan != nil {
		defer func() {
			if r := recover(); r != nil {
				engine.deathChan <- r
				fmt.Printf("panic: %s\n", string(debug.Stack()))
			}
		}()
	}

	engine.RegisterHandlers()
	for i := 0; i < workerCount; i++ {
		go engine.workerLoop()
	}
}

func (engine *Engine) workerLoop() {
	log.Info("starting sync worker")
	for {
		log.Info("polling for new service updates")
		item, shutdown := engine.svcQueue.Get()
		if shutdown {
			log.Info("shutting down worker")
			break
		}
		err := engine.processItem(item)
		if err != nil {
			log.Error(err, "process item")
			id := item.(string)
			if id != "" {
				engine.UpdateErrorStatus(id, err)
			}
		}
		time.Sleep(syncDelay)
	}
}

func (engine *Engine) processItem(item interface{}) error {
	defer engine.svcQueue.Done(item)
	id := item.(string)

	if id == "" {
		return nil
	}

	log.Info("attempting to sync service", "id", id)
	engine.syncing = id
	svc, err := engine.svcCache.Get(id)
	if err != nil {
		fmt.Printf("failed to fetch service: %s, ignoring for now", err)
		return err
	}

	log.Info("local flag", "is:", Local)
	if Local && svc.Name == OperatorService {
		return nil
	}

	log.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	var manErr error
	manifests := make([]*unstructured.Unstructured, 0)
	manifests, manErr = engine.manifestCache.Fetch(engine.utilFactory, svc)
	if manErr != nil {
		log.Error(manErr, "failed to parse manifests")
		return manErr
	}
	log.Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := engine.splitObjects(id, manifests)
	if err != nil {
		return err
	}
	inv := inventory.WrapInventoryInfoObj(invObj)

	// deadline := time.Now().Add(engine.processingTimeout)
	// ctx, cancelCtx := context.WithDeadline(context.Background(), deadline)
	// defer cancelCtx()
	ctx := context.Background()

	if svc.DeletedAt != nil {
		log.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := engine.destroyer.Run(ctx, inv, apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationForeground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})
		return engine.UpdatePruneStatus(id, svc.Name, svc.Namespace, ch, len(manifests))
	}

	vcache := manis.VersionCache(manifests)
	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err := engine.CheckNamespace(svc.Namespace); err != nil {
		log.Error(err, "failed to check namespace")
		return err
	}
	ch := engine.applier.Run(ctx, inv, manifests, apply.ApplierOptions{
		ServerSideOptions: common.ServerSideOptions{
			// It's supported since Kubernetes 1.16, so there should be no reason not to use it.
			// https://kubernetes.io/docs/reference/using-api/server-side-apply/
			ServerSideApply: true,
			// GitOps repository is the source of truth and that's what we are applying, so overwrite any conflicts.
			// https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
			ForceConflicts: true,
			// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
			FieldManager: fieldManager,
		},
		ReconcileTimeout: 10 * time.Second,
		// If we are not waiting for status, tell the applier to not
		// emit the events.
		EmitStatusEvents:       true,
		NoPrune:                false,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           20 * time.Second,
		InventoryPolicy:        inventory.PolicyAdoptAll,
	})

	return engine.UpdateApplyStatus(id, svc.Name, svc.Namespace, ch, false, vcache)
}

func (engine *Engine) splitObjects(id string, objs []*unstructured.Unstructured) (*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	invs := make([]*unstructured.Unstructured, 0, 1)
	resources := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if inventory.IsInventoryObject(obj) {
			invs = append(invs, obj)
		} else {
			resources = append(resources, obj)
		}
	}
	switch len(invs) {
	case 0:
		invObj, err := engine.defaultInventoryObjTemplate(id)
		if err != nil {
			return nil, nil, err
		}
		return invObj, resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return nil, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func (engine *Engine) defaultInventoryObjTemplate(id string) (*unstructured.Unstructured, error) {
	name := "inventory-" + id
	namespace := "plrl-deploy-operator"

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					common.InventoryLabel: id,
				},
			},
		},
	}, nil
}
