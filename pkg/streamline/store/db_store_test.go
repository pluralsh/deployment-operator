package store_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pluralsh/deployment-operator/pkg/streamline/api"
	"github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

const (
	testUID       = "test-uid"
	testGroup     = "test-group"
	testNamespace = "test-namespace"
	testKind      = "Test"
	testVersion   = "v1"
	testName      = "test-component"
	testChildUID  = "child-uid"
)

func createComponent(uid string, option ...CreateComponentOption) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: testGroup, Version: testVersion, Kind: testKind})
	u.SetNamespace(testNamespace)
	u.SetName(testName)
	u.SetUID(types.UID(uid))

	for _, opt := range option {
		opt(&u)
	}

	return u
}

type CreateComponentOption func(u *unstructured.Unstructured)

func WithParent(uid string) CreateComponentOption {
	return func(u *unstructured.Unstructured) {
		u.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: testGroup + "/" + testVersion,
			Kind:       testKind,
			Name:       testName,
			UID:        types.UID(uid),
		}})
	}
}

// WithService should be called as a last option to ensure that TrackingIdentifierKey will be valid.
func WithService(id string) CreateComponentOption {
	return func(u *unstructured.Unstructured) {
		u.SetAnnotations(map[string]string{
			common.OwningInventoryKey:    id,
			common.TrackingIdentifierKey: common.NewKeyFromUnstructured(lo.FromPtr(u)).String(),
		})
	}
}

func WithGVK(group, version, kind string) CreateComponentOption {
	return func(u *unstructured.Unstructured) {
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	}
}

func WithNamespace(namespace string) CreateComponentOption {
	return func(u *unstructured.Unstructured) {
		u.SetNamespace(namespace)
	}
}

func WithName(name string) CreateComponentOption {
	return func(u *unstructured.Unstructured) {
		u.SetName(name)
	}
}

type CreateStoreKeyOption func(entry *common.Component)

func WithStoreKeyName(name string) CreateStoreKeyOption {
	return func(entry *common.Component) {
		entry.Name = name
	}
}

func WithStoreKeyNamespace(namespace string) CreateStoreKeyOption {
	return func(entry *common.Component) {
		entry.Namespace = namespace
	}
}

func WithStoreKeyGroup(group string) CreateStoreKeyOption {
	return func(entry *common.Component) {
		entry.Group = group
	}
}

func WithStoreKeyVersion(version string) CreateStoreKeyOption {
	return func(entry *common.Component) {
		entry.Version = version
	}
}

func WithStoreKeyKind(kind string) CreateStoreKeyOption {
	return func(entry *common.Component) {
		entry.Kind = kind
	}
}

func createStoreKey(option ...CreateStoreKeyOption) common.StoreKey {
	result := common.Component{
		Group:     testGroup,
		Version:   testVersion,
		Kind:      testKind,
		Namespace: testNamespace,
		Name:      testName,
	}

	for _, opt := range option {
		opt(&result)
	}

	return result.StoreKey()
}

func TestComponentCache_Init(t *testing.T) {
	t.Run("cache should initialize", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			err := storeInstance.Shutdown()
			if err != nil {
				t.Errorf("failed to shutdown store: %v", err)
			}
		}(storeInstance)
	})
}

func TestComponentCache_SetComponent(t *testing.T) {
	t.Run("cache should save and return simple parent and child structure", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			err := storeInstance.Shutdown()
			if err != nil {
				t.Errorf("failed to shutdown store: %v", err)
			}
		}(storeInstance)

		uid := testUID

		component := createComponent(uid, WithName("parent-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		childComponent := createComponent(testChildUID, WithParent(uid), WithName("child-component"))
		err = storeInstance.SaveComponent(childComponent)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, testChildUID, children[0].UID)
		assert.Equal(t, uid, *children[0].ParentUID)
	})
}

