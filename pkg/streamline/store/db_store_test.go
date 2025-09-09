package store_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pluralsh/deployment-operator/pkg/streamline/api"
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
	testNode      = "test-node"
)

var (
	nowTimestamp     = time.Now().Unix()
	hourAgoTimestamp = time.Now().Add(-time.Hour).Unix()
)

type CreateComponentOption func(component *client.ComponentChildAttributes)

func WithGroup(group string) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.Group = &group
	}
}

func WithVersion(version string) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.Version = version
	}
}

func WithKind(kind string) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.Kind = kind
	}
}

func WithNamespace(namespace string) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.Namespace = &namespace
	}
}

func WithName(name string) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.Name = name
	}
}

func WithState(state client.ComponentState) CreateComponentOption {
	return func(component *client.ComponentChildAttributes) {
		component.State = &state
	}
}

func createComponent(uid string, parentUID *string, option ...CreateComponentOption) client.ComponentChildAttributes {
	result := client.ComponentChildAttributes{
		UID:       uid,
		ParentUID: parentUID,
		Group:     lo.ToPtr(testGroup),
		Version:   testVersion,
		Kind:      testKind,
		Namespace: lo.ToPtr(testNamespace),
		Name:      testName,
		State:     lo.ToPtr(client.ComponentStateRunning),
	}

	for _, opt := range option {
		opt(&result)
	}

	return result
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

		component := createComponent(uid, nil, WithName("parent-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		childComponent := createComponent(testChildUID, &uid, WithName("child-component"))
		err = storeInstance.SaveComponentAttributes(childComponent)
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
		component := createComponent(rootUID, nil, WithName("root-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, &rootUID, WithName("level-1-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, &uid1, WithName("level-2-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, &uid2, WithName("level-3-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, &uid3, WithName("level-4-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, &uid4, WithName("level-5-component"))
		err = storeInstance.SaveComponentAttributes(component)
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
		component := createComponent(rootUID, nil, WithName("multi-root-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, &rootUID, WithName("multi-level-1-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, &uid1, WithName("multi-level-2-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, &uid2, WithName("multi-level-3-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, &uid3, WithName("multi-level-4-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		uid44 := "uid-44"
		component = createComponent(uid44, &uid3, WithName("multi-level-4b-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, &uid4, WithName("multi-level-5-component"))
		err = storeInstance.SaveComponentAttributes(component)
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
		component := createComponent(uid, nil, WithName("delete-parent-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, &uid, WithName("delete-child-component"))
		err = storeInstance.SaveComponentAttributes(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", &childUid, WithName("delete-grandchild-component"))
		err = storeInstance.SaveComponentAttributes(grandchildComponent)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 2)

		err = storeInstance.DeleteComponent(types.UID(childUid))
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
		component := createComponent(uid, nil, WithName("multi-delete-parent"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, &uid, WithName("multi-delete-child"))
		err = storeInstance.SaveComponentAttributes(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", &childUid, WithName("multi-delete-grandchild"))
		err = storeInstance.SaveComponentAttributes(grandchildComponent)
		require.NoError(t, err)

		child2Uid := "child2-uid"
		child2Component := createComponent(child2Uid, &uid, WithName("multi-delete-child2"))
		err = storeInstance.SaveComponentAttributes(child2Component)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 3)

		err = storeInstance.DeleteComponent(types.UID(childUid))
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
		component := createComponent(uid, nil, WithName("group-test-parent"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		child := createComponent("child-uid", &uid, WithGroup(group), WithName("group-test-child"))
		err = storeInstance.SaveComponentAttributes(child)
		require.NoError(t, err)

		children, err := storeInstance.GetComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Equal(t, group, *children[0].Group)

		// Test empty group
		child.UID = "child2-uid"
		child.Group = lo.ToPtr("")
		err = storeInstance.SaveComponentAttributes(child)
		require.NoError(t, err)

		tested, err := storeInstance.GetComponentByUID("child2-uid")
		require.NoError(t, err)
		require.Nil(t, tested.Group)

		// Test nil group
		child.UID = "child3-uid"
		child.Group = lo.ToPtr("")
		err = storeInstance.SaveComponentAttributes(child)
		require.NoError(t, err)

		tested, err = storeInstance.GetComponentByUID("child3-uid")
		require.NoError(t, err)
		require.Nil(t, tested.Group)
	})
}

func TestComponentCache_HealthScore(t *testing.T) {
	t.Run("cache should calculate correct health score", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		uid := testUID
		component := createComponent(uid, nil, WithState(client.ComponentStateRunning), WithKind("Pod"), WithName("test-pod-1"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		child1 := createComponent("child1", &uid, WithState(client.ComponentStateRunning), WithKind("Pod"), WithName("child-pod-1"))
		err = storeInstance.SaveComponentAttributes(child1)
		require.NoError(t, err)

		child2 := createComponent("child2", &uid, WithState(client.ComponentStateRunning), WithKind("Pod"), WithName("child-pod-2"))
		err = storeInstance.SaveComponentAttributes(child2)
		require.NoError(t, err)

		score, err := storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(100), score)

		child3 := createComponent("child3", &uid, WithState(client.ComponentStateFailed), WithKind("Pod"), WithName("child-pod-3"))
		err = storeInstance.SaveComponentAttributes(child3)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(75), score)

		child4 := createComponent("child4", &uid, WithState(client.ComponentStateFailed), WithKind("Deployment"), WithName("child-deployment-1"))
		err = storeInstance.SaveComponentAttributes(child4)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(60), score)

		// Invalid certificate should deduct an additional 10 points.
		child5 := createComponent("child5", &uid, WithState(client.ComponentStateFailed), WithKind("Certificate"), WithName("child-cert-1"))
		err = storeInstance.SaveComponentAttributes(child5)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(40), score)

		// Failing resources in kube-system namespace should deduct an additional 20 points.
		child6 := createComponent("child6", &uid, WithState(client.ComponentStateFailed), WithKind("Pod"), WithNamespace("kube-system"), WithName("child-pod-kube-system"))
		err = storeInstance.SaveComponentAttributes(child6)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(12), score)

		// Failing persistent volume should deduct an additional 10 points.
		// The score should not go below 0.
		child7 := createComponent("child7", &uid, WithState(client.ComponentStateFailed), WithKind("PersistentVolume"), WithName("child-pv-1"))
		err = storeInstance.SaveComponentAttributes(child7)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})

	t.Run("cache should calculate correct health score for components with no children", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		uid := testUID
		component := createComponent(uid, nil, WithState(client.ComponentStateRunning), WithName("standalone-component"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		score, err := storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(100), score)
	})

	t.Run("cache should calculate health score with critical system component failures", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		baseComponent := createComponent(testUID, nil, WithState(client.ComponentStateRunning), WithName("base-test-component"))
		err = storeInstance.SaveComponentAttributes(baseComponent)
		require.NoError(t, err)

		runningPod := createComponent("running-pod", nil, WithState(client.ComponentStateRunning), WithKind("Pod"), WithName("running-pod-unique"))
		err = storeInstance.SaveComponentAttributes(runningPod)
		require.NoError(t, err)

		runningDeployment := createComponent("running-deployment", nil, WithState(client.ComponentStateRunning), WithKind("Deployment"), WithName("running-deployment-unique"))
		err = storeInstance.SaveComponentAttributes(runningDeployment)
		require.NoError(t, err)

		runningService := createComponent("running-service", nil, WithState(client.ComponentStateRunning), WithKind("Service"), WithName("running-service-unique"))
		err = storeInstance.SaveComponentAttributes(runningService)
		require.NoError(t, err)

		// Test CoreDNS failure (50 point deduction)
		coredns := createComponent("coredns", nil, WithState(client.ComponentStateFailed), WithKind("Deployment"), WithName("coredns-test"))
		err = storeInstance.SaveComponentAttributes(coredns)
		require.NoError(t, err)

		score, err := storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(30), score)

		// Test AWS CNI failure (additional 50 point deduction)
		awscni := createComponent("aws-cni", nil, WithState(client.ComponentStateFailed), WithKind("DaemonSet"), WithName("aws-cni-test"))
		err = storeInstance.SaveComponentAttributes(awscni)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)

		// Test ingress-nginx service failure (would deduct 50 but already at 0)
		ingress := createComponent("ingress", nil, WithState(client.ComponentStateFailed), WithKind("Service"), WithName("ingress-nginx-controller-test"), WithNamespace("ingress-nginx"))
		err = storeInstance.SaveComponentAttributes(ingress)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})

	t.Run("cache should calculate health score with combined resource failures", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		baseComponent := createComponent(testUID, nil, WithState(client.ComponentStateRunning), WithName("base-combined-test"))
		err = storeInstance.SaveComponentAttributes(baseComponent)
		require.NoError(t, err)

		// Failed Certificate (10 point deduction)
		cert := createComponent("cert", nil, WithState(client.ComponentStateFailed), WithKind("Certificate"), WithName("test-cert-combined"))
		err = storeInstance.SaveComponentAttributes(cert)
		require.NoError(t, err)

		score, err := storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(40), score)

		// Failed kube-system resource (20 point deduction)
		kubeSystem := createComponent("kube-system-res", nil, WithState(client.ComponentStateFailed), WithKind("Pod"), WithNamespace("kube-system"), WithName("kube-system-pod-test"))
		err = storeInstance.SaveComponentAttributes(kubeSystem)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(3), score)

		// Failed PersistentVolume (10 point deduction)
		pv := createComponent("pv", nil, WithState(client.ComponentStateFailed), WithKind("PersistentVolume"), WithName("test-pv-combined"))
		err = storeInstance.SaveComponentAttributes(pv)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)

		// Failed istio-system resource (50 point deduction)
		istio := createComponent("istio-res", nil, WithState(client.ComponentStateFailed), WithKind("Service"), WithNamespace("istio-system"), WithName("istio-service-test"))
		err = storeInstance.SaveComponentAttributes(istio)
		require.NoError(t, err)

		score, err = storeInstance.GetHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})
}

func TestComponentCache_UniqueConstraint(t *testing.T) {
	t.Run("should allow components with different GVK-namespace-name combinations", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component1 := createComponent("uid-1", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component1)
		require.NoError(t, err)

		// Component with different name - should succeed
		component2 := createComponent("uid-2", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("my-other-app"))
		err = storeInstance.SaveComponentAttributes(component2)
		require.NoError(t, err)

		// Component with different namespace - should succeed
		component3 := createComponent("uid-3", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("production"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component3)
		require.NoError(t, err)

		// Component with different kind - should succeed
		component4 := createComponent("uid-4", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("StatefulSet"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component4)
		require.NoError(t, err)

		// Component with different version - should succeed
		component5 := createComponent("uid-5", nil,
			WithGroup("apps"),
			WithVersion("v2"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component5)
		require.NoError(t, err)

		// Component with different group - should succeed
		component6 := createComponent("uid-6", nil,
			WithGroup("extensions"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component6)
		require.NoError(t, err)
	})

	t.Run("should allow component updates", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component := createComponent("uid-1", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("my-app"))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		component.State = lo.ToPtr(client.ComponentStatePaused)
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)
	})

	t.Run("should allow components with same GVK-namespace-name but different UID", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		component1 := createComponent("uid-1", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("duplicate-app"))
		err = storeInstance.SaveComponentAttributes(component1)
		require.NoError(t, err)

		component2 := createComponent("uid-2", nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("duplicate-app"))
		err = storeInstance.SaveComponentAttributes(component2)
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
		component := createComponent(uid, nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("updatable-app"),
			WithState(client.ComponentStateRunning))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		// Update the same component with different state - should succeed
		updatedComponent := createComponent(uid, nil,
			WithGroup("apps"),
			WithVersion("v1"),
			WithKind("Deployment"),
			WithNamespace("default"),
			WithName("updatable-app"),
			WithState(client.ComponentStateFailed))
		err = storeInstance.SaveComponentAttributes(updatedComponent)
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

		component := createComponent("uid-1", nil, WithGroup(group), WithVersion(version), WithKind(kind), WithName(name), WithNamespace(namespace))
		err = storeInstance.SaveComponentAttributes(component)
		require.NoError(t, err)

		sameComponentWithDifferentUID := createComponent("uid-2", nil, WithGroup(group), WithVersion(version), WithKind(kind), WithName(name), WithNamespace(namespace))
		err = storeInstance.SaveComponentAttributes(sameComponentWithDifferentUID)
		require.NoError(t, err)

		dbc, err := storeInstance.GetComponent(u)
		require.NoError(t, err)
		assert.Equal(t, "uid-2", dbc.UID)
	})

	t.Run("should treat nil values in the same way as empty strings", func(t *testing.T) {
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

		componentWithEmptyNamespace := createComponent("uid", nil, WithGroup(group), WithVersion(version), WithKind(kind), WithName(name), WithNamespace(namespace))
		err = storeInstance.SaveComponentAttributes(componentWithEmptyNamespace)
		require.NoError(t, err)

		componentWithNilNamespace := createComponent("uid-2", nil, WithGroup(group), WithVersion(version), WithKind(kind), WithName(name))
		componentWithNilNamespace.Namespace = nil
		err = storeInstance.SaveComponentAttributes(componentWithNilNamespace)
		require.NoError(t, err)

		databaseComponent, err := storeInstance.GetComponent(u)
		require.NoError(t, err)
		assert.Equal(t, "uid-2", databaseComponent.UID, "component in database should have updated UID")

		entry, err := storeInstance.GetComponentByUID("uid")
		require.NoError(t, err)
		require.Nil(t, entry, "component with old UID should not be found")
	})
}

func createPod(s store.Store, name, uid string, timestamp int64) error {
	return s.SaveComponentAttributes(
		createComponent(uid, nil, WithKind("Pod"), WithName(name), WithNamespace(testNamespace), WithState(client.ComponentStateFailed)), testNode, timestamp, nil)
}

func TestPendingPodsCache(t *testing.T) {
	t.Run("cache should store pods with all required attributes", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		require.NoError(t, createPod(storeInstance, "pending-pod-1", "pod-1-uid", hourAgoTimestamp))
		require.NoError(t, createPod(storeInstance, "pending-pod-2", "pod-2-uid", hourAgoTimestamp))

		stats, err := storeInstance.GetNodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should ignore fresh pending pods that were created within last 5 minutes", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		require.NoError(t, createPod(storeInstance, "fresh-pending-pod", "pod-uid", nowTimestamp))
		require.NoError(t, createPod(storeInstance, "pending-pod-1", "pod-1-uid", hourAgoTimestamp))
		require.NoError(t, createPod(storeInstance, "pending-pod-2", "pod-2-uid", hourAgoTimestamp))

		stats, err := storeInstance.GetNodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should delete pod", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		require.NoError(t, createPod(storeInstance, "pending-pod-1", "pod-1-uid", hourAgoTimestamp))

		stats, err := storeInstance.GetNodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, int64(1), *stats[0].PendingPods)

		err = storeInstance.DeleteComponent("pod-1-uid")
		require.NoError(t, err)

		stats, err = storeInstance.GetNodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 0)
	})
}

func TestComponentCache_ComponentInsights(t *testing.T) {
	t.Run("should retrieve expected component insights without errors", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		// Define test components with various states
		testComponents := []client.ComponentChildAttributes{
			// Running components
			createComponent("app-frontend-1", nil, WithKind("Deployment"), WithState(client.ComponentStateRunning), WithName("app-frontend-1")),
			createComponent("app-backend-1", nil, WithKind("Deployment"), WithState(client.ComponentStateRunning), WithName("app-backend-1")),
			createComponent("app-database-1", nil, WithKind("Deployment"), WithState(client.ComponentStateRunning), WithName("app-database-1")),

			// Running components chain (ignored because of depth level > 4)
			createComponent("app-1", nil, WithKind("Deployment"), WithState(client.ComponentStateRunning), WithName("app-1")),
			createComponent("app-child-1", lo.ToPtr("app-1"), WithKind("Pod"), WithState(client.ComponentStateRunning), WithName("app-child-1")),
			createComponent("app-child-2", lo.ToPtr("app-child-1"), WithKind("Pod"), WithState(client.ComponentStateRunning), WithName("app-child-2")),
			createComponent("app-child-3", lo.ToPtr("app-child-2"), WithKind("Pod"), WithState(client.ComponentStateRunning), WithName("app-child-3")),
			createComponent("app-child-4", lo.ToPtr("app-child-3"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-child-4")),

			// 1-level Failed components
			createComponent("app-redis-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-redis-1")),
			createComponent("app-cronjob-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-cronjob-1")),

			// Pending component
			createComponent("app-migration-1", nil, WithKind("Deployment"), WithState(client.ComponentStatePending), WithName("app-migration-1")),

			// Ingress (failed) -> Certificate (failed)
			createComponent("app-ingress-1", nil, WithKind("Ingress"), WithState(client.ComponentStateFailed), WithName("app-ingress-1")),
			createComponent("app-certificate-1", lo.ToPtr("app-ingress-1"), WithKind("Certificate"), WithState(client.ComponentStateFailed), WithName("app-certificate-1")),

			// Ingress (pending) -> Certificate (failed)
			createComponent("app-ingress-2", nil, WithKind("Ingress"), WithState(client.ComponentStatePending), WithName("app-ingress-2")),
			createComponent("app-certificate-2", lo.ToPtr("app-ingress-2"), WithKind("Certificate"), WithState(client.ComponentStateFailed), WithName("app-certificate-2")),

			// StatefulSet (failed)
			createComponent("app-statefulset-1", nil, WithKind("StatefulSet"), WithState(client.ComponentStateFailed), WithName("app-statefulset-1")),

			// DaemonSet (failed)
			createComponent("app-daemonset-1", nil, WithKind("DaemonSet"), WithState(client.ComponentStateFailed), WithName("app-daemonset-1")),

			// Deployment (pending) -> Pod (failed)
			createComponent("app-deployment-1", nil, WithKind("Deployment"), WithState(client.ComponentStatePending), WithName("app-deployment-1")),
			createComponent("app-pod-1", lo.ToPtr("app-deployment-1"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-pod-1")),

			// CRD (pending) -> Deployment (pending) -> Pod (failed)
			createComponent("app-crd-1", nil, WithKind("CustomResourceDefinition"), WithState(client.ComponentStatePending), WithName("app-crd-1")),
			createComponent("app-deployment-2", lo.ToPtr("app-crd-1"), WithKind("Deployment"), WithState(client.ComponentStatePending), WithName("app-deployment-2")),
			createComponent("app-pod-2", lo.ToPtr("app-deployment-2"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-pod-2")),

			// CRD (failed) -> Deployment (failed) -> Pod (failed)
			createComponent("app-crd-2", nil, WithKind("CustomResourceDefinition"), WithState(client.ComponentStateFailed), WithName("app-crd-2")),
			createComponent("app-deployment-3", lo.ToPtr("app-crd-2"), WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-deployment-3")),
			createComponent("app-pod-3", lo.ToPtr("app-deployment-3"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-pod-3")),

			// CRD (failed) -> Deployment (failed) -> ReplicaSet (failed) -> Pod (failed)
			createComponent("app-crd-3", nil, WithKind("CustomResourceDefinition"), WithState(client.ComponentStateFailed), WithName("app-crd-3")),
			createComponent("app-deployment-4", lo.ToPtr("app-crd-3"), WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-deployment-4")),
			createComponent("app-replicaset-1", lo.ToPtr("app-deployment-4"), WithKind("ReplicaSet"), WithState(client.ComponentStateFailed), WithName("app-replicaset-1")),
			createComponent("app-pod-4", lo.ToPtr("app-replicaset-1"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-pod-4")),

			// Deployment (pending) -> (ReplicaSet (pending) -> Pod (failed)) | Secret (running)
			createComponent("app-deployment-5", nil, WithKind("Deployment"), WithState(client.ComponentStatePending), WithName("app-deployment-5")),
			createComponent("app-replicaset-2", lo.ToPtr("app-deployment-5"), WithKind("ReplicaSet"), WithState(client.ComponentStatePending), WithName("app-replicaset-2")),
			createComponent("app-pod-5", lo.ToPtr("app-replicaset-2"), WithKind("Pod"), WithState(client.ComponentStateFailed), WithName("app-pod-5")),
			createComponent("app-secret-1", lo.ToPtr("app-deployment-5"), WithKind("Secret"), WithState(client.ComponentStateRunning), WithName("app-secret-1")),
		}

		expectedComponents := []string{
			"app-redis-1",
			"app-cronjob-1",
			"app-ingress-1",
			"app-certificate-1",
			"app-ingress-2",
			"app-certificate-2",
			"app-statefulset-1",
			"app-daemonset-1",
			"app-deployment-1",
			"app-deployment-2",
			"app-deployment-3",
			"app-deployment-4",
			"app-deployment-5",
		}

		// Insert all test components into cache
		for _, tc := range testComponents {
			err := storeInstance.SaveComponentAttributes(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := storeInstance.GetComponentInsights()
		require.NoError(t, err, "Failed to get component insights")

		actualNames := algorithms.Map(
			insights,
			func(i client.ClusterInsightComponentAttributes) string { return i.Name },
		)

		// Verify expected components in insights
		// Sort both arrays to ensure order-independent comparison
		sort.Strings(actualNames)
		sort.Strings(expectedComponents)

		require.Equal(t,
			expectedComponents,
			actualNames,
			"Expected components not found in insights",
		)
	})

	t.Run("should properly assign priorities based on component kind", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		// Define test components with various kinds to test priority assignment
		testComponents := []client.ComponentChildAttributes{
			// Critical priority resources
			createComponent("ingress-1", nil, WithKind("Ingress"), WithState(client.ComponentStateFailed), WithName("ingress-1")),
			createComponent("certificate-1", nil, WithKind("Certificate"), WithState(client.ComponentStateFailed), WithName("certificate-1")),
			createComponent("cert-manager-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("cert-manager-webhook"), WithNamespace("cert-manager")),
			createComponent("coredns-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("coredns")),

			// High priority resources
			createComponent("statefulset-1", nil, WithKind("StatefulSet"), WithState(client.ComponentStateFailed), WithName("statefulset-1")),
			createComponent("node-exporter", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("node-exporter")),

			// Medium priority resources
			createComponent("daemonset-1", nil, WithKind("DaemonSet"), WithState(client.ComponentStateFailed), WithName("daemonset-1")),

			// Low priority resources (default)
			createComponent("deployment-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("deployment-1")),
		}

		expectedComponentPriorityMap := map[string]client.InsightComponentPriority{
			"ingress-1":            client.InsightComponentPriorityCritical,
			"certificate-1":        client.InsightComponentPriorityCritical,
			"cert-manager-webhook": client.InsightComponentPriorityCritical,
			"coredns":              client.InsightComponentPriorityCritical,
			"statefulset-1":        client.InsightComponentPriorityHigh,
			"node-exporter":        client.InsightComponentPriorityHigh,
			"daemonset-1":          client.InsightComponentPriorityMedium,
			"deployment-1":         client.InsightComponentPriorityLow,
		}

		// Insert all test components into cache
		for _, tc := range testComponents {
			err := storeInstance.SaveComponentAttributes(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := storeInstance.GetComponentInsights()
		require.NoError(t, err, "Failed to get component insights")

		// Build a map of component name to priority for easier testing
		priorityMap := make(map[string]client.InsightComponentPriority)
		for _, insight := range insights {
			priorityMap[insight.Name] = *insight.Priority
		}

		for name, expectedPriority := range expectedComponentPriorityMap {
			assert.Equal(t, expectedPriority, priorityMap[name], "Priority for %s should be %s", name, expectedPriority)
		}
	})

	t.Run("should assign priorities based on string similarity in resource names and namespaces", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func() {
			if err := storeInstance.Shutdown(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		// Define test components with variations of similar names to test fuzzy matching
		testComponents := []client.ComponentChildAttributes{
			// Components with names very similar to critical priority resources
			createComponent("cert-manager-1", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("cert-manager"), WithNamespace("kube-system")),
			createComponent("coredns-similar", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("core-dns"), WithNamespace("kube-system")),
			createComponent("istio-ingressgateway", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("istio-ingressgateway"), WithNamespace("istio-system")),
			createComponent("linkerd-proxy", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("linkerd-proxy"), WithNamespace("linkerd")),
			createComponent("ebs-csi-node", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("ebs-csi-node"), WithNamespace("kube-system")),
			createComponent("gce-pd-csi-controller", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("gce-pd-csi-controller-sa"), WithNamespace("kube-system")),

			// Components with namespaces containing priority keywords
			createComponent("app-in-cert-manager", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-in-sensitive-ns-1"), WithNamespace("cert-manager")),
			createComponent("app-in-kube-proxy", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("app-in-sensitive-ns-2"), WithNamespace("kube-proxy")),

			// Components with partial name matches to high priority resources
			createComponent("node-exporter-similar", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("node-metrics-exporter"), WithNamespace("monitoring")),

			// Components with no special priority that could be slightly similar to other resources
			createComponent("app-similar-to-istio", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("iso-ist-app"), WithNamespace("default")),
			createComponent("app-similar-to-cert-manager", nil, WithKind("Deployment"), WithState(client.ComponentStateFailed), WithName("custom-cert-provisioner"), WithNamespace("default")),
		}

		expectedComponentPriorityMap := map[string]client.InsightComponentPriority{
			"cert-manager":             client.InsightComponentPriorityCritical,
			"core-dns":                 client.InsightComponentPriorityCritical,
			"istio-ingressgateway":     client.InsightComponentPriorityCritical,
			"linkerd-proxy":            client.InsightComponentPriorityCritical,
			"ebs-csi-node":             client.InsightComponentPriorityCritical,
			"gce-pd-csi-controller-sa": client.InsightComponentPriorityCritical,
			"app-in-sensitive-ns-1":    client.InsightComponentPriorityCritical,
			"app-in-sensitive-ns-2":    client.InsightComponentPriorityCritical,
			"node-metrics-exporter":    client.InsightComponentPriorityHigh,
			"iso-ist-app":              client.InsightComponentPriorityLow,
			"custom-cert-provisioner":  client.InsightComponentPriorityLow,
		}

		// Insert all test components into cache
		for _, tc := range testComponents {
			err := storeInstance.SaveComponentAttributes(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := storeInstance.GetComponentInsights()
		require.NoError(t, err, "Failed to get component insights")

		// Build a map of component name to priority for easier testing
		priorityMap := make(map[string]client.InsightComponentPriority)
		for _, insight := range insights {
			priorityMap[insight.Name] = *insight.Priority
		}

		for name, expectedPriority := range expectedComponentPriorityMap {
			assert.Equal(t, expectedPriority, priorityMap[name], "Priority for %s should be %s", name, expectedPriority)
		}
	})

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

		testComponents := []client.ComponentChildAttributes{
			// Components with names very similar to critical priority resources
			createComponent("a", nil, WithKind("Namespace"), WithState(client.ComponentStateRunning), WithName("a")),
			createComponent("b", nil, WithKind("Namespace"), WithState(client.ComponentStateRunning), WithName("b")),
			createComponent("c", nil, WithKind("Namespace"), WithState(client.ComponentStateRunning), WithName("c")),
			createComponent("node-1", nil, WithKind("Node"), WithState(client.ComponentStateRunning), WithName("node-1")),
			createComponent("node-2", nil, WithKind("Node"), WithState(client.ComponentStateRunning), WithName("node-2")),
			createComponent("node-3", nil, WithKind("Node"), WithState(client.ComponentStateRunning), WithName("node-3")),
		}

		// Insert all test components into cache
		for _, tc := range testComponents {
			err := storeInstance.SaveComponentAttributes(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

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
		node1 := createComponent("node-1", nil, WithKind("Node"), WithName("worker-1"))
		err = storeInstance.SaveComponentAttributes(node1)
		require.NoError(t, err)

		node2 := createComponent("node-2", nil, WithKind("Node"), WithName("worker-2"))
		err = storeInstance.SaveComponentAttributes(node2)
		require.NoError(t, err)

		// Create 3 namespaces
		ns1 := createComponent("ns-1", nil, WithKind("Namespace"), WithName("default"))
		err = storeInstance.SaveComponentAttributes(ns1)
		require.NoError(t, err)

		ns2 := createComponent("ns-2", nil, WithKind("Namespace"), WithName("kube-system"))
		err = storeInstance.SaveComponentAttributes(ns2)
		require.NoError(t, err)

		ns3 := createComponent("ns-3", nil, WithKind("Namespace"), WithName("production"))
		err = storeInstance.SaveComponentAttributes(ns3)
		require.NoError(t, err)

		// Create some other resources that should not be counted
		pod := createComponent("pod-1", nil, WithKind("Pod"), WithName("test-pod"))
		err = storeInstance.SaveComponentAttributes(pod)
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
		parent := createComponent(parentUID, nil, WithName("parent-with-many-children"))
		err = storeInstance.SaveComponentAttributes(parent)
		require.NoError(t, err)

		// Create 150 child components to test the 100 limit
		totalChildren := 150
		for i := 0; i < totalChildren; i++ {
			childUID := fmt.Sprintf("child-%d", i)
			childName := fmt.Sprintf("child-component-%d", i)
			child := createComponent(childUID, &parentUID, WithName(childName))
			err := storeInstance.SaveComponentAttributes(child)
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
		parent := createComponent(parentUID, nil, WithName("parent-with-few-children"))
		err = storeInstance.SaveComponentAttributes(parent)
		require.NoError(t, err)

		// Create 50 child components (under the limit)
		totalChildren := 50
		for i := 0; i < totalChildren; i++ {
			childUID := fmt.Sprintf("few-child-%d", i)
			childName := fmt.Sprintf("few-child-component-%d", i)
			child := createComponent(childUID, &parentUID, WithName(childName))
			err := storeInstance.SaveComponentAttributes(child)
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
		root := createComponent(rootUID, nil, WithName("root-with-deep-hierarchy"))
		err = storeInstance.SaveComponentAttributes(root)
		require.NoError(t, err)

		// Create a multi-level hierarchy that exceeds 100 total descendants
		// Level 1: 30 components
		level1UIDs := make([]string, 30)
		for i := 0; i < 30; i++ {
			uid := fmt.Sprintf("level1-%d", i)
			level1UIDs[i] = uid
			component := createComponent(uid, &rootUID, WithName(fmt.Sprintf("level1-component-%d", i)))
			err := storeInstance.SaveComponentAttributes(component)
			require.NoError(t, err)
		}

		// Level 2: 40 components (distributed among level 1 components)
		level2Count := 0
		for i := 0; i < 20 && level2Count < 40; i++ {
			parentUID := level1UIDs[i%len(level1UIDs)]
			for j := 0; j < 2 && level2Count < 40; j++ {
				uid := fmt.Sprintf("level2-%d-%d", i, j)
				component := createComponent(uid, &parentUID, WithName(fmt.Sprintf("level2-component-%d-%d", i, j)))
				err := storeInstance.SaveComponentAttributes(component)
				require.NoError(t, err)
				level2Count++
			}
		}

		// Level 3: 50 components (this will push us over 100 total)
		level3Count := 0
		for i := 0; i < 25 && level3Count < 50; i++ {
			// Find a level 2 component to be parent
			level2UID := fmt.Sprintf("level2-%d-0", i%20)
			for j := 0; j < 2 && level3Count < 50; j++ {
				uid := fmt.Sprintf("level3-%d-%d", i, j)
				component := createComponent(uid, &level2UID, WithName(fmt.Sprintf("level3-component-%d-%d", i, j)))
				err := storeInstance.SaveComponentAttributes(component)
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

		obj := unstructured.Unstructured{}
		obj.SetUID("test")
		obj.SetName("test")
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "test",
			Version: "v1",
			Kind:    "Test",
		})

		err = storeInstance.SaveComponent(obj)
		require.NoError(t, err)

		err = storeInstance.UpdateComponentSHA(obj, store.ApplySHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ServerSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ManifestSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA)
		require.NoError(t, err)

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

		obj := unstructured.Unstructured{}
		obj.SetUID("test")
		obj.SetName("test")
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "test",
			Version: "v1",
			Kind:    "Test",
		})

		err = storeInstance.SaveComponent(obj)
		require.NoError(t, err)

		err = storeInstance.UpdateComponentSHA(obj, store.ApplySHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ServerSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ManifestSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA)
		require.NoError(t, err)

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

		obj := unstructured.Unstructured{}
		obj.SetUID("test")
		obj.SetName("test")
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "test",
			Version: "v1",
			Kind:    "Test",
		})

		err = storeInstance.SaveComponent(obj)
		require.NoError(t, err)

		err = storeInstance.UpdateComponentSHA(obj, store.ApplySHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ServerSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.ManifestSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA)
		require.NoError(t, err)

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

		obj := unstructured.Unstructured{}
		obj.SetUID("test")
		obj.SetName("test")
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "test",
			Version: "v1",
			Kind:    "Test",
		})

		err = storeInstance.SaveComponent(obj)
		require.NoError(t, err)

		err = storeInstance.UpdateComponentSHA(obj, store.ApplySHA)
		require.NoError(t, err)

		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)

		time.Sleep(time.Second)

		err = storeInstance.ExpireOlderThan(2 * time.Second)
		require.NoError(t, err)

		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)

		err = storeInstance.UpdateComponentSHA(obj, store.ApplySHA)
		require.NoError(t, err)
		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)

		time.Sleep(1500 * time.Millisecond)

		err = storeInstance.ExpireOlderThan(2 * time.Second)
		require.NoError(t, err)
		entry, err = storeInstance.GetComponent(obj)
		require.NoError(t, err)
		require.NotNil(t, entry)
		assert.NotEmpty(t, entry.ApplySHA)
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
		c1 := createComponent("gvk-uid-1", nil, WithGroup(gvk.Group), WithVersion(gvk.Version), WithKind(gvk.Kind), WithNamespace("ns-1"), WithName("alpha"))
		require.NoError(t, storeInstance.SaveComponentAttributes(c1))

		c2 := createComponent("gvk-uid-2", nil, WithGroup(gvk.Group), WithVersion(gvk.Version), WithKind(gvk.Kind), WithNamespace("ns-2"), WithName("beta"))
		require.NoError(t, storeInstance.SaveComponentAttributes(c2))

		c3 := createComponent("gvk-uid-3", nil, WithGroup(gvk.Group), WithVersion(gvk.Version), WithKind(gvk.Kind), WithNamespace("ns-3"), WithName("gamma"))
		require.NoError(t, storeInstance.SaveComponentAttributes(c3))

		// Insert components with different GVK to ensure they are filtered out
		diff1 := createComponent("other-uid-1", nil, WithGroup("apps"), WithVersion("v1"), WithKind("StatefulSet"), WithNamespace("ns-1"), WithName("alpha"))
		require.NoError(t, storeInstance.SaveComponentAttributes(diff1))

		diff2 := createComponent("other-uid-2", nil, WithGroup("extensions"), WithVersion("v1"), WithKind("Deployment"), WithNamespace("ns-1"), WithName("delta"))
		require.NoError(t, storeInstance.SaveComponentAttributes(diff2))

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
			assert.Equal(t, "", e.ServerSHA, "server SHA is not set when saving component attributes, it should be empty")
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
		component1 := createComponent("uid-1", nil, WithGroup(deploymentsGVK.Group), WithVersion(deploymentsGVK.Version), WithKind(deploymentsGVK.Kind), WithName("deployment-1"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component1)
		require.NoError(t, err)

		component2 := createComponent("uid-2", nil,
			WithGroup(deploymentsGVK.Group), WithVersion(deploymentsGVK.Version), WithKind(deploymentsGVK.Kind),
			WithName("deployment-2"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component2)
		require.NoError(t, err)

		component3 := createComponent("uid-3", nil,
			WithGroup(deploymentsGVK.Group), WithVersion(deploymentsGVK.Version), WithKind(deploymentsGVK.Kind),
			WithName("deployment-3"), WithNamespace("kube-system"))
		err = storeInstance.SaveComponentAttributes(component3)
		require.NoError(t, err)

		// Create component with different GVK
		component4 := createComponent("uid-4", nil,
			WithGroup(servicesGVK.Group), WithVersion(servicesGVK.Version), WithKind(servicesGVK.Kind),
			WithName("service-1"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component4)
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
		assert.Equal(t, "uid-4", services[0].UID)
	})

	t.Run("should handle empty group in delete operation", func(t *testing.T) {
		storeInstance, err := store.NewDatabaseStore(store.WithStorage(api.StorageFile))
		assert.NoError(t, err)
		defer func(storeInstance store.Store) {
			require.NoError(t, storeInstance.Shutdown(), "failed to shutdown store")
		}(storeInstance)

		// Create components with empty group (core resources)
		pod1 := createComponent("pod-uid-1", nil, WithGroup(""), WithVersion("v1"), WithKind("Pod"), WithName("pod-1"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(pod1)
		require.NoError(t, err)

		pod2 := createComponent("pod-uid-2", nil, WithGroup(""), WithVersion("v1"), WithKind("Pod"), WithName("pod-2"), WithNamespace("kube-system"))
		err = storeInstance.SaveComponentAttributes(pod2)
		require.NoError(t, err)

		service := createComponent("service-uid", nil, WithGroup(""), WithVersion("v1"), WithKind("Service"), WithName("service-1"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(service)
		require.NoError(t, err)

		// Verify pods exist
		pods, err := storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
		require.NoError(t, err)
		assert.Len(t, pods, 2)

		// Delete all pods (empty group)
		err = storeInstance.DeleteComponents("", "v1", "Pod")
		require.NoError(t, err)

		// Verify pods are deleted
		pods, err = storeInstance.GetComponentsByGVK(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
		require.NoError(t, err)
		assert.Len(t, pods, 0)

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

		component := createComponent("existing-uid", nil, WithGroup("apps"), WithVersion("v1"), WithKind("Deployment"), WithName("existing-deployment"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component)
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
		component1 := createComponent("service-comp-1", nil, WithGroup("apps"), WithVersion("v1"), WithKind("Deployment"), WithName("app-deployment"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component1, "test-node", time.Now().Unix(), serviceID)
		require.NoError(t, err)

		component2 := createComponent("service-comp-2", nil, WithGroup(""), WithVersion("v1"), WithKind("Pod"), WithName("app-pod"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(component2, "test-node", time.Now().Unix(), serviceID)
		require.NoError(t, err)

		// Create component with different service ID
		otherComponent := createComponent("other-comp", nil, WithGroup("apps"), WithVersion("v1"), WithKind("Deployment"), WithName("other-deployment"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(otherComponent, "test-node", time.Now().Unix(), "other-service")
		require.NoError(t, err)

		// Create component with no service ID
		noServiceComponent := createComponent("no-service-comp", nil, WithGroup(""), WithVersion("v1"), WithKind("Service"), WithName("no-service"), WithNamespace("default"))
		err = storeInstance.SaveComponentAttributes(noServiceComponent, "test-node", time.Now().Unix(), nil)
		require.NoError(t, err)

		components, err := storeInstance.GetServiceComponents(serviceID)
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
		component := createComponent("test-comp", nil, WithName("test-component"))
		err = storeInstance.SaveComponentAttributes(component, "test-node", time.Now().Unix(), "existing-service")
		require.NoError(t, err)

		// Try to get components for non-existent service
		components, err := storeInstance.GetServiceComponents("non-existent-service")
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

		obj := unstructured.Unstructured{}
		obj.SetUID("test-expire")
		obj.SetName("test-component")
		obj.SetNamespace("default")
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

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

		obj := unstructured.Unstructured{}
		obj.SetUID("test-expire-sha")
		obj.SetName("test-component")
		obj.SetNamespace("default")
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

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

		// Create an unstructured object
		obj := unstructured.Unstructured{}
		obj.SetUID("test-commit-transient")
		obj.SetName("test-component")
		obj.SetNamespace("default")
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

		require.NoError(t, storeInstance.SaveComponent(obj))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.ManifestSHA))
		require.NoError(t, storeInstance.UpdateComponentSHA(obj, store.TransientManifestSHA))

		// Get initial state
		entry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		initialManifestSHA := entry.ManifestSHA
		transientSHA := entry.TransientManifestSHA
		assert.NotEmpty(t, initialManifestSHA)
		assert.NotEmpty(t, transientSHA)

		// Commit transient SHA
		err = storeInstance.CommitTransientSHA(obj)
		require.NoError(t, err)

		// Verify transient SHA was committed
		updatedEntry, err := storeInstance.GetComponent(obj)
		require.NoError(t, err)
		assert.Equal(t, transientSHA, updatedEntry.ManifestSHA)
		assert.Empty(t, updatedEntry.TransientManifestSHA)
	})
}
