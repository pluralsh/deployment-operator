package sync

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	console "github.com/pluralsh/console-client-go"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

//const (
//	// The field manager name for the ones agentk owns, see
//	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
//	fieldManager = "application/apply-patch"
//)

func (engine *Engine) GateControlLoop() {
	if engine.deathChan != nil {
		defer func() {
			if r := recover(); r != nil {
				engine.deathChan <- r
				fmt.Printf("panic: %s\n", string(debug.Stack()))
			}
		}()
	}

	for i := 0; i < workerCount; i++ {
		go engine.gateWorkerLoop()
	}
}

func (engine *Engine) gateWorkerLoop() {
	log.Info("starting sync worker for gates")
	for {
		log.Info("polling for new gate updates")
		gate, shutdown := engine.gateQueue.Get()
		if shutdown {
			log.Info("shutting down worker")
			break
		}
		err := engine.processGate(gate)
		if err != nil {
			log.Error(err, "process gate")
			id := gate.(string)
			if id != "" {
				engine.UpdateErrorStatus(id, err)
			}
		}
		time.Sleep(syncDelay)
	}
}

func (engine *Engine) processGate(item interface{}) error {
	//state truth is still always in the console!
	//so logic should most likely be
	//1. get the PipelineGate
	//2. if status is OPEN, then sync the gate on the cluster and set status to PENDING
	//  - if the gate is already synced into the cluster, i.e. CRD created, then do nothing, this can be the case if the gate was already synced,
	//	but reconciliation wasn't quick enough failed to update the status
	//  - only way to check if the gate is synced is to check if the CRD object exists -> k8s API call
	//3. if status is CLOSED, then do nothing, reconciliation will take care of clean up
	//4. if status is PENDING, it has already been synced, so do nothing
	defer engine.gateQueue.Done(item)
	gate, ok := item.(console.PipelineGateFragment)
	if !ok {
		// handle if assertion fails (shouldn't happen)
		err := fmt.Errorf("unexpected type: %T", item)
		log.Error(err, "failed to process gate item: %s, ignoring for now")
		return err
	}

	log.Info("attempting to sync gate", "id", gate.ID)
	// TODO: shouldn't it always be in the gate cache? and if it's not do we put it there?
	//gate, err := engine.gateCache.Get(gate.ID, gate)
	//if err != nil {
	//	fmt.Printf("failed to fetch gate: %s, ignoring for now", err)
	//	return err
	//}

	log.Info("syncing gate", "name", gate.Name)

	gateCR, err := engine.client.ParsePipelineGateCR(&gate)
	// TODO error handling
	if err != nil {
		log.Error(err, "failed to parse gate CR from gate fragment")
		return err
	}

	// apply the gate CR to the cluster
	engine.clientset.PipelineV1alpha1().PipelineGates(gateCR.Namespace).Apply(context.Background(), gateCR, metav1.ApplyOptions{})

	var manErr error
	manifests, manErr := engine.manifestCache.Fetch(engine.utilFactory, gate)
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

	vcache := manis.VersionCache(manifests)

	if gate.DeletedAt != nil {
		log.Info("Deleting service", "name", gate.Name, "namespace", gate.Namespace)
		ch := engine.destroyer.Run(ctx, inv, apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationForeground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})
		return engine.UpdatePruneStatus(id, gate.Name, gate.Namespace, ch, len(manifests), vcache)
	}

	log.Info("Apply service", "name", gate.Name, "namespace", gate.Namespace)
	if err := engine.CheckNamespace(gate.Namespace); err != nil {
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

	return engine.UpdateApplyStatus(id, gate.Name, gate.Namespace, ch, false, vcache)
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