func TestComponentCache_ComponentChildren(t *testing.T) {
	t.Run("cache should save and return multi-level structure", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Root
		rootUID := "root-uid"
		component := createComponent(rootUID, WithName("root-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, WithParent(rootUID), WithName("level-1-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, WithParent(uid1), WithName("level-2-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, WithParent(uid2), WithName("level-3-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, WithParent(uid3), WithName("level-4-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, WithParent(uid4), WithName("level-5-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 4)
	})

	t.Run("cache should save and return multi-level structure with multiple children", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Root
		rootUID := testUID
		component := createComponent(rootUID, WithName("multi-root-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, WithParent(rootUID), WithName("multi-level-1-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, WithParent(uid1), WithName("multi-level-2-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, WithParent(uid2), WithName("multi-level-3-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, WithParent(uid3), WithName("multi-level-4-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		uid44 := "uid-44"
		component = createComponent(uid44, WithParent(uid3), WithName("multi-level-4b-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, WithParent(uid4), WithName("multi-level-5-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 5)
	})
}

func TestComponentCache_DeleteComponent(t *testing.T) {
	t.Run("cache should support basic cascade deletion", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		uid := testUID
		component := createComponent(uid, WithName("delete-parent-component"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, WithParent(uid), WithName("delete-child-component"))
		err = storeInstance.SaveComponent(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", WithParent(childUid), WithName("delete-grandchild-component"))
		err = storeInstance.SaveComponent(grandchildComponent)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 2)

		err = storeInstance.DeleteComponent(createStoreKey(WithStoreKeyName("delete-child-component")))
		require.NoError(t, err)

		children, err = storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 0)
	})

	t.Run("cache should support multi-level cascade deletion", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		uid := testUID
		component := createComponent(uid, WithName("multi-delete-parent"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, WithParent(uid), WithName("multi-delete-child"))
		err = storeInstance.SaveComponent(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", WithParent(childUid), WithName("multi-delete-grandchild"))
		err = storeInstance.SaveComponent(grandchildComponent)
		require.NoError(t, err)

		child2Uid := "child2-uid"
		child2Component := createComponent(child2Uid, WithParent(uid), WithName("multi-delete-child2"))
		err = storeInstance.SaveComponent(child2Component)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 3)

		err = storeInstance.DeleteComponent(createStoreKey(WithStoreKeyName("multi-delete-child")))
		require.NoError(t, err)

		children, err = storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
	})
}

func TestComponentCache_GroupHandling(t *testing.T) {
	t.Run("cache should correctly store and return group", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		group := testGroup

		uid := testUID
		component := createComponent(uid, WithName("group-test-parent"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		child := createComponent("child-uid", WithParent(uid), WithGVK(group, testVersion, testKind), WithName("group-test-child"))
		err = storeInstance.SaveComponent(child)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Equal(t, group, *children[0].Group)

		// Test empty group
		child = createComponent("child2-uid", WithParent(uid), WithGVK("", testVersion, testKind), WithName("group-test-child"))
		err = storeInstance.SaveComponent(child)
		require.NoError(t, err)

		tested, err := storeInstance.GetComponentByUID("child2-uid")
		require.NoError(t, err)
		require.Nil(t, tested.Group)

		// Test nil group
		// Test nil group
		child = createComponent("child3-uid", WithParent(uid), WithGVK("", testVersion, testKind), WithName("group-test-child"))
		err = storeInstance.SaveComponent(child)
		require.NoError(t, err)

		tested, err = storeInstance.GetComponentByUID("child3-uid")
		require.NoError(t, err)
		require.Nil(t, tested.Group)
	})
}

func TestComponentCache_UniqueConstraint(t *testing.T) {
	t.Run("should allow components with different GVK-namespace-name combinations", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component1 := createComponent("uid-1",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component1)
		require.NoError(t, err)

		// Component with different name - should succeed
		component2 := createComponent("uid-2",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("my-other-app"))
		err = storeInstance.SaveComponent(component2)
		require.NoError(t, err)

		// Component with different namespace - should succeed
		component3 := createComponent("uid-3",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("production"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component3)
		require.NoError(t, err)

		// Component with different kind - should succeed
		component4 := createComponent("uid-4",
			WithGVK("apps", "v1", "StatefulSet"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component4)
		require.NoError(t, err)

		// Component with different version - should succeed
		component5 := createComponent("uid-5",
			WithGVK("apps", "v2", "Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component5)
		require.NoError(t, err)

		// Component with different group - should succeed
		component6 := createComponent("uid-6",
			WithGVK("extensions", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component6)
		require.NoError(t, err)
	})

	t.Run("should allow component updates", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component := createComponent("uid-1",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		component = createComponent("uid-1",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)
	})

	t.Run("should allow components with same GVK-namespace-name but different UID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component1 := createComponent("uid-1",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("duplicate-app"))
		err = storeInstance.SaveComponent(component1)
		require.NoError(t, err)

		component2 := createComponent("uid-2",
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("duplicate-app"))
		err = storeInstance.SaveComponent(component2)
		require.NoError(t, err)
	})

	t.Run("should allow updating existing component with same UID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		uid := "update-test-uid"

		// Create initial component
		component := createComponent(uid,
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("updatable-app"))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		// Update the same component with different state - should succeed
		updatedComponent := createComponent(uid,
			WithGVK("apps", "v1", "Deployment"),
			WithNamespace("default"),
			WithName("updatable-app"))
		err = storeInstance.SaveComponent(updatedComponent)
		require.NoError(t, err)
	})

	t.Run("should handle UID changes for resource with the same identity", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		group := "apps"
		version := "v1"
		kind := "Deployment"
		name := "test"
		namespace := ""

		var u unstructured.Unstructured
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
		u.SetName(name)
		u.SetNamespace(namespace)

		component := createComponent("uid-1", WithGVK(group, version, kind), WithName(name), WithNamespace(namespace))
		err = storeInstance.SaveComponent(component)
		require.NoError(t, err)

		sameComponentWithDifferentUID := createComponent("uid-2", WithGVK(group, version, kind), WithName(name), WithNamespace(namespace))
		err = storeInstance.SaveComponent(sameComponentWithDifferentUID)
		require.NoError(t, err)

		dbc, err := storeInstance.GetComponent(u)
		require.NoError(t, err)
		assert.Equal(t, "uid-2", dbc.UID)
	})
}

func TestComponentCache_ComponentInsights(t *testing.T) {
	t.Run("should handle empty cache without errors", func(t *testing.T) {
		// Initialize a fresh cache for this test
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		// Get component insights from empty cache
		insights, err := storeInstance.GetComponentInsights()
		require.NoError(t, err, "Failed to get component insights from empty cache")
		require.Nil(t, insights, "Expected non-nil insights object from empty cache")
	})
}

func TestComponentCountsCache(t *testing.T) {
	t.Run("cache should return counts of nodes, pods and namespaces", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create 3 namespaces
		err = storeInstance.SaveComponent(createComponent("a", WithGVK("", testVersion, "Namespace"), WithName("a")))
		require.NoError(t, err)
		err = storeInstance.SaveComponent(createComponent("b", WithGVK("", testVersion, "Namespace"), WithName("b")))
		require.NoError(t, err)
		err = storeInstance.SaveComponent(createComponent("c", WithGVK("", testVersion, "Namespace"), WithName("c")))
		require.NoError(t, err)

		// Create 3 nodes
		err = storeInstance.SaveComponent(createComponent("node-1", WithGVK("", testVersion, "Node"), WithName("node-1")))
		require.NoError(t, err)
		err = storeInstance.SaveComponent(createComponent("node-2", WithGVK("", testVersion, "Node"), WithName("node-2")))
		require.NoError(t, err)
		err = storeInstance.SaveComponent(createComponent("node-3", WithGVK("", testVersion, "Node"), WithName("node-3")))
		require.NoError(t, err)

		nodes, namespaces, err := storeInstance.GetComponentCounts()
		require.NoError(t, err, "Failed to get component counts")

		assert.Equal(t, nodes, int64(3))
		assert.Equal(t, namespaces, int64(3))
	})

	t.Run("should return correct counts of nodes and namespaces", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create 2 nodes
		err = storeInstance.SaveComponent(createComponent("node-1", WithGVK("", testVersion, "Node"), WithName("worker-1")))
		require.NoError(t, err)

		err = storeInstance.SaveComponent(createComponent("node-2", WithGVK("", testVersion, "Node"), WithName("worker-2")))
		require.NoError(t, err)

		// Create 3 namespaces
		err = storeInstance.SaveComponent(createComponent("ns-1", WithGVK("", testVersion, "Namespace"), WithName("default")))
		require.NoError(t, err)

		err = storeInstance.SaveComponent(createComponent("ns-2", WithGVK("", testVersion, "Namespace"), WithName("kube-system")))
		require.NoError(t, err)

		err = storeInstance.SaveComponent(createComponent("ns-3", WithGVK("", testVersion, "Namespace"), WithName("production")))
		require.NoError(t, err)

		// Create some other resources that should not be counted
		err = storeInstance.SaveComponent(createComponent("pod-1", WithGVK("", testVersion, "Pod"), WithName("test-pod")))
		require.NoError(t, err)

		nodeCount, namespaceCount, err := storeInstance.GetComponentCounts()
		require.NoError(t, err, "failed to get component counts")

		assert.Equal(t, int64(2), nodeCount)
		assert.Equal(t, int64(3), namespaceCount)
	})
}

func TestComponentCache_ComponentChildrenLimit(t *testing.T) {
	t.Run("should limit component children to 100 items", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create parent component
		parentUID := "parent-with-many-children"
		err = storeInstance.SaveComponent(createComponent(parentUID, WithName("parent-with-many-children")))
		require.NoError(t, err)
		require.NoError(t, err)

		// Create 150 child components to test the 100 limit
		totalChildren := 150
		for i := 0; i < totalChildren; i++ {
			err := storeInstance.SaveComponent(createComponent(
				fmt.Sprintf("child-%d", i),
				WithName(fmt.Sprintf("child-component-%d", i)),
				WithParent(parentUID)))
			require.NoError(t, err)
			require.NoError(t, err)
		}

		// Retrieve children and verify limit is applied
		children, err := storeInstance.GetComponentChildren(parentUID)
		require.NoError(t, err)

		// Should return exactly 100 children, not more
		assert.Equal(t, 100, len(children), "Expected exactly 100 children due to LIMIT clause")

		// Verify all returned children have the correct parent
		for _, child := range children {
			assert.Equal(t, parentUID, *child.ParentUID, "All returned children should have correct parent UID")
		}
	})

	t.Run("should return all children when under 100 limit", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create parent component
		parentUID := "parent-with-few-children"
		err = storeInstance.SaveComponent(createComponent(parentUID, WithName("parent-with-few-children")))
		require.NoError(t, err)

		// Create 50 child components (under the limit)
		totalChildren := 50
		for i := 0; i < totalChildren; i++ {
			err := storeInstance.SaveComponent(createComponent(
				fmt.Sprintf("few-child-%d", i),
				WithParent(parentUID),
				WithName(fmt.Sprintf("few-child-component-%d", i))))
			require.NoError(t, err)
		}

		// Retrieve children and verify all are returned
		children, err := storeInstance.GetComponentChildren(parentUID)
		require.NoError(t, err)

		// Should return all 50 children since we're under the limit
		assert.Equal(t, totalChildren, len(children), "Expected all children when under 100 limit")

		// Verify all returned children have the correct parent
		for _, child := range children {
			assert.Equal(t, parentUID, *child.ParentUID, "All returned children should have correct parent UID")
		}
	})

	t.Run("should apply limit to multi-level hierarchies", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create root component
		rootUID := "root-with-deep-hierarchy"
		err = storeInstance.SaveComponent(createComponent(rootUID, WithName("root-with-deep-hierarchy")))
		require.NoError(t, err)

		// Create a multi-level hierarchy that exceeds 100 total descendants
		// Level 1: 30 components
		level1UIDs := make([]string, 30)
		for i := 0; i < 30; i++ {
			uid := fmt.Sprintf("level1-%d", i)
			level1UIDs[i] = uid
			err := storeInstance.SaveComponent(createComponent(uid, WithParent(rootUID), WithName(fmt.Sprintf("level1-component-%d", i))))
			require.NoError(t, err)
		}

		// Level 2: 40 components (distributed among level 1 components)
		level2Count := 0
		for i := 0; i < 20 && level2Count < 40; i++ {
			parentUID := level1UIDs[i%len(level1UIDs)]
			for j := 0; j < 2 && level2Count < 40; j++ {
				uid := fmt.Sprintf("level2-%d-%d", i, j)
				err := storeInstance.SaveComponent(createComponent(uid, WithParent(parentUID), WithName(fmt.Sprintf("level2-component-%d-%d", i, j))))
				require.NoError(t, err)
				level2Count++
			}
		}

		// Level 3: 50 components (this will push us over 100)
		level3Count := 0
		for i := 0; i < 25 && level3Count < 50; i++ {
			// Find a level 2 component to be parent
			level2UID := fmt.Sprintf("level2-%d-0", i%20)
			for j := 0; j < 2 && level3Count < 50; j++ {
				uid := fmt.Sprintf("level3-%d-%d", i, j)
				err := storeInstance.SaveComponent(createComponent(uid, WithParent(level2UID), WithName(fmt.Sprintf("level3-component-%d-%d", i, j))))
				require.NoError(t, err)
				level3Count++
			}
		}

		// Total descendants should be 30 + 40 + 50 = 120, but limit should cap at 100
		children, err := storeInstance.GetComponentChildren(rootUID)
		require.NoError(t, err)

		assert.Equal(t, 100, len(children), "Expected exactly 100 descendants due to LIMIT clause in multi-level hierarchy")
	})
}

func TestUpdateSHA(t *testing.T) {
	t.Run("cache should update SHA", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test", "test", "test", "test", "v1", "Test")
		require.NoError(t, storeInstance.SaveComponent(obj))

		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ApplySHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ServerSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)
		assert.NotEmpty(t, entry.ServerSHA)
		assert.NotEmpty(t, entry.ManifestSHA)
		assert.NotEmpty(t, entry.TransientManifestSHA)
	})
}

