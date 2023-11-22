package hook

import (
	"context"
	"reflect"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type Inventory struct {
	name      string
	namespace string
	client    *kubernetes.Clientset
	ctx       context.Context
}

func NewInventory(ctx context.Context, namespace, name string, client *kubernetes.Clientset) *Inventory {
	return &Inventory{
		name:      name,
		namespace: namespace,
		client:    client,
		ctx:       ctx,
	}
}

func (i *Inventory) GetDeleted() (object.ObjMetadataSet, error) {
	invMap, err := i.client.CoreV1().ConfigMaps(i.namespace).Get(i.ctx, i.name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if invMap == nil {
		return object.ObjMetadataSet{}, nil
	}
	if invMap.Annotations == nil {
		return object.ObjMetadataSet{}, nil
	}
	result := object.ObjMetadataSet{}
	for k, _ := range invMap.Annotations {
		obj, err := object.ParseObjMetadata(k)
		if err == nil {
			result = append(result, obj)
		}
	}
	return result, nil
}

func (i *Inventory) SetDeleted(deleted object.ObjMetadataSet) error {
	if len(deleted) == 0 {
		return nil
	}
	invMap, err := i.client.CoreV1().ConfigMaps(i.namespace).Get(i.ctx, i.name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if invMap.Annotations == nil {
		invMap.Annotations = map[string]string{}
	}
	newAnnotations := map[string]string{}
	for _, d := range deleted {
		newAnnotations[d.String()] = ""
	}
	if reflect.DeepEqual(invMap.Annotations, newAnnotations) {
		return nil
	}

	for _, d := range deleted {
		invMap.Annotations[d.String()] = ""
	}
	_, err = i.client.CoreV1().ConfigMaps(i.namespace).Update(i.ctx, invMap, metav1.UpdateOptions{})

	return err
}

func (i *Inventory) Load() (object.ObjMetadataSet, error) {
	invMap, err := i.client.CoreV1().ConfigMaps(i.namespace).Get(i.ctx, i.name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	if invMap == nil {
		return object.ObjMetadataSet{}, nil
	}
	if invMap.Data == nil {
		return object.ObjMetadataSet{}, nil
	}

	invElem, err := ConvertInventoryMap(invMap)
	if err != nil {
		return nil, err
	}
	invObj := inventory.WrapInventoryObj(invElem)
	return invObj.Load()
}

func ConvertInventoryMap(inventoryMap *v1.ConfigMap) (*unstructured.Unstructured, error) {
	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(inventoryMap)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: res,
	}, nil
}
