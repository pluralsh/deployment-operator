package filters

import (
	"context"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/apply/filter"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/samber/lo"
)

type CrdFilter struct {
	Client    dynamic.Interface
	Mapper    meta.RESTMapper
	Inv       inventory.Info
	InvPolicy inventory.Policy
}

// Name returns a filter identifier for logging.
func (crdf CrdFilter) Name() string {
	return "CrdFilter"
}

func (crdf CrdFilter) Filter(obj *unstructured.Unstructured) error {
	// optimization to avoid unnecessary API calls
	if obj.GetKind() != "CustomResourceDefinition" {
		return nil
	}

	// Object must be retrieved from the cluster to get the inventory id.
	clusterObj, err := crdf.getObject(object.UnstructuredToObjMetadata(obj))
	if err != nil {
		if apierrors.IsNotFound(err) {
			// This simply means the object hasn't been created yet.
			return nil
		}
		return filter.NewFatalError(fmt.Errorf("failed to get current object from cluster: %w", err))
	}

	newObj, err := crdf.dryRunApply(obj)
	if err != nil {
		fmt.Printf("Failed to dry run apply crd %s", err)
		return nil
	}
	newObj.SetManagedFields([]metav1.ManagedFieldsEntry{})
	clusterObj.SetManagedFields([]metav1.ManagedFieldsEntry{})

	live, err := clusterObj.MarshalJSON()
	if err != nil {
		return err
	}

	found, err := newObj.MarshalJSON()
	if err != nil {
		return err
	}

	patch, _ := jsonpatch.CreateMergePatch(live, found)
	if string(patch) == "{}" {
		return fmt.Errorf("no changes to be made for CRD %s, skipping", obj.GetName())
	}

	return nil
}

// getObject retrieves the passed object from the cluster, or an error if one occurred.
func (crdf CrdFilter) getObject(id object.ObjMetadata) (*unstructured.Unstructured, error) {
	mapping, err := crdf.Mapper.RESTMapping(id.GroupKind)
	if err != nil {
		return nil, err
	}
	namespacedClient, err := crdf.Client.Resource(mapping.Resource).Namespace(id.Namespace), nil
	if err != nil {
		return nil, err
	}
	return namespacedClient.Get(context.TODO(), id.Name, metav1.GetOptions{})
}

func (crdf CrdFilter) dryRunApply(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	id := object.UnstructuredToObjMetadata(obj)
	mapping, err := crdf.Mapper.RESTMapping(id.GroupKind)
	if err != nil {
		return nil, err
	}
	namespacedClient, err := crdf.Client.Resource(mapping.Resource).Namespace(id.Namespace), nil
	if err != nil {
		return nil, err
	}

	data, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return namespacedClient.Patch(context.TODO(), id.Name, types.ApplyPatchType, data, metav1.PatchOptions{
		DryRun:       []string{metav1.DryRunAll},
		Force:        lo.ToPtr(true),
		FieldManager: "deploy-operator",
	})
}
