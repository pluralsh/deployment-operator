package streamline

import (
	"sync"

	"github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

var (
	globalStoreInstance *GlobalStore
	mu                  sync.Mutex
)

func InitGlobalStore(s store.Store) {
	mu.Lock()
	defer mu.Unlock()

	if globalStoreInstance != nil {
		return
	}

	globalStoreInstance = &GlobalStore{store: s}
}

func GetGlobalStore() *GlobalStore {
	mu.Lock()
	defer mu.Unlock()

	return globalStoreInstance
}

type GlobalStore struct {
	store store.Store
}

func (in *GlobalStore) SaveComponent(obj unstructured.Unstructured) (err error) {
	return in.store.SaveComponent(obj)
}

func (in *GlobalStore) GetComponent(obj unstructured.Unstructured) (result *store.Entry, err error) {
	return in.store.GetComponent(obj)
}

func (in *GlobalStore) GetComponentChildren(uid string) (result []client.ComponentChildAttributes, err error) {
	return in.store.GetComponentChildren(uid)
}

func (in *GlobalStore) UpdateComponentSHA(obj unstructured.Unstructured, shaType store.SHAType) error {
	return globalStoreInstance.UpdateComponentSHA(obj, shaType)
}

func (in *GlobalStore) CommitTransientSHA(obj unstructured.Unstructured) error {
	return globalStoreInstance.CommitTransientSHA(obj)
}

func (in *GlobalStore) ExpireSHA(obj unstructured.Unstructured) error {
	return globalStoreInstance.ExpireSHA(obj)
}
