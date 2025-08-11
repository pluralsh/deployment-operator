package cache

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

var inventoryMap cmap.ConcurrentMap[string, *Inventory]

func init() {
	inventoryMap = cmap.New[*Inventory]()
}

func RegisterInventory(name, namespace string) *Inventory {
	if _, ok := inventoryMap.Get(name); !ok {
		inventoryMap.Set(name, &Inventory{
			name:      name,
			namespace: namespace,
			objMetas:  make(object.ObjMetadataSet, 0),
			objStatus: make([]actuation.ObjectStatus, 0),
		})
	}
	inv, _ := inventoryMap.Get(name)
	return inv
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

func (c *Inventory) Merge(objs object.ObjMetadataSet) (object.ObjMetadataSet, error) {
	c.objMetas = c.objMetas.Union(objs)
	return c.objMetas, nil
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
