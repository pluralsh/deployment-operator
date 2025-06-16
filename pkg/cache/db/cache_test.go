package db_test

import (
	"sort"
	"testing"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pluralsh/deployment-operator/pkg/cache/db"
)

const (
	dbFile        = "/tmp/component-cache.db"
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
		db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(dbFile))
		defer db.GetComponentCache().Close()
	})
}

func TestComponentCache_SetComponent(t *testing.T) {
	t.Run("cache should save and return simple parent and child structure", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID

		component := createComponent(uid, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		childComponent := createComponent(testChildUID, &uid)
		err = db.GetComponentCache().SetComponent(childComponent)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, testChildUID, children[0].UID)
		assert.Equal(t, uid, *children[0].ParentUID)
	})
}

func TestComponentCache_ComponentChildren(t *testing.T) {
	t.Run("cache should save and return multi-level structure", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		// Root
		rootUID := "root-uid"
		component := createComponent(rootUID, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, &rootUID)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, &uid1)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, &uid2)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, &uid3)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, &uid4)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 4)
	})

	t.Run("cache should save and return multi-level structure with multiple children", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		// Root
		rootUID := testUID
		component := createComponent(rootUID, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = createComponent(uid1, &rootUID)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = createComponent(uid2, &uid1)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = createComponent(uid3, &uid2)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = createComponent(uid4, &uid3)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		uid44 := "uid-44"
		component = createComponent(uid44, &uid3)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = createComponent(uid5, &uid4)
		err = db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 5)
	})
}

func TestComponentCache_DeleteComponent(t *testing.T) {
	t.Run("cache should support basic cascade deletion", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID
		component := createComponent(uid, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, &uid)
		err = db.GetComponentCache().SetComponent(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", &childUid)
		err = db.GetComponentCache().SetComponent(grandchildComponent)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 2)

		err = db.GetComponentCache().DeleteComponent(childUid)
		require.NoError(t, err)

		children, err = db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 0)
	})

	t.Run("cache should support multi-level cascade deletion", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID
		component := createComponent(uid, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := createComponent(childUid, &uid)
		err = db.GetComponentCache().SetComponent(childComponent)
		require.NoError(t, err)

		grandchildComponent := createComponent("grandchild-uid", &childUid)
		err = db.GetComponentCache().SetComponent(grandchildComponent)
		require.NoError(t, err)

		child2Uid := "child2-uid"
		child2Component := createComponent(child2Uid, &uid)
		err = db.GetComponentCache().SetComponent(child2Component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 3)

		err = db.GetComponentCache().DeleteComponent(childUid)
		require.NoError(t, err)

		children, err = db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
	})
}

func TestComponentCache_GroupHandling(t *testing.T) {
	t.Run("cache should correctly store and return group", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		group := testGroup

		uid := testUID
		component := createComponent(uid, nil)
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		child := createComponent(testChildUID, &uid, WithGroup(group))
		err = db.GetComponentCache().SetComponent(child)
		require.NoError(t, err)

		children, err := db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Equal(t, group, *children[0].Group)

		// Test empty group
		emptyGroup := ""
		child.Group = &emptyGroup
		err = db.GetComponentCache().SetComponent(child)
		require.NoError(t, err)

		children, err = db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Nil(t, children[0].Group)

		// Test nil group
		child.Group = nil
		err = db.GetComponentCache().SetComponent(child)
		require.NoError(t, err)

		children, err = db.GetComponentCache().ComponentChildren(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		require.Nil(t, children[0].Group)
	})
}

