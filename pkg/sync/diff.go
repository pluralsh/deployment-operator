package sync

import (
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (engine *Engine) diff(manifests []*unstructured.Unstructured, namespace, svcId string) (*diff.DiffResultList, error) {
	liveObjs, err := engine.cache.GetManagedLiveObjs(manifests, isManaged(svcId))
	if err != nil {
		return nil, err
	}

	reconciliation := sync.Reconcile(manifests, liveObjs, namespace, engine.cache)
	return diff.DiffArray(reconciliation.Target, reconciliation.Live, diff.WithManager(SSAManager), diff.WithLogr(log))
}
