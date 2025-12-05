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
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/pluralsh/deployment-operator/internal/utils"
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

func (in *DatabaseStore) GetResourceHealth(resources []unstructured.Unstructured) (hasPendingResources, hasFailedResources bool, err error) {
	if len(resources) == 0 {
		return false, false, nil
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
		END as has_pending_resources,
		CASE 
			WHEN COUNT(*) = 0 THEN 0
			WHEN COUNT(CASE WHEN health = 2 THEN 1 END) > 0 THEN 1
			ELSE 0
		END as has_failed_resources
		FROM component 
		WHERE applied = 1 AND ("group", version, kind, namespace, name) IN (
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

	err = sqlitex.ExecuteTransient(conn, sb.String(), &sqlitex.ExecOptions{
		Args: args,
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hasPendingResources = stmt.ColumnBool(0)
			hasFailedResources = stmt.ColumnBool(1)
			return nil
		},
	})
	if err != nil {
		return false, false, fmt.Errorf("failed to check resource health: %w", err)
	}

	return hasPendingResources, hasFailedResources, nil
}

func (in *DatabaseStore) SaveComponent(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	status := common.ToStatus(&obj)
	state := NewComponentState(status)
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

	serverSHA, err := utils.HashResource(obj)
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to calculate resource SHA", "name", obj.GetName(), "namespace", obj.GetNamespace(), "gvk", gvk.String())
		return err
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	in.maybeSaveHookComponent(conn, obj, lo.FromPtr(status), serviceID)

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
			smcommon.GetDeletePhase(obj),
			serverSHA,
			true,
		},
	})
}

