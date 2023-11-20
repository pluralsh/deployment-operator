package sync

import (
	"context"
	"fmt"

	"github.com/pluralsh/deployment-operator/pkg/hook"
	"github.com/pluralsh/polly/containers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

func (engine *Engine) managePreInstallHooks(ctx context.Context, namespace, name, id string, hookObjects []*unstructured.Unstructured, objects []*unstructured.Unstructured) error {
	installed, err := engine.isInstalled(id, objects)
	if err != nil {
		return err
	}
	if installed {
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
	hookSet := containers.Set[*unstructured.Unstructured]{}
	deleteBefore := containers.Set[*unstructured.Unstructured]{}
	deleteFailed := containers.Set[*unstructured.Unstructured]{}
	deleteSucceeded := containers.Set[*unstructured.Unstructured]{}
	for _, h := range hooks {
		if h.DeletePolicies.Has(hook.BeforeHookCreation) {
			deleteBefore.Add(h.Object)
		} else if h.DeletePolicies.Has(hook.HookFailed) {
			deleteFailed.Add(h.Object)
		} else if h.DeletePolicies.Has(hook.HookSucceeded) {
			deleteSucceeded.Add(h.Object)
		}
		hookSet.Add(h.Object)
	}
	dynamicClient, err := engine.utilFactory.DynamicClient()
	if err != nil {
		return err
	}
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
	if invMap != nil {
		// delete previous resources
		for _, r := range deleteBefore.List() {
			gvk := r.GroupVersionKind()
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: gvk.Kind,
			}
			if err := dynamicClient.Resource(gvr).Namespace(r.GetNamespace()).Delete(ctx, r.GetName(), metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	manifests = hookSet.List()
	if len(manifests) == 0 {
		return nil
	}

	ch := engine.applier.Run(ctx, inv, manifests, GetDefaultApplierOptions())
	statsCollector, statusCollector, err := GetStatusCollector(ch, false)
	if err != nil {
		return err
	}

	if err := FormatSummary(namespace, name, *statsCollector); err != nil {
		return err
	}

	for _, v := range statusCollector.latestStatus {
		if v.Resource == nil {
			continue
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
		return false, err
	}
	if apierrors.IsNotFound(err) {
		return false, nil
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
