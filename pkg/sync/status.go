package sync

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func errorAttributes(source string, err error) *console.ServiceErrorAttributes {
	if err == nil {
		return nil
	}

	return &console.ServiceErrorAttributes{
		Source:  source,
		Message: err.Error(),
	}
}

func (engine *Engine) updateStatus(id string, results []common.ResourceSyncResult, err *console.ServiceErrorAttributes) error {
	errs := make([]*console.ServiceErrorAttributes, 0)
	if err != nil {
		errs = append(errs, err)
	}

	components, compErr := engine.collectComponents(id, results)
	if err != nil {
		errs = append(errs, errorAttributes("reconciliation", compErr))
	}
	errs = lo.Filter(errs, func(e *console.ServiceErrorAttributes, ind int) bool { return e != nil })

	return engine.client.UpdateComponents(id, components, errs)
}

func (engine *Engine) collectComponents(id string, results []common.ResourceSyncResult) ([]*console.ComponentAttributes, error) {
	res := make([]*console.ComponentAttributes, 0)
	seen := map[kube.ResourceKey]bool{}
	liveObjs, err := engine.cache.GetManagedLiveObjs([]*unstructured.Unstructured{}, isManaged(id))
	if err != nil {
		return res, err
	}

	for _, syncResult := range results {
		component := fromSyncResult(syncResult)
		obj, ok := liveObjs[syncResult.ResourceKey]
		if ok {
			seen[syncResult.ResourceKey] = true
			gvk := obj.GroupVersionKind()
			component.State = toStatus(obj)
			component.Version = gvk.Version
			component.Group = gvk.Group
			component.Kind = gvk.Kind
		}
		res = append(res, component)
	}

	for k, obj := range liveObjs {
		if seen[k] {
			continue
		}
		gvk := obj.GroupVersionKind()
		res = append(res, &console.ComponentAttributes{
			State:     toStatus(obj),
			Group:     gvk.Group,
			Kind:      gvk.Kind,
			Version:   gvk.Version,
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
			Synced:    true,
		})
	}

	return res, nil
}

func fromSyncResult(res common.ResourceSyncResult) *console.ComponentAttributes {
	rk := res.ResourceKey
	return &console.ComponentAttributes{
		Group:     rk.Group,
		Kind:      rk.Kind,
		Namespace: rk.Namespace,
		Name:      rk.Name,
		Synced:    res.Status == common.ResultCodeSynced,
	}
}

func toStatus(obj *unstructured.Unstructured) *console.ComponentState {
	h, _ := health.GetResourceHealth(obj, nil)
	if h == nil {
		return nil
	}

	if h.Status == health.HealthStatusDegraded {
		return lo.ToPtr(console.ComponentStateFailed)
	}

	if h.Status == health.HealthStatusHealthy {
		return lo.ToPtr(console.ComponentStateRunning)
	}

	return lo.ToPtr(console.ComponentStatePending)
}
