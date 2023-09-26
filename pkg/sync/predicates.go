package sync

import (
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func addAnnotations(mans []*unstructured.Unstructured, svcId string) {
	for i := range mans {
		annotations := mans[i].GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[SyncAnnotation] = svcId
		annotations[SyncShaAnnotation] = Sha(svcId, kube.GetResourceKey(mans[i]))
		mans[i].SetAnnotations(annotations)
	}
}

func isManaged(svcId string) func(*cache.Resource) bool {
	return func(r *cache.Resource) bool {
		res, ok := r.Info.(*Resource)
		return ok && res != nil && res.ServiceId == svcId && res.Sha == Sha(svcId, r.ResourceKey())
	}
}

func isManagedRecursive(svcId string) func(*cache.Resource) bool {
	return func(r *cache.Resource) bool {
		res, ok := r.Info.(*Resource)
		return ok && res != nil && res.ServiceId == svcId
	}
}

func svcId(resource *cache.Resource) *string {
	res, ok := resource.Info.(*Resource)
	if !ok || res == nil {
		return nil
	}

	id := res.ServiceId
	return &id
}
