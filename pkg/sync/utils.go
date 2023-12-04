package sync

import (
	"sort"
	"time"

	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/pkg/hook"
	"github.com/pluralsh/polly/containers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

func GenDefaultInventoryUnstructuredMap(namespace, name, id string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					common.InventoryLabel: id,
				},
			},
		},
	}
}

func GetInventoryName(id string) string {
	return "inventory-" + id
}

func GetDefaultPruneOptions() apply.DestroyerOptions {
	return apply.DestroyerOptions{
		InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
		DryRunStrategy:          common.DryRunNone,
		DeleteTimeout:           20 * time.Second,
		DeletePropagationPolicy: metav1.DeletePropagationBackground,
		EmitStatusEvents:        true,
		ValidationPolicy:        1,
	}
}

func GetDefaultApplierOptions() apply.ApplierOptions {
	return apply.ApplierOptions{
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
		ReconcileTimeout: 10 * time.Second,
		// If we are not waiting for status, tell the applier to not
		// emit the events.
		EmitStatusEvents:       true,
		NoPrune:                false,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           20 * time.Second,
		InventoryPolicy:        inventory.PolicyAdoptAll,
	}
}

func GetHooks(obj []*unstructured.Unstructured) []hook.Hook {
	hooks := make([]hook.Hook, 0)
	for _, h := range obj {
		hooks = append(hooks, hook.Hook{
			Weight:         hook.Weight(h),
			Types:          containers.ToSet(hook.Types(h)),
			DeletePolicies: containers.ToSet(hook.DeletePolicies(h)),
			Kind:           h.GetObjectKind(),
			Object:         h,
		})
	}
	sort.Slice(hooks, func(i, j int) bool {
		kindI := hooks[i].Kind.GroupVersionKind()
		kindJ := hooks[j].Kind.GroupVersionKind()
		return kindI.Kind < kindJ.Kind
	})
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Weight < hooks[j].Weight
	})

	return hooks
}

func GetDeletePolicyHooks(hooks []hook.Hook) (deleteBefore, deleteFailed, deleteSucceeded object.ObjMetadataSet, err error) {
	deleteBefore = object.ObjMetadataSet{}
	deleteFailed = object.ObjMetadataSet{}
	deleteSucceeded = object.ObjMetadataSet{}
	for _, h := range hooks {
		if h.DeletePolicies.Has(hook.BeforeHookCreation) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return nil, nil, nil, err
			}
			deleteBefore = append(deleteBefore, obj)
		}
		if h.DeletePolicies.Has(hook.HookFailed) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return nil, nil, nil, err
			}
			deleteFailed = append(deleteFailed, obj)
		}
		if h.DeletePolicies.Has(hook.HookSucceeded) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return nil, nil, nil, err
			}
			deleteSucceeded = append(deleteSucceeded, obj)
		}
	}
	return
}
