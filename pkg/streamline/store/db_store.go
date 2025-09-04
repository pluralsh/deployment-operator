package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/api"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
)

const (
	// defaultStorage defines the default storage mode.
	defaultStorage = api.StorageMemory

	// defaultPoolSize defines the default maximum number of concurrent database connections.
	defaultPoolSize = 50
)

// Option represents a function that configures the database store.
type Option func(store *DatabaseStore)

// WithPoolSize sets the maximum number of concurrent connections in the pool.
func WithPoolSize(size int) Option {
	return func(in *DatabaseStore) {
		in.poolSize = size
	}
}

// WithStorage sets the storage mode for the cache.
func WithStorage(storage api.Storage) Option {
	return func(in *DatabaseStore) {
		in.storage = storage
	}
}

// WithFilePath sets the path where the cache file will be stored in the file storage mode.
func WithFilePath(path string) Option {
	return func(in *DatabaseStore) {
		in.filePath = path
	}
}

type DatabaseStore struct {
	// TODO: Consider adding it to read and write functions.
	mu sync.Mutex

	// storage options for the database.
	storage api.Storage

	// filePath of the data file. Used only when using file storage.
	filePath string

	// poolSize limits the number of concurrent connections to the database.
	poolSize int

	// pool of connections to the database.
	pool *sqlitex.Pool
}

func NewDatabaseStore(options ...Option) (Store, error) {
	store := &DatabaseStore{
		storage:  defaultStorage,
		poolSize: defaultPoolSize,
	}

	for _, option := range options {
		option(store)
	}

	if err := store.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database store: %w", err)
	}

	return store, nil
}

func (in *DatabaseStore) init() error {
	var connectionString string
	if in.storage == api.StorageFile {
		if len(in.filePath) == 0 {
			tempDir, err := os.MkdirTemp("", "db-store-*")
			if err != nil {
				return err
			}

			in.filePath = filepath.Join(tempDir, "store.db")
		}
		connectionString = "file:" + in.filePath + "?mode=rwc"
		klog.V(log.LogLevelDefault).InfoS("using file storage", "path", in.filePath)
	} else {
		connectionString = string(in.storage)
		klog.V(log.LogLevelDefault).InfoS("using memory storage")
	}

	pool, err := sqlitex.NewPool(connectionString, sqlitex.PoolOptions{PoolSize: in.poolSize})
	if err != nil {
		return err
	}
	in.pool = pool

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteScript(conn, createTables, nil)
}

func (in *DatabaseStore) SaveComponent(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	serviceID, hasServiceID := obj.GetAnnotations()[smcommon.OwningInventoryKey]
	state := NewComponentState(common.ToStatus(&obj))

	var nodeName string
	if gvk.Group == "" && gvk.Kind == common.PodKind {
		if nodeName, _, _ = unstructured.NestedString(obj.Object, "spec", "nodeName"); len(nodeName) == 0 {
			return nil // If the pod is not assigned to a node, we don't need to store it
		}

		if !hasServiceID && state == ComponentStateRunning {
			return nil // If the pod does not belong to any service and is running, we don't need to store it
		}
	}

	ownerRefs := obj.GetOwnerReferences()
	var ownerRef *string
	if len(ownerRefs) > 0 {
		ownerRef = lo.ToPtr(string(ownerRefs[0].UID))
		for _, ref := range ownerRefs {
			if ref.Controller != nil && *ref.Controller {
				ownerRef = lo.ToPtr(string(ref.UID))
				break
			}
		}
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, setComponent, &sqlitex.ExecOptions{
		Args: []interface{}{
			obj.GetUID(),
			lo.FromPtr(ownerRef),
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			obj.GetNamespace(),
			obj.GetName(),
			NewComponentState(common.ToStatus(&obj)),
			nodeName,
			obj.GetCreationTimestamp().Unix(),
			serviceID,
		},
	})
}

func (in *DatabaseStore) SaveComponentAttributes(obj client.ComponentChildAttributes, args ...any) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	if len(args) != 3 {
		args = []any{nil, nil, nil}
	}

	return sqlitex.ExecuteTransient(conn, setComponent, &sqlitex.ExecOptions{
		Args: append([]interface{}{
			obj.UID,
			lo.FromPtr(obj.ParentUID),
			lo.FromPtr(obj.Group),
			obj.Version,
			obj.Kind,
			lo.FromPtr(obj.Namespace),
			obj.Name,
			NewComponentState(obj.State),
		}, args...),
	})
}

