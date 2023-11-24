package sync

import (
	"context"
	"fmt"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/hook"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/polly/containers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func (engine *Engine) manageHooks(ctx context.Context, hookType hook.Type, namespace, name, id string, hookObjects []*unstructured.Unstructured) ([]*console.ComponentAttributes, error) {
	if len(hookObjects) == 0 {
		return nil, nil
	}
	vcache := manis.VersionCache(hookObjects)
	hookSet := map[object.ObjMetadata]*unstructured.Unstructured{}
	typedHooks := make([]hook.Hook, 0)
	client, err := engine.utilFactory.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	hookInventory := hook.NewInventory(ctx, inventoryFileNamespace, getInvHookName(id, hookType), client)
	deleted, err := hookInventory.GetDeleted()
	if err != nil {
		return nil, err
	}

	hooks := GetHooks(hookObjects)
	for _, h := range hooks {
		if h.Types.Has(hookType) {
			objKey, err := object.RuntimeToObjMeta(h.Object)
			if err != nil {
				return nil, err
			}
			if !deleted.Contains(objKey) {
				hookSet[objKey] = h.Object
				typedHooks = append(typedHooks, h)
			}
		}
	}

	// nothing to update
	if len(typedHooks) == 0 {
		return nil, nil
	}

	inv := inventory.WrapInventoryInfoObj(hookInventoryObjTemplate(id, hookType))
	return engine.hooksHandler(ctx, namespace, name, typedHooks, hookSet, hookInventory, vcache, inv)
}

func (engine *Engine) deleteHooks(ctx context.Context, namespace, name, id string, hookType hook.Type) error {
	inv := inventory.WrapInventoryInfoObj(hookInventoryObjTemplate(id, hookType))
	ch := engine.destroyer.Run(ctx, inv, GetDefaultPruneOptions())
	statsCollector, _, err := GetStatusCollector(ch, false)
	if err != nil {
		return err
	}
	if err := FormatSummary(namespace, name, *statsCollector); err != nil {
		return err
	}

	if statsCollector.DeleteStats.Failed > 0 {
		return fmt.Errorf("failed to delete hooks")
	}
	return nil
}

func (engine *Engine) hooksHandler(ctx context.Context, namespace, name string, hooks []hook.Hook, hookSet map[object.ObjMetadata]*unstructured.Unstructured, hookInventory *hook.Inventory, vcache map[manis.GroupName]string, inv inventory.Info) ([]*console.ComponentAttributes, error) {
	var manifests []*unstructured.Unstructured
	deleteBefore, deleteFailed, deleteSucceeded, err := GetDeletePolicyHooks(hooks)
	if err != nil {
		return nil, err
	}

	// delete before
	if len(deleteBefore) > 0 {
		if err := engine.updateHookInventory(ctx, inv, namespace, name, hookSet, hookInventory, deleteBefore); err != nil {
			return nil, err
		}
	}
	for _, obj := range hookSet {
		manifests = append(manifests, obj)
	}
	ch := engine.applier.Run(ctx, inv, manifests, GetDefaultApplierOptions())
	statsCollector, statusCollector, err := GetStatusCollector(ch, false)
	if err != nil {
		return nil, err
	}

	if err := FormatSummary(namespace, name, *statsCollector); err != nil {
		return nil, err
	}

	components := []*console.ComponentAttributes{}
	toDelete := object.ObjMetadataSet{}
	for k, v := range statusCollector.latestStatus {
		if v.PollResourceInfo.Status == status.FailedStatus {
			if deleteFailed.Contains(k) {
				toDelete = append(toDelete, k)
			}
		}
		if v.PollResourceInfo.Status == status.CurrentStatus {
			if deleteSucceeded.Contains(k) {
				toDelete = append(toDelete, k)
			}
		}

		consoleAttr := fromSyncResult(v, vcache)
		if consoleAttr != nil {
			components = append(components, consoleAttr)
		}
	}

	// delete failed and succeeded
	if len(deleteSucceeded) > 0 || len(deleteFailed) > 0 {
		if err := engine.updateHookInventory(ctx, inv, namespace, name, hookSet, hookInventory, toDelete); err != nil {
			return nil, err
		}
	}
	return components, hookInventory.SetDeleted(toDelete)
}

func (engine *Engine) updateHookInventory(ctx context.Context, inv inventory.Info, namespace, name string, allHooks map[object.ObjMetadata]*unstructured.Unstructured, hookInventory *hook.Inventory, deletePolicyHooks ...object.ObjMetadataSet) error {
	var manifests []*unstructured.Unstructured

	invObjSet, err := hookInventory.Load()
	if err != nil {
		return err
	}

	if len(invObjSet) == 0 {
		return nil
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
			return fmt.Errorf("failed to update %v", k)
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
