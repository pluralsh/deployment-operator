package applier

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheFilter(t *testing.T) {
	const (
		resourceName = "test-filter"
		namespace    = "default"
	)

	// Setup function to create test environment
	setupTest := func(t *testing.T) (store.Store, unstructured.Unstructured, func()) {
		// Initialize test store
		storeInstance, err := store.NewDatabaseStore()
		require.NoError(t, err)

		// Initialize global store
		streamline.InitGlobalStore(storeInstance)

		// Create test pod
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
				UID:       "test-uid-123",
				Labels: map[string]string{
					common.ManagedByLabel: common.AgentLabelValue,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test",
						Image: "test:v1",
					},
				},
			},
		}

		// Convert to unstructured
		res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		require.NoError(t, err)
		unstructuredPod := unstructured.Unstructured{Object: res}

		// Cleanup function
		cleanup := func() {
			if storeInstance != nil {
				err := storeInstance.Shutdown()
				require.NoError(t, err)
			}
		}

		return storeInstance, unstructuredPod, cleanup
	}

	t.Run("should return true for cache miss when component not in store", func(t *testing.T) {
		_, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// First time applying resource - should be cache miss (return true)
		result := CacheFilter()(unstructuredPod)
		assert.True(t, result)
	})

	t.Run("should return true for cache miss when component has no SHA", func(t *testing.T) {
		storeInstance, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// Save component without SHA
		err := storeInstance.SaveComponent(unstructuredPod)
		require.NoError(t, err)

		// Should be cache miss since no SHAs are set
		result := CacheFilter()(unstructuredPod)
		assert.True(t, result)
	})

	t.Run("should return false for cache hit when manifest hasn't changed", func(t *testing.T) {
		storeInstance, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// Save component
		err := storeInstance.SaveComponent(unstructuredPod)
		require.NoError(t, err)

		// Set manifest SHA (simulate previous apply)
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ManifestSHA)
		require.NoError(t, err)

		// Set apply SHA
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ApplySHA)
		require.NoError(t, err)

		// Set server SHA (same as apply SHA to simulate no drift)
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ServerSHA)
		require.NoError(t, err)

		// Should be cache hit since manifest hasn't changed
		result := CacheFilter()(unstructuredPod)
		assert.False(t, result)
	})

	t.Run("should return true for cache miss when manifest has changed", func(t *testing.T) {
		storeInstance, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// Save component
		err := storeInstance.SaveComponent(unstructuredPod)
		require.NoError(t, err)

		// Set SHAs for original manifest
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ManifestSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ApplySHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ServerSHA)
		require.NoError(t, err)

		// Modify the pod manifest (change image)
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
				UID:       "test-uid-123",
				Labels: map[string]string{
					common.ManagedByLabel: common.AgentLabelValue,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test",
						Image: "test:v2", // Changed image
					},
				},
			},
		}
		res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		require.NoError(t, err)
		modifiedPod := unstructured.Unstructured{Object: res}

		// Should be cache miss since manifest has changed
		result := CacheFilter()(modifiedPod)
		assert.True(t, result)
	})

	t.Run("should return true for cache miss when server SHA differs from apply SHA", func(t *testing.T) {
		storeInstance, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// Save component
		err := storeInstance.SaveComponent(unstructuredPod)
		require.NoError(t, err)

		// Set manifest and apply SHA
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ManifestSHA)
		require.NoError(t, err)
		err = storeInstance.UpdateComponentSHA(unstructuredPod, store.ApplySHA)
		require.NoError(t, err)

		// Simulate server drift by modifying the pod and setting different server SHA
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
				UID:       "test-uid-123",
				Labels: map[string]string{
					common.ManagedByLabel: common.AgentLabelValue,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test",
						Image: "test:drifted",
					},
				},
			},
		}
		res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		require.NoError(t, err)
		driftedPod := unstructured.Unstructured{Object: res}

		err = storeInstance.UpdateComponentSHA(driftedPod, store.ServerSHA)
		require.NoError(t, err)

		// Use original pod - should detect drift and return cache miss
		result := CacheFilter()(unstructuredPod)
		assert.True(t, result)
	})

	t.Run("should update transient manifest SHA on each call", func(t *testing.T) {
		storeInstance, unstructuredPod, cleanup := setupTest(t)
		defer cleanup()

		// Save component
		err := storeInstance.SaveComponent(unstructuredPod)
		require.NoError(t, err)

		// Call filter - should update transient SHA
		CacheFilter()(unstructuredPod)

		// Verify component exists and transient SHA was updated
		entry, err := storeInstance.GetComponent(unstructuredPod)
		require.NoError(t, err)
		assert.NotEmpty(t, entry.TransientManifestSHA)
	})
}
