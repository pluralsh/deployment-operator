// Package db provides a SQLite-based caching mechanism for storing and managing component relationships
// and their attributes in a hierarchical structure. It supports both in-memory and file-based storage modes.
package db

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	// cache holds the singleton instance of ComponentCache
	cache *ComponentCache

	// mutex ensures thread-safe access to the cache instance
	mutex sync.Mutex
)

const (
	// defaultPoolSize defines the default maximum number of concurrent SQLite connections
	defaultPoolSize = 50
	// defaultMode defines the default storage mode (memory-based cache)
	defaultMode = CacheModeMemory
)

// CacheMode defines the storage mode for the component cache
type CacheMode string

const (
	// CacheModeMemory stores the cache in memory using SQLite's in-memory database
	CacheModeMemory CacheMode = "file::memory:?mode=memory&cache=shared"
	// CacheModeFile stores the cache in a file on disk
	CacheModeFile CacheMode = "file"
)

// ComponentCache manages a SQLite connection pool for caching component relationships and attributes
type ComponentCache struct {
	// maximum number of concurrent connections
	poolSize int

	// storage mode (memory or file)
	mode CacheMode

	// path to the cache file (only used in file mode)
	filePath string

	// SQLite connection pool
	pool *sqlitex.Pool
}

// GetComponentCache returns the singleton instance of ComponentCache.
// If the cache hasn't been initialized, it will return nil.
// Use Init() to initialize the cache before calling this function.
func GetComponentCache() *ComponentCache {
	return cache
}

// ComponentChildren retrieves all child components and their descendants (up to 4 levels deep) for a given component UID.
// It returns a slice of ComponentChildAttributes containing information about each child component.
//
// Parameters:
//   - uid: The unique identifier of the parent component
//
// Returns:
//   - []ComponentChildAttributes: A slice containing the child components and their attributes
//   - error: An error if the database operation fails or if the connection cannot be established
func (in *ComponentCache) ComponentChildren(uid string) (result []client.ComponentChildAttributes, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	err = sqlitex.ExecuteTransient(conn, componentChildren, &sqlitex.ExecOptions{
		Args: []interface{}{uid},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, client.ComponentChildAttributes{
				UID:       stmt.ColumnText(0),
				Group:     lo.EmptyableToPtr(stmt.ColumnText(1)),
				Version:   stmt.ColumnText(2),
				Kind:      stmt.ColumnText(3),
				Namespace: lo.EmptyableToPtr(stmt.ColumnText(4)),
				Name:      stmt.ColumnText(5),
				State:     FromComponentState(ComponentState(stmt.ColumnInt32(6))),
				ParentUID: lo.EmptyableToPtr(stmt.ColumnText(7)),
			})
			return nil
		},
	})

	return result, err
}

// SetComponent stores or updates a component's attributes in the cache.
// It takes a ComponentChildAttributes parameter containing the component's data.
// If a component with the same UID exists, it will be updated; otherwise, a new entry is created.
// Returns an error if the database operation fails or if the connection cannot be established.
func (in *ComponentCache) SetComponent(component client.ComponentChildAttributes) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, setComponent, &sqlitex.ExecOptions{
		Args: []interface{}{
			component.UID,
			lo.FromPtr(component.ParentUID),
			lo.FromPtr(component.Group),
			component.Version,
			component.Kind,
			lo.FromPtr(component.Namespace),
			component.Name,
			ToComponentState(component.State),
		},
	})
}

// DeleteComponent removes a component from the cache by its unique identifier.
// It takes a uid string parameter identifying the component to delete.
// Returns an error if the operation fails or if the connection cannot be established.
func (in *ComponentCache) DeleteComponent(uid string) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	query := `DELETE FROM component WHERE uid = ?`
	return sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []any{uid},
	})
}

// HealthScore returns a percentage of healthy components to total components in the cluster.
// The percentage is calculated as the number of healthy components divided by the total number of components.
// Returns an int value between 0 and 100, where 100 indicates all components are healthy.
// Returns an error if the database operation fails or if the connection cannot be established.
func (in *ComponentCache) HealthScore() (int64, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	defer in.pool.Put(conn)

	var ratio int64
	err = sqlitex.ExecuteTransient(conn, clusterHealthScore, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			ratio = stmt.ColumnInt64(0)
			return nil
		},
	})
	return ratio, err
}

