package sync

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/klog/v2/klogr"
)

var (
	log = klogr.New()
)

const (
	syncDelay = 5 * time.Second
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

	for {
		log.Info("Polling for new service updates")

		item, shutdown := engine.svcQueue.Get()
		if shutdown {
			break
		}

		if err := engine.processItem(item); err != nil {
			log.Error(err, "found unprocessable error")
		}

		engine.syncing = ""

		time.Sleep(time.Duration(syncDelay))
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
		fmt.Printf("failed to fetch service from cache: %s, ignoring for now", err)
		return err
	}
	log.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	var manErr error
	results := make([]common.ResourceSyncResult, 0)
	manifests := make([]*unstructured.Unstructured, 0)
	if svc.DeletedAt == nil {
		manifests, manErr = engine.manifestCache.Fetch(svc)
	}

	if manErr != nil {
		if err := engine.updateStatus(svc.ID, results, errorAttributes("manifests", manErr)); err != nil {
			log.Error(err, "Failed to update service status, ignoring for now")
		}
		log.Error(manErr, "failed to parse manifests")
		return manErr
	}

	log.Info("Syncing manifests", "count", len(manifests))

	addAnnotations(manifests, svc.ID)
	results, err = engine.engine.Sync(
		context.Background(),
		manifests,
		isManaged(svc.ID),
		svc.Revision.ID,
		svc.Namespace,
		sync.WithPrune(true),
		sync.WithLogr(log),
		sync.WithSyncWaveHook(delayBetweenSyncWaves),
		sync.WithServerSideApplyManager(SSAManager),
		sync.WithServerSideApply(true),
		sync.WithNamespaceModifier(func(managedNs, liveNs *unstructured.Unstructured) (bool, error) {
			return true, nil
		}),
	)
	if err != nil {
		log.Error(err, "encountered sync error")
	}

	if err := engine.updateStatus(svc.ID, results, errorAttributes("sync", err)); err != nil {
		log.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
}
