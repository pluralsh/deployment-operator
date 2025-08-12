package db

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

var _ inventory.Client = &CacheInventoryClient{}

type CacheInventoryClient struct {
}

func (c *CacheInventoryClient) GetClusterObjs(inv inventory.Info) (object.ObjMetadataSet, error) {
	return GetInventory(inv.ID()).Load()
}

// Merge applies the union of the passed objects with the currently
// stored objects in the inventory object. Returns the set of
// objects which are not in the passed objects (objects to be pruned).
// Otherwise, returns an error if one happened.
func (c *CacheInventoryClient) Merge(inv inventory.Info, objs object.ObjMetadataSet, _ common.DryRunStrategy) (object.ObjMetadataSet, error) {
	pruneIds := object.ObjMetadataSet{}
	// Update existing cluster inventory with merged union of objects
	clusterObjs, err := c.GetClusterObjs(inv)
	if err != nil {
		return pruneIds, err
	}
	pruneIds = clusterObjs.Diff(objs)
	status := getObjStatus(object.ObjMetadataSet{}, objs)

	if err := GetInventory(inv.ID()).Store(objs, status); err != nil {
		return pruneIds, err
	}

	return pruneIds, nil
}

func (c *CacheInventoryClient) Replace(inv inventory.Info, objs object.ObjMetadataSet, status []actuation.ObjectStatus, dryRun common.DryRunStrategy) error {
	return GetInventory(inv.ID()).Store(objs, status)
}

func (c *CacheInventoryClient) DeleteInventoryObj(inv inventory.Info, _ common.DryRunStrategy) error {
	DeleteInventory(inv.ID())
	return nil
}

func (c *CacheInventoryClient) ApplyInventoryNamespace(invNamespace *unstructured.Unstructured, dryRun common.DryRunStrategy) error {
	return nil
}

func (c *CacheInventoryClient) GetClusterInventoryInfo(inv inventory.Info) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (c *CacheInventoryClient) GetClusterInventoryObjs(inv inventory.Info) (object.UnstructuredSet, error) {
	return nil, nil
}

func (c *CacheInventoryClient) ListClusterInventoryObjs(ctx context.Context) (map[string]object.ObjMetadataSet, error) {
	return nil, nil
}

// getObjStatus returns the list of object status
// at the beginning of an apply process.
func getObjStatus(pruneIds, unionIds []object.ObjMetadata) []actuation.ObjectStatus {
	status := []actuation.ObjectStatus{}
	for _, obj := range unionIds {
		status = append(status,
			actuation.ObjectStatus{
				ObjectReference: ObjectReferenceFromObjMetadata(obj),
				Strategy:        actuation.ActuationStrategyApply,
				Actuation:       actuation.ActuationPending,
				Reconcile:       actuation.ReconcilePending,
			})
	}
	for _, obj := range pruneIds {
		status = append(status,
			actuation.ObjectStatus{
				ObjectReference: ObjectReferenceFromObjMetadata(obj),
				Strategy:        actuation.ActuationStrategyDelete,
				Actuation:       actuation.ActuationPending,
				Reconcile:       actuation.ReconcilePending,
			})
	}
	return status
}

// ObjectReferenceFromObjMetadata converts an ObjMetadata to a ObjectReference
func ObjectReferenceFromObjMetadata(id object.ObjMetadata) actuation.ObjectReference {
	return actuation.ObjectReference{
		Group:     id.GroupKind.Group,
		Kind:      id.GroupKind.Kind,
		Name:      id.Name,
		Namespace: id.Namespace,
	}
}
