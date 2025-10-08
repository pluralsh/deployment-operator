package streamline

import (
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
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

func ResetGlobalStore() {
	mu.Lock()
	defer mu.Unlock()
	globalStoreInstance = nil
}

func GetGlobalStore() *GlobalStore {
	mu.Lock()
	defer mu.Unlock()

	return globalStoreInstance
}

type GlobalStore struct {
	store store.Store
}

func (in *GlobalStore) GetComponent(obj unstructured.Unstructured) (result *smcommon.Component, err error) {
	return in.store.GetComponent(obj)
}

func (in *GlobalStore) UpdateComponentSHA(obj unstructured.Unstructured, shaType store.SHAType) error {
	return in.store.UpdateComponentSHA(obj, shaType)
}

func (in *GlobalStore) CommitTransientSHA(obj unstructured.Unstructured) error {
	return in.store.CommitTransientSHA(obj)
}

func (in *GlobalStore) ExpireSHA(obj unstructured.Unstructured) error {
	return in.store.ExpireSHA(obj)
}

func (in *GlobalStore) Expire(serviceID string) error {
	return in.store.Expire(serviceID)
}

func (in *GlobalStore) DeleteComponent(key smcommon.StoreKey) error {
	return in.store.DeleteComponent(key)
}

func (in *GlobalStore) GetResourceHealth(resources []unstructured.Unstructured) (pending, failed bool, err error) {
	return in.store.GetResourceHealth(resources)
}

func (in *GlobalStore) HasSomeResources(resources []unstructured.Unstructured) (bool, error) {
	return in.store.HasSomeResources(resources)
}

func (in *GlobalStore) SaveHookComponentWithManifestSHA(appliedResource unstructured.Unstructured, manifestSHA string) error {
	return in.store.SaveHookComponentWithManifestSHA(appliedResource, manifestSHA)
}