func TestExpireSHAOlderThan(t *testing.T) {
	t.Run("should expire SHA", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test", "test", "test", "test", "v1", "Test")
		require.NoError(t, storeInstance.SaveComponent(obj))

		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ApplySHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ServerSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)
		assert.NotEmpty(t, entry.ServerSHA)
		assert.NotEmpty(t, entry.ManifestSHA)
		assert.NotEmpty(t, entry.TransientManifestSHA)

		time.Sleep(2 * time.Second)

		err = storeInstance.ExpireOlderThan(500 * time.Millisecond)
		require.NoError(t, err)

		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.Empty(t, entry.ApplySHA)
		assert.NotEmpty(t, entry.ServerSHA)
		assert.Empty(t, entry.ManifestSHA)
		assert.Empty(t, entry.TransientManifestSHA)
	})

	t.Run("should not expire SHA", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test", "test", "test", "test", "v1", "Test")
		require.NoError(t, storeInstance.SaveComponent(obj))

		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ApplySHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ServerSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)
		assert.NotEmpty(t, entry.ServerSHA)
		assert.NotEmpty(t, entry.ManifestSHA)
		assert.NotEmpty(t, entry.TransientManifestSHA)

		err = storeInstance.ExpireOlderThan(time.Second)
		require.NoError(t, err)

		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)
		assert.NotEmpty(t, entry.ServerSHA)
		assert.NotEmpty(t, entry.ManifestSHA)
		assert.NotEmpty(t, entry.TransientManifestSHA)
	})

	t.Run("trigger should update updated_at column", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test", "test", "test", "test", "v1", "Test")
		require.NoError(t, storeInstance.SaveComponent(obj))

		err = storeInstance.UpdateComponentSHA(obj, store.ServerSHA)
		require.NoError(t, err)

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ServerSHA)

		time.Sleep(time.Second)

		err = storeInstance.ExpireOlderThan(2 * time.Second)
		require.NoError(t, err)

		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ServerSHA)

		err = storeInstance.UpdateComponentSHA(obj, store.ServerSHA)
		require.NoError(t, err)
		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ServerSHA)

		time.Sleep(1500 * time.Millisecond)

		err = storeInstance.ExpireOlderThan(2 * time.Second)
		require.NoError(t, err)
		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ServerSHA)
	})
}

