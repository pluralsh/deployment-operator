package db_test

import (
	"testing"

	"github.com/pluralsh/console/go/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pluralsh/deployment-operator/pkg/cache/db"
)

const (
	dbFile = "/tmp/component-cache.db"
)

func TestComponentCache(t *testing.T) {
	t.Run("cache should initialize", func(t *testing.T) {
		db.Init(db.WithMode(db.CacheModeFile), db.WithFilePath(dbFile))
		defer db.GetComponentCache().Close()
	})

	t.Run("cache should save and return simple parent and child structure", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := "test-uid"
		state := client.ComponentState("Healthy")
		group := "test-group"
		namespace := "test-namespace"

		component := client.ComponentChildAttributes{
			UID:       uid,
			ParentUID: nil,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "test-component",
			State:     &state,
		}
		err := db.GetComponentCache().Set(component)
		require.NoError(t, err)

		childComponent := client.ComponentChildAttributes{
			UID:       "child-uid",
			ParentUID: &uid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(childComponent)
		require.NoError(t, err)

		children, err := db.GetComponentCache().Children(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, "child-uid", children[0].UID)
		assert.Equal(t, uid, *children[0].ParentUID)
	})

	t.Run("cache should save and return multi-level structure", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		state := client.ComponentState("Healthy")
		group := "test-group"
		namespace := "test-namespace"

		// Root
		rootUID := "root-uid"
		component := client.ComponentChildAttributes{
			UID:       rootUID,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "test-component",
			State:     &state,
		}
		err := db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = client.ComponentChildAttributes{
			UID:       uid1,
			ParentUID: &rootUID,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = client.ComponentChildAttributes{
			UID:       uid2,
			ParentUID: &uid1,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = client.ComponentChildAttributes{
			UID:       uid3,
			ParentUID: &uid2,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = client.ComponentChildAttributes{
			UID:       uid4,
			ParentUID: &uid3,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = client.ComponentChildAttributes{
			UID:       uid5,
			ParentUID: &uid4,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().Children(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 4)
	})

	t.Run("cache should save and return multi-level structure", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		state := client.ComponentState("Healthy")
		group := "test-group"
		namespace := "test-namespace"

		// Root
		rootUID := "test-uid"
		component := client.ComponentChildAttributes{
			UID:       rootUID,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "test-component",
			State:     &state,
		}
		err := db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 1
		uid1 := "uid-1"
		component = client.ComponentChildAttributes{
			UID:       uid1,
			ParentUID: &rootUID,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 2
		uid2 := "uid-2"
		component = client.ComponentChildAttributes{
			UID:       uid2,
			ParentUID: &uid1,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 3
		uid3 := "uid-3"
		component = client.ComponentChildAttributes{
			UID:       uid3,
			ParentUID: &uid2,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 4
		uid4 := "uid-4"
		component = client.ComponentChildAttributes{
			UID:       uid4,
			ParentUID: &uid3,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		uid44 := "uid-44"
		component = client.ComponentChildAttributes{
			UID:       uid44,
			ParentUID: &uid3,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		// Level 5
		uid5 := "uid-5"
		component = client.ComponentChildAttributes{
			UID:       uid5,
			ParentUID: &uid4,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().Children(rootUID)
		require.NoError(t, err)
		require.Len(t, children, 5)
	})

	t.Run("cache should support cascade deletion", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := "test-uid"
		state := client.ComponentState("Healthy")
		group := "test-group"
		namespace := "test-namespace"

		component := client.ComponentChildAttributes{
			UID:       uid,
			ParentUID: nil,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "test-component",
			State:     &state,
		}
		err := db.GetComponentCache().Set(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := client.ComponentChildAttributes{
			UID:       childUid,
			ParentUID: &uid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(childComponent)
		require.NoError(t, err)

		grandchildComponent := client.ComponentChildAttributes{
			UID:       "grandchild-uid",
			ParentUID: &childUid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(grandchildComponent)
		require.NoError(t, err)

		children, err := db.GetComponentCache().Children(uid)
		require.NoError(t, err)
		require.Len(t, children, 2)

		err = db.GetComponentCache().Delete(childUid)
		require.NoError(t, err)

		children, err = db.GetComponentCache().Children(uid)
		require.NoError(t, err)
		require.Len(t, children, 0)
	})

	t.Run("cache should support cascade deletion", func(t *testing.T) {
		db.Init()
		defer db.GetComponentCache().Close()

		uid := "test-uid"
		state := client.ComponentState("Healthy")
		group := "test-group"
		namespace := "test-namespace"

		component := client.ComponentChildAttributes{
			UID:       uid,
			ParentUID: nil,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "test-component",
			State:     &state,
		}
		err := db.GetComponentCache().Set(component)
		require.NoError(t, err)

		childUid := "child-uid"
		childComponent := client.ComponentChildAttributes{
			UID:       childUid,
			ParentUID: &uid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(childComponent)
		require.NoError(t, err)

		grandchildComponent := client.ComponentChildAttributes{
			UID:       "grandchild-uid",
			ParentUID: &childUid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(grandchildComponent)
		require.NoError(t, err)

		child2Uid := "child2-uid"
		child2Component := client.ComponentChildAttributes{
			UID:       child2Uid,
			ParentUID: &uid,
			Group:     &group,
			Version:   "v1",
			Kind:      "Test",
			Namespace: &namespace,
			Name:      "child-component",
			State:     &state,
		}
		err = db.GetComponentCache().Set(child2Component)
		require.NoError(t, err)

		children, err := db.GetComponentCache().Children(uid)
		require.NoError(t, err)
		require.Len(t, children, 3)

		err = db.GetComponentCache().Delete(childUid)
		require.NoError(t, err)

		children, err = db.GetComponentCache().Children(uid)
		require.NoError(t, err)
		require.Len(t, children, 1)
	})
}
