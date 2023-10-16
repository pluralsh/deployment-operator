package sync

import (
	"context"
	"fmt"
	"github.com/alitto/pond"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"runtime/debug"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/printers"
	"time"
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

	//wait.PollInfinite(syncDelay, func() (done bool, err error) {
	for {
		log.Info("Polling for new service updates")
		pool := pond.New(20, 100, pond.MinWorkers(20))
		for i := 0; i < engine.svcQueue.Len(); i++ {
			item, shutdown := engine.svcQueue.Get()
			if shutdown {
				//return true, nil
				break
			}
			pool.TrySubmit(func() {
				if err := engine.processItem(item); err != nil {
					log.Error(err, "found unprocessable error")
				}
			})
		}
		pool.StopAndWait()
		//return false, nil
		time.Sleep(syncDelay)
	}
	//})
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
		ReconcileTimeout: time.Duration(0),
		// If we are not waiting for status, tell the applier to not
		// emit the events.
		EmitStatusEvents:       true,
		NoPrune:                true,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           time.Duration(0),
		InventoryPolicy:        inventory.PolicyMustMatch,
	})
	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	printer := printers.GetPrinter(printers.DefaultPrinter(), ioStreams)
	return printer.Print(ch, common.DryRunNone, true)
	//addAnnotations(manifests, svc.ID)

	/*	diff, err := engine.diff(manifests, svc.Namespace, svc.ID)
		checkModifications := sync.WithResourceModificationChecker(true, diff)
		if err != nil {
			log.Error(err, "could not build diff list, ignoring for now")
			checkModifications = sync.WithResourceModificationChecker(false, nil)
		}*/

	/*	results, err = engine.engine.Sync(
		context.Background(),
		manifests,
		isManaged(svc.ID),
		svc.Revision.ID,
		svc.Namespace,
		sync.WithPrune(true),
		checkModifications,
		sync.WithPrunePropagationPolicy(lo.ToPtr(metav1.DeletePropagationBackground)),
		sync.WithLogr(log),
		sync.WithSyncWaveHook(delayBetweenSyncWaves),
		sync.WithServerSideApplyManager(SSAManager),
		sync.WithServerSideApply(true),
		sync.WithNamespaceModifier(func(managedNs, liveNs *unstructured.Unstructured) (bool, error) {
			return managedNs != nil && liveNs == nil, nil
		}),
	)*/

	/*	if err := engine.updateStatus(svc.ID, results, errorAttributes("sync", err)); err != nil {
		log.Error(err, "Failed to update service status, ignoring for now")
	}*/

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