func TestGetComponentsByGVK(t *testing.T) {
	t.Run("should return only components matching provided GVK", func(t *testing.T) {
		gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Insert components with matching GVK and different names/namespaces to avoid unique conflict
		require.NoError(t, storeInstance.SaveComponent(createComponent("gvk-uid-1", WithGVK(gvk.Group, gvk.Version, gvk.Kind), WithNamespace("ns-1"), WithName("alpha"))))
		require.NoError(t, storeInstance.SaveComponent(createComponent("gvk-uid-2", WithGVK(gvk.Group, gvk.Version, gvk.Kind), WithNamespace("ns-2"), WithName("beta"))))
		require.NoError(t, storeInstance.SaveComponent(createComponent("gvk-uid-3", WithGVK(gvk.Group, gvk.Version, gvk.Kind), WithNamespace("ns-3"), WithName("gamma"))))

		// Insert components with different GVK to ensure they are filtered out
		diff1 := createComponent("other-uid-1", WithGVK("apps", "v1", "StatefulSet"), WithNamespace("ns-1"), WithName("alpha"))
		require.NoError(t, storeInstance.SaveComponent(diff1))

		diff2 := createComponent("other-uid-2", WithGVK("extensions", "v1", "Deployment"), WithNamespace("ns-1"), WithName("delta"))
		require.NoError(t, storeInstance.SaveComponent(diff2))

		entries, err := storeInstance.GetComponentsByGVK(gvk)
		require.NoError(t, err)

		assert.Len(t, entries, 3, "expected exactly 3 matching entries")

		names := make([]string, 0, len(entries))
		nss := make([]string, 0, len(entries))
		for _, e := range entries {
			assert.Equal(t, gvk.Group, e.Group, "all entries should have correct group")
			assert.Equal(t, gvk.Version, e.Version, "all entries should have correct version")
			assert.Equal(t, gvk.Kind, e.Kind, "all entries should have correct kind")
			names = append(names, e.Name)
			nss = append(nss, e.Namespace)
		}
		assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, names, "expected names to match, order not guaranteed")
		assert.ElementsMatch(t, []string{"ns-1", "ns-2", "ns-3"}, nss, "expected namespaces to match, order not guaranteed")
	})
}

