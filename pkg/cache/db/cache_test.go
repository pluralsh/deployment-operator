package db_test

import (
	"testing"
	"time"

	"github.com/pluralsh/console/go/client"
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
	timeLayout    = "2006-01-02T15:04:05Z"
)

var (
	nowTimestamp     = time.Now().UTC().Format(timeLayout)
	hourAgoTimestamp = time.Now().Add(-time.Hour).UTC().Format(timeLayout)
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

func TestComponentCache(t *testing.T) {
	t.Run("cache should initialize", func(t *testing.T) {
		db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(dbFile))
		defer db.GetComponentCache().Close()
	})

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

	t.Run("cache should calculate correct health score", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := testUID
		component := createComponent(uid, nil, WithState(client.ComponentStateRunning))
		err := db.GetComponentCache().SetComponent(component)
		require.NoError(t, err)

		child1 := createComponent("child1", &uid, WithState(client.ComponentStateRunning))
		err = db.GetComponentCache().SetComponent(child1)
		require.NoError(t, err)

		child2 := createComponent("child2", &uid, WithState(client.ComponentStateFailed))
		err = db.GetComponentCache().SetComponent(child2)
		require.NoError(t, err)

		score, err := db.GetComponentCache().ClusterHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(66), score)

		child3 := createComponent("child3", &uid, WithState(client.ComponentStateFailed))
		err = db.GetComponentCache().SetComponent(child3)
		require.NoError(t, err)

		score, err = db.GetComponentCache().ClusterHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(50), score)

		child4 := createComponent("child4", &uid, WithState(client.ComponentStateFailed))
		err = db.GetComponentCache().SetComponent(child4)
		require.NoError(t, err)

		score, err = db.GetComponentCache().ClusterHealthScore()
		require.NoError(t, err)
		assert.Equal(t, int64(40), score)
	})
}

func TestPendingPodsCache(t *testing.T) {
	t.Run("cache should initialize", func(t *testing.T) {
		db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(dbFile))
		defer db.GetComponentCache().Close()
	})

	t.Run("cache should store pods with all required attributes", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		err := db.GetComponentCache().SetPod(
			"pending-pod-1",
			testNamespace,
			"pod-1-uid",
			testNode,
			hourAgoTimestamp,
		)
		require.NoError(t, err)

		err = db.GetComponentCache().SetPod(
			"pending-pod-2",
			testNamespace,
			"pod-2-uid",
			testNode,
			hourAgoTimestamp,
		)
		require.NoError(t, err)

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should ignore fresh pending pods that were created within last 5 minutes", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		err := db.GetComponentCache().SetPod(
			"fresh-pending-pod",
			testNamespace,
			"pod-uid",
			testNode,
			nowTimestamp,
		)
		require.NoError(t, err)

		err = db.GetComponentCache().SetPod(
			"pending-pod-1",
			testNamespace,
			"pod-1-uid",
			testNode,
			hourAgoTimestamp,
		)
		require.NoError(t, err)

		err = db.GetComponentCache().SetPod(
			"pending-pod-2",
			testNamespace,
			"pod-2-uid",
			testNode,
			hourAgoTimestamp,
		)
		require.NoError(t, err)

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, testNode, *stats[0].Name)
		assert.Equal(t, int64(2), *stats[0].PendingPods)
	})

	t.Run("cache should delete pod", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		err := db.GetComponentCache().SetPod(
			"pending-pod-1",
			testNamespace,
			"pod-1-uid",
			testNode,
			hourAgoTimestamp,
		)
		require.NoError(t, err)

		stats, err := db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 1)
		assert.Equal(t, int64(1), *stats[0].PendingPods)

		err = db.GetComponentCache().DeletePod("pod-1-uid")
		require.NoError(t, err)

		stats, err = db.GetComponentCache().NodeStatistics()
		require.NoError(t, err)
		require.Len(t, stats, 0)
	})
}
