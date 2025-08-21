package streamline

import (
	"sync"

	"github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

var (
	globalStoreInstance store.Store
	mu                  sync.Mutex
)

type globalStore struct {
	store store.Store
}

func (in globalStore) GetComponentChildren(uid string) (result []client.ComponentChildAttributes, err error) {
	return in.store.GetComponentChildren(uid)
}

func (in globalStore) SaveComponent(obj unstructured.Unstructured) (err error) {
	return in.store.SaveComponent(obj)
}

func InitGlobalStore(s store.Store) {
	mu.Lock()
	defer mu.Unlock()

	if globalStoreInstance != nil {
		return
	}

	globalStoreInstance = s
}

func GlobalStore() store.Store {
	mu.Lock()
	defer mu.Unlock()

	return globalStoreInstance
}

func (in globalStore) UpdateComponentSHA(obj unstructured.Unstructured, shaType store.SHAType) error {
	return globalStoreInstance.UpdateComponentSHA(obj, shaType)
}

func (in globalStore) ExpireSHA(obj unstructured.Unstructured) error {
	return globalStoreInstance.ExpireSHA(obj)
}
