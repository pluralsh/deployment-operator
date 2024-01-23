package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	"github.com/samber/lo"
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
			if id != "" && !errors.Is(err, plrlerrors.ErrExpected) {
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

	log.Info("local", "flag", Local)
	if Local && svc.Name == OperatorService {
		return nil
	}

	log.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	var manErr error
	manifests, manErr := engine.manifestCache.Fetch(engine.utilFactory, svc)
	if manErr != nil {
		log.Error(manErr, "failed to parse manifests")
		return manErr
	}

	manifests = postProcess(manifests)

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

	vcache := manis.VersionCache(manifests)

	if svc.DeletedAt != nil {
		log.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := engine.destroyer.Run(ctx, inv, apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationBackground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})
		return engine.UpdatePruneStatus(id, svc.Name, svc.Namespace, ch, len(manifests), vcache)
	}

	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err := engine.CheckNamespace(svc.Namespace); err != nil {
		log.Error(err, "failed to check namespace")
		return err
	}

	options := apply.ApplierOptions{
		ServerSideOptions: common.ServerSideOptions{
			ServerSideApply: true,
			ForceConflicts:  true,
			FieldManager:    fieldManager,
		},
		ReconcileTimeout:       10 * time.Second,
		EmitStatusEvents:       true,
		NoPrune:                false,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           20 * time.Second,
		InventoryPolicy:        inventory.PolicyAdoptAll,
	}

	// ch := engine.applier.Run(ctx, inv, manifests, options)
	// if changed, err := engine.DryRunStatus(id, svc.Name, svc.Namespace, ch, vcache); !changed || err != nil {
	// 	return err
	// }
	options.DryRunStrategy = common.DryRunNone
	ch := engine.applier.Run(ctx, inv, manifests, options)
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

func postProcess(mans []*unstructured.Unstructured) []*unstructured.Unstructured {
	return lo.Map(mans, func(man *unstructured.Unstructured, ind int) *unstructured.Unstructured {
		if man.GetKind() != "CustomResourceDefinition" {
			return man
		}

		annotations := man.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[common.LifecycleDeleteAnnotation] = common.PreventDeletion
		man.SetAnnotations(annotations)
		return man
	})
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