func TestComponentCache_HealthScore(t *testing.T) {
	t.Run("cache should calculate correct health score", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID
		component := createComponent(uid, nil, WithState(client.ComponentStateRunning), WithKind("Pod"))
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		child1 := createComponent("child1", &uid, WithState(client.ComponentStateRunning), WithKind("Pod"))
		err = db.GetComponentCache().SetComponent(child1)
		require.NoError(t, err)

		child2 := createComponent("child2", &uid, WithState(client.ComponentStateRunning), WithKind("Pod"))
		err = db.GetComponentCache().SetComponent(child2)
		require.NoError(t, err)

		score, err := db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(100), score)

		child3 := createComponent("child3", &uid, WithState(client.ComponentStateFailed), WithKind("Pod"))
		err = db.GetComponentCache().SetComponent(child3)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(75), score)

		child4 := createComponent("child4", &uid, WithState(client.ComponentStateFailed), WithKind("Deployment"))
		err = db.GetComponentCache().SetComponent(child4)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(60), score)

		// Invalid certificate should deduct an additional 10 points.
		child5 := createComponent("child5", &uid, WithState(client.ComponentStateFailed), WithKind("Certificate"))
		err = db.GetComponentCache().SetComponent(child5)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(40), score)

		// Failing resources in kube-system namespace should deduct an additional 20 points.
		child6 := createComponent("child6", &uid, WithState(client.ComponentStateFailed), WithKind("Pod"), WithNamespace("kube-system"))
		err = db.GetComponentCache().SetComponent(child6)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(12), score)

		// Failing persistent volume should deduct an additional 10 points.
		// The score should not go below 0.
		child7 := createComponent("child7", &uid, WithState(client.ComponentStateFailed), WithKind("PersistentVolume"))
		err = db.GetComponentCache().SetComponent(child7)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})

	t.Run("cache should calculate correct health score for components with no children", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID
		component := createComponent(uid, nil, WithState(client.ComponentStateRunning))
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		score, err := db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(100), score)
	})

	t.Run("cache should calculate health score with critical system component failures", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		baseComponent := createComponent(testUID, nil, WithState(client.ComponentStateRunning))
		err := db.GetComponentCache().SetComponent(baseComponent)
		require.NoError(t, err)

		runningPod := createComponent("running-pod", nil, WithState(client.ComponentStateRunning), WithKind("Pod"), WithName("running-pod"))
		err = db.GetComponentCache().SetComponent(runningPod)
		require.NoError(t, err)

		runningDeployment := createComponent("running-deployment", nil, WithState(client.ComponentStateRunning), WithKind("Deployment"), WithName("running-deployment"))
		err = db.GetComponentCache().SetComponent(runningDeployment)
		require.NoError(t, err)

		runningService := createComponent("running-service", nil, WithState(client.ComponentStateRunning), WithKind("Service"), WithName("running-service"))
		err = db.GetComponentCache().SetComponent(runningService)
		require.NoError(t, err)

		// Test CoreDNS failure (50 point deduction)
		coredns := createComponent("coredns", nil, WithState(client.ComponentStateFailed), WithName("coredns"))
		err = db.GetComponentCache().SetComponent(coredns)
		require.NoError(t, err)

		score, err := db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(30), score)

		// Test AWS CNI failure (additional 50 point deduction)
		awscni := createComponent("aws-cni", nil, WithState(client.ComponentStateFailed), WithName("aws-cni"))
		err = db.GetComponentCache().SetComponent(awscni)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)

		// Test ingress-nginx service failure (would deduct 50 but already at 0)
		ingress := createComponent("ingress", nil, WithState(client.ComponentStateFailed), WithKind("Service"), WithName("ingress-nginx-controller"), WithNamespace("ingress-nginx"))
		err = db.GetComponentCache().SetComponent(ingress)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})

	t.Run("cache should calculate health score with combined resource failures", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		baseComponent := createComponent(testUID, nil, WithState(client.ComponentStateRunning))
		err := db.GetComponentCache().SetComponent(baseComponent)
		require.NoError(t, err)

		// Failed Certificate (10 point deduction)
		cert := createComponent("cert", nil, WithState(client.ComponentStateFailed), WithKind("Certificate"))
		err = db.GetComponentCache().SetComponent(cert)
		require.NoError(t, err)

		score, err := db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(40), score)

		// Failed kube-system resource (20 point deduction)
		kubeSystem := createComponent("kube-system-res", nil, WithState(client.ComponentStateFailed), WithNamespace("kube-system"))
		err = db.GetComponentCache().SetComponent(kubeSystem)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(3), score)

		// Failed PersistentVolume (10 point deduction)
		pv := createComponent("pv", nil, WithState(client.ComponentStateFailed), WithKind("PersistentVolume"))
		err = db.GetComponentCache().SetComponent(pv)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)

		// Failed istio-system resource (50 point deduction)
		istio := createComponent("istio-res", nil, WithState(client.ComponentStateFailed), WithNamespace("istio-system"))
		err = db.GetComponentCache().SetComponent(istio)
		require.NoError(t, err)

		score, err = db.GetComponentCache().HealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(0), score)
	})
}

func createPod(name, uid string, timestamp int64) error {
	return db.GetComponentCache().SetPod(
		name,
		testNamespace,
		uid,
		"",
		testNode,
		timestamp,
		lo.ToPtr(client.ComponentStateFailed),
	)
}

func TestPendingPodsCache(t *testing.T) {
	t.Run("cache should initialize", func(t *testing.T) {
		db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(dbFile))
		defer db.GetComponentCache().Close()
	})

	t.Run("cache should store pods with all required attributes", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		require.NoError(t, createPod("pending-pod-1", "pod-1-uid", hourAgoTimestamp))
		require.NoError(t, createPod("pending-pod-2", "pod-2-uid", hourAgoTimestamp))

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should ignore fresh pending pods that were created within last 5 minutes", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		require.NoError(t, createPod("fresh-pending-pod", "pod-uid", nowTimestamp))
		require.NoError(t, createPod("pending-pod-1", "pod-1-uid", hourAgoTimestamp))
		require.NoError(t, createPod("pending-pod-2", "pod-2-uid", hourAgoTimestamp))

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should delete pod", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		require.NoError(t, createPod("pending-pod-1", "pod-1-uid", hourAgoTimestamp))

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, int64(1), *stats[0].PendingPods)

		err = db.GetComponentCache().DeleteComponent("pod-1-uid")
		require.NoError(t, err)

		stats, err = db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 0)
	})
}

func TestComponentCache_ComponentInsights(t *testing.T) {
	t.Run("should retrieve expected component insights without errors", func(t *testing.T) {
		db.Init()
		defer func() {
			if err := db.GetComponentCache().Close(); err != nil {
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
			err := db.GetComponentCache().SetComponent(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := db.GetComponentCache().ComponentInsights()
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
		db.Init()
		defer func() {
			if err := db.GetComponentCache().Close(); err != nil {
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
			err := db.GetComponentCache().SetComponent(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := db.GetComponentCache().ComponentInsights()
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
		db.Init()
		defer func() {
			if err := db.GetComponentCache().Close(); err != nil {
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
			err := db.GetComponentCache().SetComponent(tc)
			require.NoError(t, err, "Failed to add component %s to cache", tc.UID)
		}

		// Get component insights
		insights, err := db.GetComponentCache().ComponentInsights()
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
		db.Init()
		defer func() {
			if err := db.GetComponentCache().Close(); err != nil {
				t.Fatalf("Failed to close component cache: %v", err)
			}
		}()

		// Get component insights from empty cache
		insights, err := db.GetComponentCache().ComponentInsights()
		require.NoError(t, err, "Failed to get component insights from empty cache")

		require.Nil(t, insights, "Expected non-nil insights object from empty cache")
	})
}
