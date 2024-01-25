package sync

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

func (engine *Engine) SplitObjects(id string, objs []*unstructured.Unstructured) (*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	invs := make([]*unstructured.Unstructured, 0, 1)
	resources := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if inventory.IsInventoryObject(obj) {
			invs = append(invs, obj)
		} else {
			resources = append(resources, obj)
		}
	}
	switch len(invs) {
	case 0:
		invObj, err := engine.defaultInventoryObjTemplate(id)
		if err != nil {
			return nil, nil, err
		}
		return invObj, resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return nil, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func (engine *Engine) defaultInventoryObjTemplate(id string) (*unstructured.Unstructured, error) {
	name := "inventory-" + id
	namespace := "plrl-deploy-operator"

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
	}, nil
}
