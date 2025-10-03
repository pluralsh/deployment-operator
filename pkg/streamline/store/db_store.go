package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	defaultPoolSize = 10
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
		klog.V(log.LogLevelDefault).InfoS("using in-memory storage")
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

func (in *DatabaseStore) GetResourceHealth(resources []unstructured.Unstructured) (pending, failed bool, err error) {
	if len(resources) == 0 {
		return false, false, nil // Empty list is considered healthy
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return false, false, fmt.Errorf("failed to get database connection: %w", err)
	}
	defer in.pool.Put(conn)

	// Build dynamic query with placeholders for each resource
	var sb strings.Builder
	sb.WriteString(`
		SELECT
		CASE 
			WHEN COUNT(*) = 0 THEN 0
			WHEN COUNT(CASE WHEN health IN (1,3) THEN 1 END) > 0 THEN 1
			ELSE 0
		END as pending,
		CASE 
			WHEN COUNT(*) = 0 THEN 0
			WHEN COUNT(CASE WHEN health = 2 THEN 1 END) > 0 THEN 1
			ELSE 0
		END as failed,
		COUNT(*) as resource_count
		FROM component 
		WHERE ("group", version, kind, namespace, name) IN (
	`)

	// Build VALUES clause with placeholders
	valueStrings := make([]string, 0, len(resources))
	args := make([]interface{}, 0, len(resources)*5)

	for _, resource := range resources {
		gvk := resource.GroupVersionKind()
		valueStrings = append(valueStrings, "(?,?,?,?,?)")
		args = append(args,
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			resource.GetNamespace(),
			resource.GetName(),
		)
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(")")

	var resourceCount int
	err = sqlitex.ExecuteTransient(conn, sb.String(), &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			pending = stmt.ColumnBool(0)
			failed = stmt.ColumnBool(1)
			resourceCount = stmt.ColumnInt(2)
			return nil
		},
	})

	if err != nil {
		return false, false, fmt.Errorf("failed to check resource health: %w", err)
	}

	pending = pending || resourceCount != len(resources)
	return pending, failed, nil
}

func (in *DatabaseStore) HasSomeResources(resources []unstructured.Unstructured) (bool, error) {
	if len(resources) == 0 {
		return true, nil
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return false, fmt.Errorf("failed to get database connection: %w", err)
	}
	defer in.pool.Put(conn)

	// Build dynamic query with placeholders for each resource
	var sb strings.Builder
	sb.WriteString(`SELECT COUNT(*) FROM component WHERE ("group", version, kind, namespace, name) IN (`)

	// Build VALUES clause with placeholders
	valueStrings := make([]string, 0, len(resources))
	args := make([]interface{}, 0, len(resources)*5)

	for _, resource := range resources {
		gvk := resource.GroupVersionKind()
		valueStrings = append(valueStrings, "(?,?,?,?,?)")
		args = append(args,
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			resource.GetNamespace(),
			resource.GetName(),
		)
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(")")

	var resourceCount int
	err = sqlitex.ExecuteTransient(conn, sb.String(), &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			resourceCount = stmt.ColumnInt(0)
			return nil
		},
	})

	if err != nil {
		return false, fmt.Errorf("failed to check resource existence: %w", err)
	}

	return resourceCount > 0, nil
}

func (in *DatabaseStore) SaveComponent(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	state := NewComponentState(common.ToStatus(&obj))
	serviceID := smcommon.GetOwningInventory(obj)

	var nodeName string
	if gvk.Group == "" && gvk.Kind == common.PodKind {
		if nodeName, _, _ = unstructured.NestedString(obj.Object, "spec", "nodeName"); len(nodeName) == 0 {
			return nil // If the pod is not assigned to a node, we don't need to store it
		}

		if serviceID == "" && state == ComponentStateRunning {
			if err := in.DeleteComponent(smcommon.NewStoreKeyFromUnstructured(obj)); err != nil {
				klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete pod", "uid", obj.GetUID())
			}
			klog.V(log.LogLevelDebug).InfoS("skipping pod save", "name", obj.GetName(), "namespace", obj.GetNamespace())
			return nil // If the pod does not belong to any service and is running, we don't need to store it
		}
	}

	var ownerRef *string
	if ownerRefs := obj.GetOwnerReferences(); len(ownerRefs) > 0 {
		ownerRef = lo.ToPtr(string(ownerRefs[0].UID))
		for _, ref := range ownerRefs {
			if ref.Controller != nil && *ref.Controller {
				ownerRef = lo.ToPtr(string(ref.UID))
				break
			}
		}
	}

	serverSHA, err := HashResource(obj)
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to calculate resource SHA", "name", obj.GetName(), "namespace", obj.GetNamespace(), "gvk", gvk.String())
		return err
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, setComponentWithSHA, &sqlitex.ExecOptions{
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
			smcommon.GetSyncPhase(obj),
			serverSHA,
		},
	})
}