func TestComponentCache_DeleteComponents(t *testing.T) {
	t.Run("should delete components by group, version, and kind", func(t *testing.T) {
		deploymentsGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
		servicesGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}

		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create components with same GVK but different names/namespaces
		component1 := createComponent("uid-1", WithGVK(deploymentsGVK.Group, deploymentsGVK.Version, deploymentsGVK.Kind), WithName("deployment-1"), WithNamespace("default"))
		err = storeInstance.SaveComponent(component1)
		require.NoError(t, err)

		component2 := createComponent("uid-2",
			WithGVK(deploymentsGVK.Group, deploymentsGVK.Version, deploymentsGVK.Kind),
			WithName("deployment-2"), WithNamespace("default"))
		err = storeInstance.SaveComponent(component2)
		require.NoError(t, err)

		component3 := createComponent("uid-3",
			WithGVK(deploymentsGVK.Group, deploymentsGVK.Version, deploymentsGVK.Kind),
			WithName("deployment-3"), WithNamespace("kube-system"))
		err = storeInstance.SaveComponent(component3)
		require.NoError(t, err)

		// Create component with different GVK
		component4 := createComponent("uid-4",
			WithGVK(servicesGVK.Group, servicesGVK.Version, servicesGVK.Kind),
			WithName("service-1"), WithNamespace("default"))
		err = storeInstance.SaveComponent(component4)
		require.NoError(t, err)

		components, err := storeInstance.GetComponentsByGVK(deploymentsGVK)
		require.NoError(t, err, "failed to verify that deployments exist")
		assert.Len(t, components, 3)

		services, err := storeInstance.GetComponentsByGVK(servicesGVK)
		require.NoError(t, err, "failed to verify that services exist")
		assert.Len(t, services, 1)

		err = storeInstance.DeleteComponents(deploymentsGVK.Group, deploymentsGVK.Version, deploymentsGVK.Kind)
		require.NoError(t, err, "failed to delete deployments")

		components, err = storeInstance.GetComponentsByGVK(deploymentsGVK)
		require.NoError(t, err, "failed to verify that deployments were deleted")
		assert.Len(t, components, 0, "expected deployments to be deleted")

		services, err = storeInstance.GetComponentsByGVK(servicesGVK)
		require.NoError(t, err, "failed to verify that services exist")
		assert.Len(t, services, 1, "expected services to be unaffected")
	})

	t.Run("should handle empty group in delete operation", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create components with empty group (core resources)
		err = storeInstance.SaveComponent(createComponent("job-uid-1", WithGVK("", "v1", "Job"), WithName("job-1"), WithNamespace("default")))
		require.NoError(t, err)

		err = storeInstance.SaveComponent(createComponent("job-uid-2", WithGVK("", "v1", "Job"), WithName("job-2"), WithNamespace("kube-system")))
		require.NoError(t, err)

		service := createComponent("service-uid", WithGVK("", "v1", "Service"), WithName("service-1"), WithNamespace("default"))
		err = storeInstance.SaveComponent(service)
		require.NoError(t, err)

		// Verify jobs exist
		jobs, err := storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Job"})
		require.NoError(t, err)
		assert.Len(t, jobs, 2)

		// Delete all jobs (empty group)
		err = storeInstance.DeleteComponents("", "v1", "Job")
		require.NoError(t, err)

		// Verify jobs are deleted
		jobs, err = storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Job"})
		require.NoError(t, err)
		assert.Len(t, jobs, 0)

		// Verify service still exists
		services, err := storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"})
		require.NoError(t, err)
		assert.Len(t, services, 1)
	})

	t.Run("should handle deletion of non-existent components gracefully", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		err = storeInstance.SaveComponent(createComponent("existing-uid", WithGVK("apps", "v1", "Deployment"), WithName("existing-deployment"), WithNamespace("default")))
		require.NoError(t, err)

		err = storeInstance.DeleteComponents("nonexistent", "v1", "NonExistentKind")
		require.NoError(t, err)

		deployments, err := storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		require.NoError(t, err)
		assert.Len(t, deployments, 1)
	})
}

func TestComponentCache_GetServiceComponents(t *testing.T) {
	t.Run("should return components filtered by service ID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		serviceID := "test-service-123"

		// Create components with the target service ID
		err = storeInstance.SaveComponent(createComponent("service-comp-1", WithGVK("apps", "v1", "Deployment"), WithName("app-deployment"), WithNamespace("default"), WithService(serviceID)))
		require.NoError(t, err)

		err = storeInstance.SaveComponent(createComponent("service-comp-2", WithGVK("", "v1", "Job"), WithName("app-job"), WithNamespace("default"), WithService(serviceID)))
		require.NoError(t, err)

		// Create component with different service ID
		err = storeInstance.SaveComponent(createComponent("other-comp", WithService("other-service"), WithGVK("apps", "v1", "Deployment"), WithName("other-deployment"), WithNamespace("default")))
		require.NoError(t, err)

		// Create component with no service ID
		err = storeInstance.SaveComponent(createComponent("no-service-comp", WithGVK("", "v1", "Service"), WithName("no-service"), WithNamespace("default")))
		require.NoError(t, err)

		components, err := storeInstance.GetServiceComponents(serviceID, true)
		require.NoError(t, err, "failed to get components for service")
		assert.Len(t, components, 2, "expected 2 components with matching service ID")

		foundUIDs := make(map[string]bool)
		for _, comp := range components {
			foundUIDs[comp.UID] = true
			assert.Equal(t, serviceID, comp.ServiceID, "expected component to have matching service ID")
		}

		assert.True(t, foundUIDs["service-comp-1"])
		assert.True(t, foundUIDs["service-comp-2"])
		assert.False(t, foundUIDs["other-comp"])
		assert.False(t, foundUIDs["no-service-comp"])
	})

	t.Run("should return empty slice for non-existent service ID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create some components with different service IDs
		err = storeInstance.SaveComponent(createComponent("test-comp", WithName("test-component")))
		require.NoError(t, err)

		// Try to get components for non-existent service
		components, err := storeInstance.GetServiceComponents("non-existent-service", true)
		require.NoError(t, err)
		assert.Len(t, components, 0)
	})
}