func (in *DatabaseStore) SaveComponents(objects []unstructured.Unstructured, applied *bool) error {
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
	if applied != nil {
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
		  delete_phase,
		  server_sha,
		  applied
		) VALUES `)
	} else {
		sb.WriteString(`
		INSERT INTO component (
		  uid,
		  parent_uid,
		  "group",
		  version,
		  kind,
		  namespace,
		  name,
		  node,
		  created_at,
		  service_id,
		  delete_phase,
		  server_sha
		) VALUES `)
	}

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

		serverSHA, err := utils.HashResource(obj)
		if err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to calculate resource SHA", "name", obj.GetName(), "namespace", obj.GetNamespace(), "gvk", gvk.String())
			continue
		}

		if applied != nil {
			valueStrings = append(valueStrings, fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s',%d,'%s',%d,'%s','%s','%s', '%d')",
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
				smcommon.GetDeletePhase(obj),
				serverSHA,
				applied,
			))
		} else {
			valueStrings = append(valueStrings, fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s', '%s', %d, '%s','%s','%s')",
				obj.GetUID(),
				lo.FromPtr(ownerRef),
				gvk.Group,
				gvk.Version,
				gvk.Kind,
				obj.GetNamespace(),
				obj.GetName(),
				nodeName,
				obj.GetCreationTimestamp().Unix(),
				serviceID,
				smcommon.GetDeletePhase(obj),
				serverSHA,
			))
		}
	}

	if len(valueStrings) == 0 {
		return nil // Nothing to insert
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(` ON CONFLICT("group", version, kind, namespace, name) DO UPDATE SET
	  uid = excluded.uid,
	  parent_uid = excluded.parent_uid,
	  node = excluded.node,
	  created_at = excluded.created_at,
	  service_id = excluded.service_id,
      delete_phase = excluded.delete_phase,
	  server_sha = excluded.server_sha
	`)

	if applied != nil {
		sb.WriteString(`, health = excluded.health, applied = excluded.applied`)
	}

	in.maybeSaveHookComponents(conn, objects)

	return sqlitex.Execute(conn, sb.String(), nil)
}

func (in *DatabaseStore) SyncServiceComponents(serviceID string, resources []unstructured.Unstructured) error {
	// Get the list of all components for the service from the store.
	components, err := in.GetServiceComponents(serviceID, false)
	if err != nil {
		return err
	}

	// Create a set of keys for an easy lookup.
	resourceKeys := containers.NewSet[smcommon.Key]()
	for _, resource := range resources {
		resourceKeys.Add(smcommon.NewKeyFromUnstructured(resource))
	}

	// Check if all components that were not applied yet are still present in the resources.
	// Those that are not present anymore should be deleted from the store.
	// TODO: Use single delete query to minimize number of database calls.
	for _, component := range components {
		if !resourceKeys.Has(component.Key()) {
			if err = in.DeleteComponent(component.StoreKey()); err != nil {
				return err
			}
		}
	}

	// Save resources to the store.
	return in.SaveComponents(resources, nil)
}

func (in *DatabaseStore) SetServiceChildren(serviceID, parentUID string, keys []smcommon.StoreKey) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	defer func() {
		in.pool.Put(conn)
		klog.V(log.LogLevelDebug).InfoS("saved components in batch",
			"count", len(keys),
			"duration", time.Since(now),
		)
	}()

	updatedCount := 0
	// Begin transaction
	if err := sqlitex.Execute(conn, "BEGIN TRANSACTION", nil); err != nil {
		return 0, err
	}

	defer func() {
		if err != nil {
			if e := sqlitex.Execute(conn, "ROLLBACK", nil); e != nil {
				klog.ErrorS(e, "failed to rollback transaction")
				return
			}
			return
		}
		if e := sqlitex.Execute(conn, "COMMIT", nil); e != nil {
			klog.ErrorS(e, "failed to commit transaction")
			return
		}
	}()

	stmt := `
		UPDATE component
		SET parent_uid = ?, service_id = ?
		WHERE "group" = ? AND version = ? AND kind = ? AND namespace = ? AND name = ?
		RETURNING 1
`

	for _, key := range keys {
		gvk := key.GVK
		err = sqlitex.Execute(conn, stmt, &sqlitex.ExecOptions{
			Args: []interface{}{
				parentUID,
				serviceID,
				gvk.Group,
				gvk.Version,
				gvk.Kind,
				key.Namespace,
				key.Name,
			},
			ResultFunc: func(_ *sqlite.Stmt) error {
				updatedCount++
				return nil
			},
		})
		if err != nil {
			return updatedCount, err // rollback triggered in defer
		}
	}

	return updatedCount, nil
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
				Manifest:             stmt.ColumnBool(13),
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
				UID:         stmt.ColumnText(0),
				Group:       stmt.ColumnText(1),
				Version:     stmt.ColumnText(2),
				Kind:        stmt.ColumnText(3),
				Namespace:   stmt.ColumnText(4),
				Name:        stmt.ColumnText(5),
				ServerSHA:   stmt.ColumnText(6),
				DeletePhase: stmt.ColumnText(7),
				Manifest:    stmt.ColumnBool(8),
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

func (in *DatabaseStore) GetServiceComponents(serviceID string, onlyApplied bool) (smcommon.Components, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	var sb strings.Builder
	sb.WriteString(`SELECT uid, parent_uid, "group", version, kind, name, namespace, health, delete_phase, manifest
	FROM component WHERE service_id = ?`)
	if onlyApplied {
		sb.WriteString(" AND applied = 1")
	}
	sb.WriteString(` AND (manifest = 1 OR (parent_uid IS NULL OR parent_uid = ''))`)

	result := make([]smcommon.Component, 0)
	err = sqlitex.ExecuteTransient(conn, sb.String(), &sqlitex.ExecOptions{
		Args: []interface{}{serviceID},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, smcommon.Component{
				UID:         stmt.ColumnText(0),
				ParentUID:   stmt.ColumnText(1),
				Group:       stmt.ColumnText(2),
				Version:     stmt.ColumnText(3),
				Kind:        stmt.ColumnText(4),
				Name:        stmt.ColumnText(5),
				Namespace:   stmt.ColumnText(6),
				Status:      ComponentState(stmt.ColumnInt32(7)).String(),
				ServiceID:   serviceID,
				DeletePhase: stmt.ColumnText(8),
				Manifest:    stmt.ColumnBool(9),
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
		Args: []interface{}{uid, uid},
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

	sha, err := utils.HashResource(obj)
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

func (in *DatabaseStore) SyncAppliedResource(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	sha, err := utils.HashResource(obj)
	if err != nil {
		return err
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, `
		UPDATE component 
		SET 
		    apply_sha = ?,
		    server_sha = ?,
			manifest_sha = CASE 
				WHEN transient_manifest_sha IS NULL OR transient_manifest_sha = '' 
				THEN manifest_sha 
				ELSE transient_manifest_sha 
			END,
			transient_manifest_sha = NULL,
			manifest = 1,
			applied = 1
		WHERE "group" = ? 
		  AND version = ? 
		  AND kind = ? 
		  AND namespace = ? 
		  AND name = ?
	`, &sqlitex.ExecOptions{
		Args: []interface{}{
			sha,                                                                 // Apply SHA.
			sha,                                                                 // Server SHA.
			gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters.
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

func (in *DatabaseStore) maybeSaveHookComponent(conn *sqlite.Conn, resource unstructured.Unstructured, state client.ComponentState, serviceID string) {
	if serviceID == "" {
		klog.V(log.LogLevelTrace).InfoS("service ID is empty, skipping saving hook")
		return
	}

	// Health check recognizes resources being deleted as pending.
	// Skipping to avoid recording pending as the last state,
	// which would prevent proper check if a hook has reached its desired state.
	if resource.GetDeletionTimestamp() != nil {
		klog.V(log.LogLevelTrace).InfoS("resource is being deleted, skipping saving hook",
			"resource", resource, "state", state)
		return
	}

	if !smcommon.HasSyncPhaseHookDeletePolicy(resource) {
		klog.V(log.LogLevelTrace).InfoS("resource does not have delete policy, skipping saving hook",
			"resource", resource, "state", state)
		return
	}

	gvk := resource.GroupVersionKind()

	if err := sqlitex.Execute(conn, setHookComponent, &sqlitex.ExecOptions{
		Args: []any{
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			resource.GetNamespace(),
			resource.GetName(),
			resource.GetUID(),
			NewComponentState(&state),
			serviceID,
			smcommon.GetPhaseHookDeletePolicy(resource),
		}}); err != nil {
		klog.V(log.LogLevelMinimal).ErrorS(err, "failed to save hook", "resource", resource)
	}
}

func (in *DatabaseStore) maybeSaveHookComponents(conn *sqlite.Conn, resources []unstructured.Unstructured) {
	count := len(resources)
	if count == 0 {
		return // Nothing to insert
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO hook_component ("group", version, kind, namespace, name, uid, status, service_id, delete_policies) VALUES `)

	valueStrings := make([]string, 0, len(resources))
	for _, resource := range resources {
		serviceID := smcommon.GetOwningInventory(resource)
		state := lo.FromPtr(common.ToStatus(&resource))

		if serviceID == "" {
			klog.V(log.LogLevelTrace).InfoS("service ID is empty, skipping saving hook")
			continue
		}

		// The health check logic marks resources being deleted as pending.
		// This would cause issues when checking if a hook has reached its desired state, so we skip in that case.
		if resource.GetDeletionTimestamp() != nil {
			klog.V(log.LogLevelTrace).InfoS("resource is being deleted, skipping saving hook",
				"resource", resource, "state", state)
			continue
		}

		if !smcommon.HasSyncPhaseHookDeletePolicy(resource) {
			klog.V(log.LogLevelTrace).InfoS("resource does not have delete policy, skipping saving hook",
				"resource", resource, "state", state)
			continue
		}

		gvk := resource.GroupVersionKind()
		valueStrings = append(valueStrings,
			fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s','%s','%s')", gvk.Group, gvk.Version, gvk.Kind,
				resource.GetNamespace(), resource.GetName(), resource.GetUID(), NewComponentState(&state), serviceID,
				smcommon.GetPhaseHookDeletePolicy(resource)))
	}

	if len(valueStrings) == 0 {
		return // Nothing to insert
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(` ON CONFLICT("group", version, kind, namespace, name) DO UPDATE SET
		uid = excluded.uid,
		status = excluded.status,
		service_id = excluded.service_id`)

	if err := sqlitex.Execute(conn, sb.String(), nil); err != nil {
		klog.V(log.LogLevelMinimal).ErrorS(err, "failed to save hook", "resources", resources)
	}
}

