package cache

import (
	"context"

	"sigs.k8s.io/cli-utils/pkg/inventory"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/object"
)

var _ inventory.Client = &CacheInventoryClient{}

type CacheInventoryClient struct {
}

func (c *CacheInventoryClient) GetClusterObjs(inv inventory.Info) (object.ObjMetadataSet, error) {
	return GetInventory(inv.ID()).Load()
}

func (c *CacheInventoryClient) Merge(inv inventory.Info, objs object.ObjMetadataSet, _ common.DryRunStrategy) (object.ObjMetadataSet, error) {
	return GetInventory(inv.ID()).Merge(objs)
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
