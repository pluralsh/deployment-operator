package sync

import (
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
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

	return engine.client.UpdateComponents(id, components, errs)
}

func (engine *Engine) collectComponents(id string, results []common.ResourceSyncResult) ([]*console.ComponentAttributes, error) {
	res := make([]*console.ComponentAttributes, 0)
	liveObjs, err := engine.cache.GetManagedLiveObjs([]*unstructured.Unstructured{}, isManagedRecursive(id))
	if err != nil {
		return res, err
	}

	for _, syncResult := range results {
		component := fromSyncResult(syncResult)
		obj, ok := liveObjs[syncResult.ResourceKey]
		if ok {
			gvk := obj.GroupVersionKind()
			component.State = lo.ToPtr(toStatus(obj))
			component.Version = gvk.Version
			component.Group = gvk.Group
			component.Kind = gvk.Kind
		}
		res = append(res, component)
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

func toStatus(obj *unstructured.Unstructured) console.ComponentState {
	h, _ := health.GetResourceHealth(obj, nil)
	if h == nil {
		return console.ComponentStateFailed
	}

	if h.Status == health.HealthStatusDegraded {
		return console.ComponentStateFailed
	}

	if h.Status == health.HealthStatusHealthy {
		return console.ComponentStateRunning
	}

	return console.ComponentStatePending
}
