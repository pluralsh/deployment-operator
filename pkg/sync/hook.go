package sync

import (
	"context"
	"fmt"
	"github.com/pluralsh/polly/containers"

	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/pluralsh/deployment-operator/pkg/hook"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func (engine *Engine) managePreInstallHooks(ctx context.Context, namespace, name, id string, hookObjects []*unstructured.Unstructured, objects []*unstructured.Unstructured) error {
	if len(hookObjects) == 0 {
		return nil
	}
	preInstallHooks := make([]hook.Hook, 0)
	hooks := GetHooks(hookObjects)
	for _, h := range hooks {
		if h.Types.Has(hook.PreInstall) {
			preInstallHooks = append(preInstallHooks, h)
		}
	}

	return engine.preInstallHooks(ctx, namespace, name, id, preInstallHooks)
}

func (engine *Engine) preInstallHooks(ctx context.Context, namespace, name, id string, hooks []hook.Hook) error {
	inv := inventory.WrapInventoryInfoObj(hookInventoryObjTemplate(id, hook.PreInstall))
	var manifests []*unstructured.Unstructured
	hookSet := map[object.ObjMetadata]*unstructured.Unstructured{}
	deleteBefore := object.ObjMetadataSet{}
	deleteFailed := object.ObjMetadataSet{}
	deleteSucceeded := object.ObjMetadataSet{}
	for _, h := range hooks {
		if h.DeletePolicies.Has(hook.BeforeHookCreation) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return err
			}
			deleteBefore = append(deleteBefore, obj)
		} else if h.DeletePolicies.Has(hook.HookFailed) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return err
			}
			deleteFailed = append(deleteFailed, obj)
		} else if h.DeletePolicies.Has(hook.HookSucceeded) {
			obj, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return err
			}
			deleteSucceeded = append(deleteSucceeded, obj)
		}
		objKey, err := object.RuntimeToObjMeta(h.Object)
		if err != nil {
			return err
		}
		hookSet[objKey] = h.Object
	}

	if err := engine.updateHookInventory(ctx, namespace, name, id, hookSet, deleteBefore); err != nil {
		return err
	}

	for _, obj := range hookSet {
		manifests = append(manifests, obj)
	}
	ch := engine.applier.Run(ctx, inv, manifests, GetDefaultApplierOptions())
	statsCollector, statusCollector, err := GetStatusCollector(ch, false)
	if err != nil {
		return err
	}

	if err := FormatSummary(namespace, name, *statsCollector); err != nil {
		return err
	}
	toDelete := object.ObjMetadataSet{}
	for k, v := range statusCollector.latestStatus {
		if v.PollResourceInfo.Status == status.FailedStatus {
			if deleteFailed.Contains(k) {
				toDelete = append(toDelete, k)
			}
		}
		if v.PollResourceInfo.Status == status.FailedStatus {
			if deleteSucceeded.Contains(k) {
				toDelete = append(toDelete, k)
			}
		}
	}

	return engine.updateHookInventory(ctx, namespace, name, id, hookSet, toDelete)
}

func (engine *Engine) updateHookInventory(ctx context.Context, namespace, name, id string, allHooks map[object.ObjMetadata]*unstructured.Unstructured, deletePolicyHooks ...object.ObjMetadataSet) error {
	inv := inventory.WrapInventoryInfoObj(hookInventoryObjTemplate(id, hook.PreInstall))
	var manifests []*unstructured.Unstructured
	client, err := engine.utilFactory.KubernetesClientSet()
	if err != nil {
		return err
	}
	invMap, err := client.CoreV1().ConfigMaps(inventoryFileNamespace).Get(ctx, getInvHookName(id, hook.PreInstall), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	if invMap == nil {
		return nil
	}
	if invMap.Data == nil {
		return nil
	}

	invElem, err := ConvertInventoryMap(invMap)
	if err != nil {
		return err
	}
	invObj := inventory.WrapInventoryObj(invElem)
	invObjSet, err := invObj.Load()
	if err != nil {
		return err
	}

	for _, v := range deletePolicyHooks {
		diffObj := invObjSet.Diff(v)
		for _, objKey := range diffObj {
			manifests = append(manifests, allHooks[objKey])
		}
	}
	manifestSet := containers.ToSet[*unstructured.Unstructured](manifests)
	// delete previous resources
	ch := engine.applier.Run(ctx, inv, manifestSet.List(), GetDefaultApplierOptions())
	statsCollector, statusCollector, err := GetStatusCollector(ch, false)
	if err != nil {
		return err
	}
	if err := FormatSummary(namespace, name, *statsCollector); err != nil {
		return err
	}
	for k, v := range statusCollector.latestStatus {
		if v.PollResourceInfo.Status == status.FailedStatus {
			return fmt.Errorf("failed to delete %v", k)
		}
	}
	return nil
}

func (engine *Engine) isInstalled(id string, objects []*unstructured.Unstructured) (bool, error) {
	inventoryName := GetInventoryName(id)
	client, err := engine.utilFactory.KubernetesClientSet()
	if err != nil {
		return false, err
	}
	invConfigMap, err := client.CoreV1().ConfigMaps(inventoryFileNamespace).Get(context.Background(), inventoryName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if len(invConfigMap.Data) == len(objects) {
		return true, nil
	}

	return false, nil
}

func hookInventoryObjTemplate(id string, t hook.Type) *unstructured.Unstructured {
	name := getInvHookName(id, t)
	return GenDefaultInventoryUnstructuredMap(inventoryFileNamespace, name, name)
}

func getInvHookName(id string, t hook.Type) string {
	return fmt.Sprintf("%s-hook-%s", t, id)
}
