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

	inventoryFileNamespace = "plrl-deploy-operator"
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
	manifests, hooks, manErr := engine.manifestCache.Fetch(engine.utilFactory, svc)
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
	delete := false
	if svc.DeletedAt != nil {
		delete = true
	}

	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err := engine.CheckNamespace(svc.Namespace); err != nil {
		log.Error(err, "failed to check namespace")
		return err
	}

	hookComponents, err := engine.managePreInstallHooks(ctx, svc.Namespace, svc.Name, id, hooks, delete)
	if err != nil {
		return err
	}

	if delete {
		log.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := engine.destroyer.Run(ctx, inv, GetDefaultPruneOptions())
		return engine.UpdatePruneStatus(id, svc.Name, svc.Namespace, ch, len(manifests), vcache)
	}

	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	ch := engine.applier.Run(ctx, inv, manifests, GetDefaultApplierOptions())
	return engine.UpdateApplyStatus(id, svc.Name, svc.Namespace, ch, false, vcache, hookComponents)
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
	return GenDefaultInventoryUnstructuredMap(inventoryFileNamespace, GetInventoryName(id), id), nil
}