func (in *DatabaseStore) GetHookComponents(serviceID string) ([]smcommon.HookComponent, error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer in.pool.Put(conn)

	result := make([]smcommon.HookComponent, 0)
	err = sqlitex.ExecuteTransient(
		conn,
		`SELECT "group", version, kind, namespace, name, uid, status, manifest_sha, delete_policies FROM hook_component WHERE service_id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{serviceID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				result = append(result, smcommon.HookComponent{
					Group:          stmt.ColumnText(0),
					Version:        stmt.ColumnText(1),
					Kind:           stmt.ColumnText(2),
					Namespace:      stmt.ColumnText(3),
					Name:           stmt.ColumnText(4),
					UID:            stmt.ColumnText(5),
					Status:         ComponentState(stmt.ColumnInt32(6)).String(),
					ManifestSHA:    stmt.ColumnText(7),
					ServiceID:      serviceID,
					DeletePolicies: smcommon.SplitHookDeletePolicy(stmt.ColumnText(8)),
				})
				return nil
			},
		})

	return result, err
}

func (in *DatabaseStore) SaveHookComponentWithManifestSHA(manifest, appliedResource unstructured.Unstructured) error {
	serviceID := smcommon.GetOwningInventory(manifest)
	if serviceID == "" {
		klog.V(log.LogLevelTrace).InfoS("service ID is empty, skipping saving hook")
		return nil
	}

	if !smcommon.HasSyncPhaseHookDeletePolicy(manifest) {
		klog.V(log.LogLevelTrace).InfoS("resource does not have delete policy, skipping saving hook")
		return nil
	}

	manifestSHA, err := utils.HashResource(manifest)
	if err != nil {
		return err
	}
	if manifestSHA == "" {
		klog.V(log.LogLevelTrace).InfoS("manifest SHA empty, skipping saving hook")
		return nil
	}

	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	gvk := manifest.GroupVersionKind()

	return sqlitex.Execute(
		conn,
		setHookComponentWithManifestSHA,
		&sqlitex.ExecOptions{
			Args: []any{
				gvk.Group,
				gvk.Version,
				gvk.Kind,
				manifest.GetNamespace(),
				manifest.GetName(),
				appliedResource.GetUID(),
				NewComponentState(common.ToStatus(&appliedResource)),
				manifestSHA,
				serviceID,
			}})
}

func (in *DatabaseStore) ExpireHookComponents(serviceID string) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	return sqlitex.ExecuteTransient(conn, `DELETE FROM hook_component WHERE service_id = ?`,
		&sqlitex.ExecOptions{Args: []any{serviceID}})
}

func (in *DatabaseStore) CheckpointWAL() error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	if err := sqlitex.Execute(conn, "PRAGMA wal_checkpoint(TRUNCATE);", nil); err != nil {
		return fmt.Errorf("wal checkpoint failed: %w", err)
	}

	return nil
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