func (in *DatabaseStore) GetComponent(obj unstructured.Unstructured) (result *Entry, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	gvk := obj.GroupVersionKind()

	err = sqlitex.ExecuteTransient(conn, getComponent, &sqlitex.ExecOptions{
		Args: []interface{}{obj.GetName(), obj.GetNamespace(), gvk.Group, gvk.Version, gvk.Kind},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = &Entry{
				UID:                  stmt.ColumnText(0),
				Group:                stmt.ColumnText(1),
				Version:              stmt.ColumnText(2),
				Kind:                 stmt.ColumnText(3),
				Namespace:            stmt.ColumnText(4),
				Name:                 stmt.ColumnText(5),
				Status:               ComponentState(stmt.ColumnInt32(6)).String(),
				ParentUID:            stmt.ColumnText(7),
				ManifestSHA:          stmt.ColumnText(8),
				TransientManifestSHA: stmt.ColumnText(9),
				ApplySHA:             stmt.ColumnText(10),
				ServerSHA:            stmt.ColumnText(11),
			}
			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) GetComponentByUID(uid types.UID) (result *client.ComponentChildAttributes, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	err = sqlitex.ExecuteTransient(conn, getComponentByUID, &sqlitex.ExecOptions{
		Args: []interface{}{uid},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = &client.ComponentChildAttributes{
				UID:       stmt.ColumnText(0),
				Group:     lo.EmptyableToPtr(stmt.ColumnText(1)),
				Version:   stmt.ColumnText(2),
				Kind:      stmt.ColumnText(3),
				Namespace: lo.EmptyableToPtr(stmt.ColumnText(4)),
				Name:      stmt.ColumnText(5),
				State:     ComponentState(stmt.ColumnInt32(6)).Attribute(),
				ParentUID: lo.EmptyableToPtr(stmt.ColumnText(7)),
			}
			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) DeleteComponent(uid types.UID) error {
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

func (in *DatabaseStore) GetServiceComponents(serviceID string) ([]Entry, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	result := make([]Entry, 0)
	err = sqlitex.ExecuteTransient(conn, getComponentsByServiceID, &sqlitex.ExecOptions{
		Args: []interface{}{serviceID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, Entry{
				UID:       stmt.ColumnText(0),
				ParentUID: stmt.ColumnText(1),
				Group:     stmt.ColumnText(2),
				Version:   stmt.ColumnText(3),
				Kind:      stmt.ColumnText(4),
				Name:      stmt.ColumnText(5),
				Namespace: stmt.ColumnText(6),
				Status:    ComponentState(stmt.ColumnInt32(7)).String(),
				ServiceID: serviceID,
			})
			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) GetComponentChildren(uid string) (result []client.ComponentChildAttributes, err error) {
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
				State:     ComponentState(stmt.ColumnInt32(6)).Attribute(),
				ParentUID: lo.EmptyableToPtr(stmt.ColumnText(7)),
			})
			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) GetComponentInsights() (result []client.ClusterInsightComponentAttributes, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	err = sqlitex.ExecuteTransient(conn, failedComponents, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			name := stmt.ColumnText(5)
			namespace := stmt.ColumnText(4)
			kind := stmt.ColumnText(3)
			result = append(result, client.ClusterInsightComponentAttributes{
				Group:     lo.ToPtr(stmt.ColumnText(1)),
				Version:   stmt.ColumnText(2),
				Kind:      kind,
				Namespace: lo.EmptyableToPtr(namespace),
				Name:      name,
				Priority:  InsightComponentPriority(name, namespace, kind),
			})
			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) GetComponentCounts() (nodeCount, namespaceCount int64, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return
	}
	defer in.pool.Put(conn)

	err = sqlitex.ExecuteTransient(conn, serverCounts, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			nodeCount = stmt.ColumnInt64(0)
			namespaceCount = stmt.ColumnInt64(1)
			return nil
		},
	})
	return
}

func (in *DatabaseStore) GetNodeStatistics() ([]*client.NodeStatisticAttributes, error) {
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
				Health:      NodeStatisticHealth(pendingPods),
			})
			return nil
		},
	})
	return result, err
}

func (in *DatabaseStore) GetHealthScore() (int64, error) {
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

func (in *DatabaseStore) UpdateComponentSHA(obj unstructured.Unstructured, shaType SHAType) error {
	gvk := obj.GroupVersionKind()

	sha, err := HashResource(obj)
	if err != nil {
		return err
	}

	var column string
	switch shaType {
	case ManifestSHA:
		column = "manifest_sha"
	case TransientManifestSHA:
		column = "transient_manifest_sha"
	case ApplySHA:
		column = "apply_sha"
	case ServerSHA:
		column = "server_sha"
	default:
		return fmt.Errorf("unsupported SHAType: %v", shaType)
	}

	query := fmt.Sprintf(`
		UPDATE component
		SET %s = ?
		WHERE "group" = ? AND version = ? AND kind = ? AND namespace = ? AND name = ?`,
		column,
	)

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []interface{}{
			sha,       // SET value
			gvk.Group, // WHERE clause
			gvk.Version,
			gvk.Kind,
			obj.GetNamespace(),
			obj.GetName(),
		},
	})
}

func (in *DatabaseStore) CommitTransientSHA(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, commitTransientSHA, &sqlitex.ExecOptions{
		Args: []interface{}{
			gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters
		},
	})
}

func (in *DatabaseStore) ExpireSHA(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, expireSHA, &sqlitex.ExecOptions{
		Args: []interface{}{
			gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters
		},
	})
}

func (in *DatabaseStore) Expire(serviceID string) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, expire, &sqlitex.ExecOptions{
		Args: []interface{}{serviceID},
	})
}

func (in *DatabaseStore) Shutdown() error {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.pool != nil {
		if err := in.pool.Close(); err != nil {
			return err
		}
	}

	if in.storage == api.StorageFile && len(in.filePath) > 0 {
		if err := os.Remove(in.filePath); err != nil {
			return err
		}
	}

	return nil
}

func (in *DatabaseStore) ExpireOlderThan(ttl time.Duration) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	cutoff := time.Now().Add(-ttl).Unix()

	return sqlitex.ExecuteTransient(conn, expireOlderThan, &sqlitex.ExecOptions{
		Args: []interface{}{cutoff},
	})
}
