package db_test

import (
	"testing"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/cache/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewComponentCache(t *testing.T) {
	t.Run("default initialization", func(t *testing.T) {
		_, err := db.NewComponentCache(db.WithMode(db.CacheModeMemory))
		require.NoError(t, err)
	})
}

func TestComponentCache_Set_Children(t *testing.T) {
	cache, err := db.NewComponentCache()
	require.NoError(t, err)

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

	err = cache.Set(component)
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

	err = cache.Set(childComponent)
	require.NoError(t, err)

	children, err := cache.Children(uid)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, "child-uid", children[0].UID)
	assert.Equal(t, uid, *children[0].ParentUID)
}
