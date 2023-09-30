package sync

import (
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (engine *Engine) diff(manifests []*unstructured.Unstructured, namespace, svcId string) (*diff.DiffResultList, error) {
	liveObjs, err := engine.cache.GetManagedLiveObjs(manifests, isManaged(svcId))
	if err != nil {
		return nil, err
	}

	live := lo.Map(manifests, func(r *unstructured.Unstructured, ind int) *unstructured.Unstructured {
		key := kube.GetResourceKey(r)
		key.Namespace = namespace
		if v, ok := liveObjs[key]; ok {
			return v
		}
		return nil
	})

	return diff.DiffArray(manifests, live, diff.WithManager(SSAManager))
}