func TestComponentCache_Expire(t *testing.T) {
	t.Run("should expire SHA values for service", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test-expire", "test-component", "default",
			"apps", "v1", "Deployment")

		require.NoError(t, storeInstance.SaveComponent(obj))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ApplySHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		assert.NotEmpty(t, entry.ManifestSHA, "expected manifest SHA to be set")
		assert.NotEmpty(t, entry.ApplySHA, "expected apply SHA to be set")

		require.NoError(t, storeInstance.Expire("test-service"), "failed to expire SHA values for service")
	})
}

func TestComponentCache_ExpireSHA(t *testing.T) {
	t.Run("should expire SHA values for specific component", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test-expire", "test-component", "default",
			"apps", "v1", "Deployment")

		require.NoError(t, storeInstance.SaveComponent(obj))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ApplySHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		assert.NotEmpty(t, entry.ManifestSHA, "expected manifest SHA to be set")
		assert.NotEmpty(t, entry.TransientManifestSHA, "expected transient manifest SHA to be set")
		assert.NotEmpty(t, entry.ApplySHA, "expected apply SHA to be set")

		require.NoError(t, storeInstance.ExpireSHA(obj), "failed to expire SHA values for component")

		expiredEntry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		assert.Empty(t, expiredEntry.ManifestSHA, "expected manifest SHA to be expired")
		assert.Empty(t, expiredEntry.TransientManifestSHA, "expected transient manifest SHA to be expired")
		assert.Empty(t, expiredEntry.ApplySHA, "expected apply SHA to be expired")
		assert.NotEmpty(t, expiredEntry.ServerSHA, "server SHA should remain")
	})
}

func TestComponentCache_CommitTransientSHA(t *testing.T) {
	t.Run("should commit transient SHA to manifest SHA", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := newUnstructured("test-commit-transient", "test-component", "default",
			"apps", "v1", "Deployment")

		require.NoError(t, storeInstance.SaveComponent(obj))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		transientSHA := entry.TransientManifestSHA
		assert.NotEmpty(t, entry.ManifestSHA, "initial manifest SHA should be set")
		assert.NotEmpty(t, transientSHA)

		require.NoError(t, storeInstance.CommitTransientSHA(obj), "failed to commit transient SHA")

		updatedEntry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err, "failed to get updated component entry")
		assert.Equal(t, transientSHA, updatedEntry.ManifestSHA, "expected transient SHA to be committed")
		assert.Empty(t, updatedEntry.TransientManifestSHA, "transient SHA should be empty after commit")
	})
}

func TestComponentCache_SaveComponents(t *testing.T) {
	var objs []unstructured.Unstructured

	storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
	assert.NoError(t, err)
	defer func(storeInstance store.Store) {
		require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
	}(storeInstance)

	for i := 0; i < 10; i++ {
		uid := fmt.Sprintf("uid-%d", i)
		name := fmt.Sprintf("component-%d", i)
		obj := newUnstructured(uid, name, "default", "apps", "v1", "Deployment")
		objs = append(objs, obj)
	}
	require.NoError(t, storeInstance.SaveComponents(objs))

	for _, obj := range objs {
		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err, "failed to get component %s", obj.GetName())
		require.NotNil(t, entry, "expected component %s to exist", obj.GetName())
		require.Equal(t, obj.GetName(), entry.Name, "expected component name to match")
	}
}

func newUnstructured(uid, name, namespace, group, version, kind string) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetUID(types.UID(uid))
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: version, Kind: kind})
	return obj
}

// Helper function to create unstructured.Unstructured objects for testing
func createUnstructuredResource(group, version, kind, namespace, name string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetUID(types.UID(fmt.Sprintf("%s-%s-%s", kind, namespace, name)))
	return u
}

func createHookJob(namespace, name, serviceID string) unstructured.Unstructured {
	group := ""
	version := "v1"
	kind := "Job"

	u := createUnstructuredResource(group, version, kind, namespace, name)

	u.SetAnnotations(map[string]string{
		common.OwningInventoryKey:        serviceID,
		common.TrackingIdentifierKey:     common.NewKeyFromUnstructured(u).String(),
		common.SyncPhaseHookDeletePolicy: common.HookDeletePolicySucceeded,
	})

	u.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Complete",
				"status": "True",
			},
		},
	}

	return u
}