func (in *DatabaseStore) SaveComponents(objects []unstructured.Unstructured) error {
	if len(objects) == 0 {
		return nil
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	now := time.Now()
	defer func() {
		in.pool.Put(conn)
		klog.V(log.LogLevelDebug).InfoS("saved components in batch",
			"count", len(objects),
			"duration", time.Since(now),
		)
	}()

	var sb strings.Builder
	sb.WriteString(`
		INSERT INTO component (
		  uid,
		  parent_uid,
		  "group",
		  version,
		  kind,
		  namespace,
		  name,
		  health,
		  node,
		  created_at,
		  service_id,
		  sync_phase,
		  server_sha
		) VALUES `)

	valueStrings := make([]string, 0, len(objects))

	for _, obj := range objects {
		var nodeName string
		gvk := obj.GroupVersionKind()
		serviceID := smcommon.GetOwningInventory(obj)
		state := NewComponentState(common.ToStatus(&obj))

		if gvk.Group == "" && gvk.Kind == common.PodKind {
			if nodeName, _, _ = unstructured.NestedString(obj.Object, "spec", "nodeName"); len(nodeName) == 0 {
				continue // If the pod is not assigned to a node, we don't need to store it
			}

			if serviceID == "" && state == ComponentStateRunning {
				if err := in.deleteComponent(conn, smcommon.NewStoreKeyFromUnstructured(obj)); err != nil {
					klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete pod", "uid", obj.GetUID())
				}
				klog.V(log.LogLevelDebug).InfoS("skipping pod save", "name", obj.GetName(), "namespace", obj.GetNamespace())
				continue // If the pod does not belong to any service and is running, we don't need to store it
			}
		}

		var ownerRef *string
		if ownerRefs := obj.GetOwnerReferences(); len(ownerRefs) > 0 {
			ownerRef = lo.ToPtr(string(ownerRefs[0].UID))
			for _, ref := range ownerRefs {
				if ref.Controller != nil && *ref.Controller {
					ownerRef = lo.ToPtr(string(ref.UID))
					break
				}
			}
		}

		serverSHA, err := HashResource(obj)
		if err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to calculate resource SHA", "name", obj.GetName(), "namespace", obj.GetNamespace(), "gvk", gvk.String())
			continue
		}

		valueStrings = append(valueStrings, fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s',%d,'%s',%d,'%s','%s','%s')",
			obj.GetUID(),
			lo.FromPtr(ownerRef),
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			obj.GetNamespace(),
			obj.GetName(),
			int(state),
			nodeName,
			obj.GetCreationTimestamp().Unix(),
			serviceID,
			smcommon.GetSyncPhase(obj),
			serverSHA,
		))
	}

	if len(valueStrings) == 0 {
		return nil // Nothing to insert
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(` ON CONFLICT("group", version, kind, namespace, name) DO UPDATE SET
	  uid = excluded.uid,
	  parent_uid = excluded.parent_uid,
	  health = excluded.health,
	  node = excluded.node,
	  created_at = excluded.created_at,
	  service_id = excluded.service_id,
      sync_phase = excluded.sync_phase,
	  server_sha = excluded.server_sha
	`)

	return sqlitex.Execute(conn, sb.String(), nil)
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

func (in *DatabaseStore) GetComponent(obj unstructured.Unstructured) (result *smcommon.Component, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	gvk := obj.GroupVersionKind()

	err = sqlitex.ExecuteTransient(conn, getComponent, &sqlitex.ExecOptions{
		Args: []interface{}{obj.GetName(), obj.GetNamespace(), gvk.Group, gvk.Version, gvk.Kind},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = &smcommon.Component{
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
				ServiceID:            stmt.ColumnText(12),
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

func (in *DatabaseStore) GetComponentsByGVK(gvk schema.GroupVersionKind) (result []smcommon.Component, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}
	defer in.pool.Put(conn)

	err = sqlitex.ExecuteTransient(conn, getComponentsByGVK, &sqlitex.ExecOptions{
		Args: []interface{}{gvk.Group, gvk.Version, gvk.Kind},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, smcommon.Component{
				UID:       stmt.ColumnText(0),
				Group:     stmt.ColumnText(1),
				Version:   stmt.ColumnText(2),
				Kind:      stmt.ColumnText(3),
				Namespace: stmt.ColumnText(4),
				Name:      stmt.ColumnText(5),
				ServerSHA: stmt.ColumnText(6),
				SyncPhase: stmt.ColumnText(7),
			})

			return nil
		},
	})

	return result, err
}

func (in *DatabaseStore) DeleteComponent(key smcommon.StoreKey) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return in.deleteComponent(conn, key)
}

func (in *DatabaseStore) deleteComponent(conn *sqlite.Conn, key smcommon.StoreKey) error {
	return sqlitex.ExecuteTransient(conn, `DELETE FROM component WHERE "group" = ? AND version = ? AND kind = ? AND namespace = ? AND name = ?`,
		&sqlitex.ExecOptions{Args: []any{key.GVK.Group, key.GVK.Version, key.GVK.Kind, key.Namespace, key.Name}})
}

func (in *DatabaseStore) DeleteComponents(group, version, kind string) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(
		conn, `DELETE FROM component WHERE "group" = ? AND version = ? AND kind = ?`,
		&sqlitex.ExecOptions{Args: []any{group, version, kind}})
}