// SetPod stores or updates a pod's attributes in the cache.
// It takes pod name, namespace, uid, node name and creation timestamp as parameters.
// If a pod with the same UID exists, it will be updated; otherwise, a new entry is created.
// Returns an error if the database operation fails or if the connection cannot be established.
func (in *ComponentCache) SetPod(name, namespace, uid, node string, createdAt int64) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, setPod, &sqlitex.ExecOptions{
		Args: []interface{}{
			name,
			namespace,
			uid,
			node,
			createdAt,
		},
	})
}

// DeletePod removes a pod from the cache by its unique identifier.
// It takes a uid string parameter identifying the pod to delete.
// Returns an error if the operation fails or if the connection cannot be established.
func (in *ComponentCache) DeletePod(uid string) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	query := `DELETE FROM pod WHERE uid = ?`
	return sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []any{uid},
	})
}

// NodeStatistics returns a list of node statistics including the node name and count of pending pods
// that were created more than 5 minutes ago. Each NodeStatisticAttributes contains the node name and
// the number of pending pods for that node. The health field is currently not implemented.
// Returns an error if the database operation fails or if the connection cannot be established.
func (in *ComponentCache) NodeStatistics() ([]*client.NodeStatisticAttributes, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	result := make([]*client.NodeStatisticAttributes, 0)
	err = sqlitex.ExecuteTransient(conn, nodeStatistics, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			pendingPods := stmt.ColumnInt64(1)
			result = append(result, &client.NodeStatisticAttributes{
				Name:        lo.ToPtr(stmt.ColumnText(0)),
				PendingPods: &pendingPods,
				Health:      nodeHealth(pendingPods),
			})
			return nil
		},
	})
	return result, err
}

func nodeHealth(pendingPods int64) *client.NodeStatisticHealth {
	switch {
	case pendingPods == 0:
		return lo.ToPtr(client.NodeStatisticHealthHealthy)
	case pendingPods <= 3:
		return lo.ToPtr(client.NodeStatisticHealthWarning)
	default:
		return lo.ToPtr(client.NodeStatisticHealthFailed)
	}
}

// Close closes the connection pool and cleans up temporary file if necessary
func (in *ComponentCache) Close() error {
	mutex.Lock()
	defer mutex.Unlock()

	defer func() {
		cache = nil
	}()

	if cache == nil {
		return nil
	}

	if in.pool != nil {
		if err := in.pool.Close(); err != nil {
			return err
		}
	}

	// Clean up temp file if we created one
	if in.mode == CacheModeFile && len(in.filePath) > 0 {
		// Remove the file
		if err := os.Remove(in.filePath); err != nil {
			return err
		}
	}

	return nil
}

func (in *ComponentCache) init() error {
	var connectionString string

	if in.mode == CacheModeFile {
		if len(in.filePath) == 0 {
			tempDir, err := os.MkdirTemp("", "component-cache-*")
			if err != nil {
				return err
			}

			in.filePath = filepath.Join(tempDir, "cache.db")
		}

		connectionString = "file:" + in.filePath + "?mode=rwc"
	} else {
		connectionString = string(in.mode)
	}

	pool, err := sqlitex.NewPool(connectionString, sqlitex.PoolOptions{
		PoolSize: in.poolSize,
	})
	if err != nil {
		return err
	}

	in.pool = pool
	return in.initTables()
}

func (in *ComponentCache) initTables() error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteScript(conn, createTables, nil)
}

// Option represents a function that configures the ComponentCache
type Option func(*ComponentCache)

// WithPoolSize sets the maximum number of concurrent connections in the pool
func WithPoolSize(size int) Option {
	return func(in *ComponentCache) {
		in.poolSize = size
	}
}

// WithMode sets the storage mode for the cache (memory or file)
func WithMode(mode CacheMode) Option {
	return func(in *ComponentCache) {
		in.mode = mode
	}
}

// WithFilePath sets the path where the cache file will be stored (only used in file mode)
func WithFilePath(path string) Option {
	return func(in *ComponentCache) {
		in.filePath = path
	}
}

// Init initializes the component cache with the provided options.
// If the cache is already initialized, it returns nil.
// Default values are used for any options not provided.
func Init(args ...Option) {
	mutex.Lock()
	defer mutex.Unlock()

	if cache != nil {
		return
	}

	cache = &ComponentCache{
		poolSize: defaultPoolSize,
		mode:     defaultMode,
	}

	for _, arg := range args {
		arg(cache)
	}

	if err := cache.init(); err != nil {
		klog.Fatalf("failed to initialize component cache: %v", err)
	}
}