func TestComponentCache_ProcessedHookComponents(t *testing.T) {
	t.Run("should save and retrieve processed hook components by service ID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		serviceID := "svc-basic"
		r := []unstructured.Unstructured{
			createHookJob("default", "migrator", serviceID),
			createHookJob("default", "check", serviceID),
		}

		require.NoError(t, storeInstance.SaveComponents(r))

		result, err := storeInstance.GetHookComponents(serviceID)
		require.NoError(t, err)
		require.Len(t, result, len(r))

		expect := map[string]struct {
			group, version, kind, ns string
		}{
			"migrator": {group: "", version: "v1", kind: "Job", ns: "default"},
			"check":    {group: "", version: "v1", kind: "Job", ns: "default"},
		}

		for _, m := range result {
			assert.Equal(t, serviceID, m.ServiceID)
			want := expect[m.Name]
			assert.Equal(t, want.group, m.Group)
			assert.Equal(t, want.version, m.Version)
			assert.Equal(t, want.kind, m.Kind)
			assert.Equal(t, want.ns, m.Namespace)
		}
	})

	t.Run("should return empty list for non-existent service ID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		result, err := storeInstance.GetHookComponents("non-existent-service")
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("should isolate hooks by service ID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		serviceID1 := "svc-app1"
		serviceID2 := "svc-app2"

		hooks := []unstructured.Unstructured{
			createHookJob("default", "app1-migrator", serviceID1),
			createHookJob("default", "app1-seeder", serviceID1),
			createHookJob("default", "app2-migrator", serviceID2),
		}

		require.NoError(t, storeInstance.SaveComponents(hooks))

		// Check service 1 hooks
		result1, err := storeInstance.GetHookComponents(serviceID1)
		require.NoError(t, err)
		require.Len(t, result1, 2)
		for _, m := range result1 {
			assert.Equal(t, serviceID1, m.ServiceID)
			assert.Contains(t, []string{"app1-migrator", "app1-seeder"}, m.Name)
		}

		// Check service 2 hooks
		result2, err := storeInstance.GetHookComponents(serviceID2)
		require.NoError(t, err)
		require.Len(t, result2, 1)
		assert.Equal(t, serviceID2, result2[0].ServiceID)
		assert.Equal(t, "app2-migrator", result2[0].Name)
	})

	t.Run("should handle different hook resource types", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		serviceID := "svc-various-hooks"

		// Create different types of hook resources
		hookJob := createHookJob("default", "migration-job", serviceID)

		hookPod := createUnstructuredResource("", "v1", "Pod", "default", "migration-pod")
		hookPod.SetAnnotations(map[string]string{
			common.OwningInventoryKey:        serviceID,
			common.TrackingIdentifierKey:     common.NewKeyFromUnstructured(hookPod).String(),
			common.SyncPhaseHookDeletePolicy: common.HookDeletePolicySucceeded,
		})
		hookPod.Object["spec"] = map[string]interface{}{
			"nodeName": "node-1",
		}
		hookPod.Object["status"] = map[string]interface{}{
			"phase": "Succeeded",
		}

		hookConfigMap := createUnstructuredResource("", "v1", "ConfigMap", "default", "migration-config")
		hookConfigMap.SetAnnotations(map[string]string{
			common.OwningInventoryKey:        serviceID,
			common.TrackingIdentifierKey:     common.NewKeyFromUnstructured(hookConfigMap).String(),
			common.SyncPhaseHookDeletePolicy: common.HookDeletePolicySucceeded,
		})

		hooks := []unstructured.Unstructured{hookJob, hookPod, hookConfigMap}

		require.NoError(t, storeInstance.SaveComponents(hooks))

		result, err := storeInstance.GetHookComponents(serviceID)
		require.NoError(t, err)
		require.Len(t, result, 3)

		kinds := make(map[string]bool)
		for _, m := range result {
			kinds[m.Kind] = true
		}
		assert.True(t, kinds["Job"])
		assert.True(t, kinds["Pod"])
		assert.True(t, kinds["ConfigMap"])
	})

	t.Run("should preserve UID and status information", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		serviceID := "svc-uid-check"
		hook := createHookJob("default", "test-hook", serviceID)
		expectedUID := hook.GetUID()

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{hook}))

		result, err := storeInstance.GetHookComponents(serviceID)
		require.NoError(t, err)
		require.Len(t, result, 1)

		assert.Equal(t, string(expectedUID), result[0].UID)
		assert.NotEmpty(t, result[0].Status)
	})

	t.Run("should handle large number of hooks", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		serviceID := "svc-many-hooks"
		hookCount := 50
		hooks := make([]unstructured.Unstructured, hookCount)

		for i := 0; i < hookCount; i++ {
			hooks[i] = createHookJob("default", fmt.Sprintf("hook-%d", i), serviceID)
		}

		require.NoError(t, storeInstance.SaveComponents(hooks))

		result, err := storeInstance.GetHookComponents(serviceID)
		require.NoError(t, err)
		require.Len(t, result, hookCount)

		// Verify all hooks are unique by name
		names := make(map[string]bool)
		for _, m := range result {
			names[m.Name] = true
		}
		assert.Len(t, names, hookCount)
	})
}

