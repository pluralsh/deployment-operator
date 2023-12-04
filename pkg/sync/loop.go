package sync

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	console "github.com/pluralsh/console-client-go"

	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/hook"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	if svc.DeletedAt != nil {
		// delete hooks
		if err := engine.deleteHooks(ctx, svc.Namespace, svc.Name, id, hook.PreInstall); err != nil {
			return err
		}
		if err := engine.deleteHooks(ctx, svc.Namespace, svc.Name, id, hook.PostInstall); err != nil {
			return err
		}

		log.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := engine.destroyer.Run(ctx, inv, GetDefaultPruneOptions())
		return engine.UpdatePruneStatus(id, svc.Name, svc.Namespace, ch, len(manifests), vcache)
	}

	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err := engine.CheckNamespace(svc.Namespace); err != nil {
		log.Error(err, "failed to check namespace")
		return err
	}

	// mange pre-install hooks
	preInstallComponents, err := engine.manageHooks(ctx, hook.PreInstall, svc.Namespace, svc.Name, id, hooks)
	if err != nil {
		return err
	}

	for _, c := range preInstallComponents {
		if *c.State != console.ComponentStateRunning {
			// wait until hooks are completed
			if err := engine.updateStatus(id, preInstallComponents, errorAttributes("sync", err)); err != nil {
				log.Error(err, "Failed to update service status, ignoring for now")
			}
			return nil
		}
	}

	log.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	ch := engine.applier.Run(ctx, inv, manifests, GetDefaultApplierOptions())
	components, err := engine.UpdateApplyStatus(id, svc.Name, svc.Namespace, ch, false, vcache)
	if err != nil {
		return err
	}
	components = append(components, preInstallComponents...)

	installed, err := engine.isInstalled(id, manifests)
	if err != nil {
		return err
	}
	if installed {
		postInstallComponents, err := engine.manageHooks(ctx, hook.PostInstall, svc.Namespace, svc.Name, id, hooks)
		if err != nil {
			return err
		}
		components = append(components, postInstallComponents...)
	}

	if err := engine.updateStatus(id, components, errorAttributes("sync", err)); err != nil {
		log.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
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
