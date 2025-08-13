package db

import (
	"context"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

var inventoryMap cmap.ConcurrentMap[string, *Inventory]

func init() {
	inventoryMap = cmap.New[*Inventory]()
}

func RegisterInventory(ctx context.Context, name, namespace string) (*Inventory, error) {
	if _, ok := inventoryMap.Get(name); !ok {
		var obj object.ObjMetadataSet
		var err error
		var lastSize int
		var iteration int

		// Load the data from DB cache. Wait mechanism for the first run.
		// If the result size hasn’t changed between two consecutive checks, it returns the result.
		if err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(_ context.Context) (bool, error) {
			obj, err = GetComponentCache().GetComponentsByServiceID(name)
			if err != nil {
				return false, err
			}

			currentSize := len(obj)

			// new service, nothing on server side
			if iteration >= 3 && currentSize == 0 {
				return true, nil
			}

			// Stable size, return result
			if lastSize == currentSize && lastSize > 0 {
				return true, nil
			}
			lastSize = currentSize
			iteration++

			return false, nil
		}); err != nil {
			return nil, err
		}

		inv := &Inventory{
			name:      name,
			namespace: namespace,
			objMetas:  obj,
			objStatus: make([]actuation.ObjectStatus, 0),
		}
		inventoryMap.Set(name, inv)
	}
	inv, _ := inventoryMap.Get(name)
	return inv, nil
}

func GetInventory(name string) *Inventory {
	inv, _ := inventoryMap.Get(name)
	return inv
}

func DeleteInventory(name string) {
	inventoryMap.Remove(name)
}

type Inventory struct {
	namespace string
	name      string
	objMetas  object.ObjMetadataSet
	objStatus []actuation.ObjectStatus
}

var _ inventory.Info = &Inventory{}
var _ inventory.Storage = &Inventory{}

func (c *Inventory) Load() (object.ObjMetadataSet, error) {
	return c.objMetas, nil
}

func (c *Inventory) Store(objs object.ObjMetadataSet, status []actuation.ObjectStatus) error {
	c.objMetas = objs
	c.objStatus = status
	return nil
}

func (c *Inventory) GetObject() (*unstructured.Unstructured, error) {
	return nil, nil
}

func (c *Inventory) Apply(d dynamic.Interface, mapper meta.RESTMapper, policy inventory.StatusPolicy) error {
	return nil
}

func (c *Inventory) ApplyWithPrune(d dynamic.Interface, mapper meta.RESTMapper, policy inventory.StatusPolicy, set object.ObjMetadataSet) error {
	return nil
}

func (c *Inventory) Namespace() string {
	return c.namespace
}

func (c *Inventory) Name() string {
	return c.name
}

func (c *Inventory) ID() string {
	return c.name
}

func (c *Inventory) Strategy() inventory.Strategy {
	return "memory"
}