func TestComponentCache_SyncAppliedResource(t *testing.T) {
	t.Run("should update apply_sha and server_sha when resource is synced", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create a test resource
		obj := createUnstructuredResource("apps", "v1", "Deployment", "default", "test-deployment")
		obj.Object["spec"] = map[string]interface{}{"replicas": "3"}

		// Save the component first
		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))

		// Get the component before sync
		componentBefore, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentBefore)

		// Sync the applied resource
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		// Get the component after sync
		componentAfter, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentAfter)

		// Verify that apply_sha and server_sha are set and equal
		require.NotEmpty(t, componentAfter.ApplySHA)
		require.NotEmpty(t, componentAfter.ServerSHA)
		require.Equal(t, componentAfter.ApplySHA, componentAfter.ServerSHA)
	})

	t.Run("should keep manifest_sha unchanged when transient_manifest_sha is NULL", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := createUnstructuredResource("apps", "v1", "StatefulSet", "default", "test-statefulset")
		obj.Object["spec"] = map[string]interface{}{"replicas": "2"}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))

		// Get the component before sync to check manifest_sha
		componentBefore, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentBefore)
		originalManifestSHA := componentBefore.ManifestSHA

		// Sync the resource
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		// Get the component after sync
		componentAfter, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentAfter)

		// manifest_sha should remain unchanged when transient_manifest_sha is NULL
		require.Equal(t, originalManifestSHA, componentAfter.ManifestSHA)
		// transient_manifest_sha should be NULL (empty string)
		require.Empty(t, componentAfter.TransientManifestSHA)
	})

	t.Run("should update manifest_sha from transient_manifest_sha when present", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := createUnstructuredResource("apps", "v1", "DaemonSet", "kube-system", "test-daemonset")
		obj.Object["spec"] = map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "test",
				},
			},
		}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))

		// First, set a transient_manifest_sha by calling UpdateComponentSHA
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		// Get the component to verify transient_manifest_sha is set
		componentBeforeSync, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentBeforeSync)
		require.NotEmpty(t, componentBeforeSync.TransientManifestSHA)
		transientSHA := componentBeforeSync.TransientManifestSHA
		originalManifestSHA := componentBeforeSync.ManifestSHA

		// Sync the resource
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		// Get the component after sync
		componentAfter, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentAfter)

		// manifest_sha should now be equal to the previous transient_manifest_sha
		require.Equal(t, transientSHA, componentAfter.ManifestSHA)
		require.NotEqual(t, originalManifestSHA, componentAfter.ManifestSHA)
		// transient_manifest_sha should be cleared (NULL/empty)
		require.Empty(t, componentAfter.TransientManifestSHA)
	})

	t.Run("should clear transient_manifest_sha after sync", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := createUnstructuredResource("", "v1", "ConfigMap", "default", "test-configmap")
		obj.Object["data"] = map[string]interface{}{
			"key": "value",
		}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		// Verify transient_manifest_sha is set before sync
		componentBefore, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotEmpty(t, componentBefore.TransientManifestSHA)

		// Sync the resource
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		// Verify transient_manifest_sha is cleared after sync
		componentAfter, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.Empty(t, componentAfter.TransientManifestSHA)
	})

	t.Run("should not affect other columns during sync", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		obj := createUnstructuredResource("", "v1", "Secret", "default", "test-secret")
		obj.Object["data"] = map[string]interface{}{
			"password": "secret",
		}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		// Get component before sync
		componentBefore, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentBefore)

		// Sync the resource
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		// Get component after sync
		componentAfter, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentAfter)

		// Verify other columns remain unchanged
		require.Equal(t, componentBefore.UID, componentAfter.UID)
		require.Equal(t, componentBefore.Group, componentAfter.Group)
		require.Equal(t, componentBefore.Version, componentAfter.Version)
		require.Equal(t, componentBefore.Kind, componentAfter.Kind)
		require.Equal(t, componentBefore.Namespace, componentAfter.Namespace)
		require.Equal(t, componentBefore.Name, componentAfter.Name)
		require.Equal(t, componentBefore.ParentUID, componentAfter.ParentUID)
		require.Equal(t, componentBefore.ServiceID, componentAfter.ServiceID)
	})

	t.Run("should handle non-existent component gracefully", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create a resource but don't save it
		obj := createUnstructuredResource("apps", "v1", "Deployment", "default", "non-existent")

		// Syncing a non-existent resource should not error (UPDATE affects 0 rows)
		require.NoError(t, storeInstance.SyncAppliedResource(obj))
	})

	t.Run("should handle resources with empty group", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Core resources have empty group
		obj := createUnstructuredResource("", "v1", "Pod", "default", "test-pod")
		obj.Object["spec"] = map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "nginx",
					"image": "nginx:latest",
				},
			},
			"nodeName": "test-node",
		}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		component, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, component)
		require.NotEmpty(t, component.ApplySHA)
		require.Equal(t, component.ApplySHA, component.ServerSHA)
	})

	t.Run("should handle resources with cluster scope (empty namespace)", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// ClusterRole has empty namespace
		obj := createUnstructuredResource("rbac.authorization.k8s.io", "v1", "ClusterRole", "", "test-clusterrole")
		obj.Object["rules"] = []interface{}{
			map[string]interface{}{
				"apiGroups": []interface{}{""},
				"resources": []interface{}{"pods"},
				"verbs":     []interface{}{"get", "list"},
			},
		}

		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))
		require.NoError(t, storeInstance.SyncAppliedResource(obj))

		component, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, component)
		require.NotEmpty(t, component.ApplySHA)
		require.Equal(t, component.ApplySHA, component.ServerSHA)
	})
}

func TestComponentCache_SetServiceChildren(t *testing.T) {
	t.Run("should return 0 if component doesn't exist", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create a test resource
		obj := createUnstructuredResource("apps", "v1", "Deployment", "default", "test-deployment")

		updated, err := storeInstance.SetServiceChildren("abc", "123", []common.StoreKey{
			{
				GVK:       obj.GroupVersionKind(),
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, 0, updated)
	})

	t.Run("should update the component", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		require.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create a test resource
		obj := createUnstructuredResource("apps", "v1", "Deployment", "default", "existing-deployment")
		require.NoError(t, storeInstance.SaveComponents([]unstructured.Unstructured{obj}))

		updated, err := storeInstance.SetServiceChildren("abc", "123", []common.StoreKey{
			{
				GVK:       obj.GroupVersionKind(),
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName(),
			},
		})

		// Update the component
		require.NoError(t, err)
		require.Equal(t, 1, updated)

		// Get the component before sync
		componentBefore, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, componentBefore)
		require.Equal(t, componentBefore.ServiceID, "abc")
		require.Equal(t, componentBefore.ParentUID, "123")
	})
}
