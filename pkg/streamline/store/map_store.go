package store

import (
	"fmt"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"sync"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type MapStore struct {
	mu      sync.RWMutex
	objects map[string]Entry
}

func NewMapStore() Store {
	return &MapStore{
		objects: make(map[string]Entry),
	}
}

func (w *MapStore) Delete(uid types.UID) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.objects, string(uid))
	return nil
}

func (w *MapStore) List() ([]Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return lo.Values(w.objects), nil
}

func (w *MapStore) Get(id string) (*Entry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	obj, ok := w.objects[id]
	if !ok {
		return &obj, nil
	}
	return nil, fmt.Errorf("object with id %s doesn't exists", id)
}

func (w *MapStore) Save(obj unstructured.Unstructured) error {
	uid := string(obj.GetUID())
	if uid == "" {
		return fmt.Errorf("entry UID can't be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.objects[uid] = Entry{
		UID:       uid,
		ParentUID: "", // TODO
		Group:     obj.GroupVersionKind().Group,
		Version:   obj.GroupVersionKind().Version,
		Kind:      obj.GroupVersionKind().Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Status:    common.ToStatus(&obj).String(),
		ServiceID: common.ServiceID(&obj),
	}
	return nil
}
