package sync

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/alitto/pond"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
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

	wait.PollInfinite(syncDelay, func() (done bool, err error) {
		log.Info("Polling for new service updates")
		pool := pond.New(20, 100, pond.MinWorkers(20))
		for i := 0; i < engine.svcQueue.Len(); i++ {
			item, shutdown := engine.svcQueue.Get()
			if shutdown {
				return true, nil
			}
			pool.TrySubmit(func() {
				if err := engine.processItem(item); err != nil {
					log.Error(err, "found unprocessable error")
				}
			})
		}
		pool.StopAndWait()
		return false, nil

	})
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
		fmt.Printf("failed to fetch service from cache: %s, ignoring for now", err)
		return err
	}

	if Local && svc.Name == OperatorService {
		return nil
	}

	log.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	var manErr error
	manifests := make([]*unstructured.Unstructured, 0)
	if svc.DeletedAt == nil {
		manifests, manErr = engine.manifestCache.Fetch(engine.utilFactory, svc)
	}

	if manErr != nil {
		log.Error(manErr, "failed to parse manifests")
		return manErr
	}
	if err := engine.CheckNamespace(svc.Namespace); err != nil {
		log.Error(err, "failed to check namespace")
		return err
	}

	log.Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := splitObjects(id, manifests)
	if err != nil {
		return err
	}
	inv := inventory.WrapInventoryInfoObj(invObj)

	ch := engine.applier.Run(context.Background(), inv, manifests, apply.ApplierOptions{
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
		NoPrune:                true,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           time.Duration(0),
		InventoryPolicy:        inventory.PolicyAdoptIfNoInventory,
	})
	return engine.UpdateStatus(id, ch, true)
}

func splitObjects(id string, objs []*unstructured.Unstructured) (*unstructured.Unstructured, []*unstructured.Unstructured, error) {
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
		return defaultInventoryObjTemplate(id), resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return nil, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func defaultInventoryObjTemplate(id string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "inventory-" + id,
				"namespace": "plrl-deploy-operator",
				"labels": map[string]interface{}{
					common.InventoryLabel: id,
				},
			},
		},
	}
}