func (in *DatabaseStore) GetServiceComponents(serviceID string) ([]smcommon.Component, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	result := make([]smcommon.Component, 0)
	err = sqlitex.ExecuteTransient(conn, getComponentsByServiceID, &sqlitex.ExecOptions{
		Args: []interface{}{serviceID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, smcommon.Component{
				UID:       stmt.ColumnText(0),
				ParentUID: stmt.ColumnText(1),
				Group:     stmt.ColumnText(2),
				Version:   stmt.ColumnText(3),
				Kind:      stmt.ColumnText(4),
				Name:      stmt.ColumnText(5),
				Namespace: stmt.ColumnText(6),
				Status:    ComponentState(stmt.ColumnInt32(7)).String(),
				ServiceID: serviceID,
				SyncPhase: stmt.ColumnText(8),
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

func (in *DatabaseStore) SaveCleanupCandidates(serviceID string, resources []unstructured.Unstructured) error {
	if serviceID == "" {
		return fmt.Errorf("service ID must be provided")
	}

	l := len(resources)
	if l == 0 {
		return nil
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	var sb strings.Builder
	sb.WriteString(`
		INSERT INTO cleanup_candidate (
		  "group",
		  version,
		  kind,
		  namespace,
		  name,
		  service_id
		) VALUES `)

	for i, r := range resources {
		gvk := r.GroupVersionKind()
		sb.WriteString(fmt.Sprintf("('%s','%s','%s','%s','%s','%s')",
			gvk.Group, gvk.Version, gvk.Kind, r.GetNamespace(), r.GetName(), serviceID))
		if i < l-1 {
			sb.WriteString(",")
		}
	}

	sb.WriteString(` ON CONFLICT("group", version, kind, namespace, name) DO UPDATE SET service_id = excluded.service_id`)

	return sqlitex.Execute(conn, sb.String(), nil)
}

func (in *DatabaseStore) GetCleanupCandidates(serviceID string) ([]smcommon.CleanupCandidate, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	result := make([]smcommon.CleanupCandidate, 0)
	err = sqlitex.ExecuteTransient(
		conn,
		`SELECT "group", version, kind, namespace, name FROM cleanup_candidate WHERE service_id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{serviceID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				result = append(result, smcommon.CleanupCandidate{
					Group:     stmt.ColumnText(0),
					Version:   stmt.ColumnText(1),
					Kind:      stmt.ColumnText(2),
					Namespace: stmt.ColumnText(3),
					Name:      stmt.ColumnText(4),
					ServiceID: serviceID,
				})
				return nil
			},
		})

	return result, err
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
