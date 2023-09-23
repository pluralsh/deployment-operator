package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/klog/v2/klogr"
)

const (
	syncDelay = 5 * time.Second
)

func (engine *Engine) ControlLoop() {
	if engine.deathChan != nil {
		defer func() {
			if r := recover(); r != nil {
				engine.deathChan <- r
			}
		}()
	}

	log := klogr.New()
	for {
		id := <-engine.svcChan
		svc, err := engine.svcCache.Get(id)
		if err != nil {
			fmt.Printf("failed to fetch service from cache: %s, ignoring for now", err)
			continue
		}

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
			continue
		}

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

		time.Sleep(time.Duration(syncDelay))
	}
}
