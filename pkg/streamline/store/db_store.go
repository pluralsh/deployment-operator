package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	_ "github.com/tursodatabase/go-libsql"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

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

	// db is the connection to the database.
	db *sql.DB

	// ctx is an internal context for the store.
	ctx context.Context
}

func NewDatabaseStore(ctx context.Context, options ...Option) (Store, error) {
	store := &DatabaseStore{
		ctx:      ctx,
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

	db, err := sql.Open("libsql", connectionString)
	if err != nil {
		return err
	}
	in.db = db

	if _, err = in.db.Exec(createComponentTable); err != nil {
		return err
	}

	for _, stmt := range strings.Split(createComponentIndexes, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err = in.db.Exec(stmt); err != nil {
			return err
		}
	}

	if _, err = in.db.Exec(createTriggerInsert); err != nil {
		return err
	}

	if _, err = in.db.Exec(createTriggerUpdate); err != nil {
		return err
	}

	if _, err = in.db.Exec(createHookComponentTable); err != nil {
		return err
	}

	for _, stmt := range strings.Split(createHookComponentIndexes, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err = in.db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func (in *DatabaseStore) GetResourceHealth(resources []unstructured.Unstructured) (hasPendingResources, hasFailedResources bool, err error) {
	if len(resources) == 0 {
		return false, false, nil
	}

	// Build dynamic query with placeholders for each resource
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString("CASE WHEN COUNT(*) = 0 THEN 0 WHEN COUNT(CASE WHEN health IN (1,3) THEN 1 END) > 0 THEN 1 ELSE 0 END as has_pending_resources, ")
	sb.WriteString("CASE WHEN COUNT(*) = 0 THEN 0 WHEN COUNT(CASE WHEN health = 2 THEN 1 END) > 0 THEN 1 ELSE 0 END as has_failed_resources ")
	sb.WriteString("FROM component ")
	sb.WriteString(`WHERE applied = 1 AND ("group", version, kind, namespace, name) IN (`)

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

	rows, err := in.db.QueryContext(in.ctx, sb.String(), args...)
	if err != nil {
		return false, false, fmt.Errorf("failed to check resource health: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&hasPendingResources, &hasFailedResources); err != nil {
			return false, false, fmt.Errorf("failed to scan resource health: %w", err)
		}
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

	in.maybeSaveHookComponent(obj, lo.FromPtr(status), serviceID)

	_, err = in.db.ExecContext(in.ctx, setComponentWithSHA,
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
	)
	return err
}

func (in *DatabaseStore) SaveComponents(objects []unstructured.Unstructured) error {
	if len(objects) == 0 {
		return nil
	}

	now := time.Now()
	defer func() {
		klog.V(log.LogLevelDebug).InfoS("saved components in batch",
			"count", len(objects),
			"duration", time.Since(now),
		)
	}()

	var sb strings.Builder
	sb.WriteString(`INSERT INTO component (
		uid, parent_uid, "group", version, kind, namespace, name, health, node, created_at, service_id, delete_phase, server_sha, applied
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
				if err := in.deleteComponent(smcommon.NewStoreKeyFromUnstructured(obj)); err != nil {
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

		valueStrings = append(valueStrings, fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s',%d,'%s',%d,'%s','%s','%s', 1)",
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
      delete_phase = excluded.delete_phase,
	  server_sha = excluded.server_sha,
	  applied = excluded.applied
	`)

	in.maybeSaveHookComponents(objects)

	_, err := in.db.ExecContext(in.ctx, sb.String())
	return err
}

func (in *DatabaseStore) SaveUnsyncedComponents(objects []unstructured.Unstructured) error {
	if len(objects) == 0 {
		return nil
	}

	now := time.Now()
	defer func() {
		klog.V(log.LogLevelDebug).InfoS("saved unsynced components in batch",
			"count", len(objects),
			"duration", time.Since(now),
		)
	}()

	var sb strings.Builder
	sb.WriteString(`INSERT INTO component (
		uid, parent_uid, "group", version, kind, namespace, name, node, service_id, delete_phase
	) VALUES `)

	valueStrings := make([]string, 0, len(objects))

	for _, obj := range objects {
		var nodeName string
		gvk := obj.GroupVersionKind()
		serviceID := smcommon.GetOwningInventory(obj)

		if gvk.Group == "" && gvk.Kind == common.PodKind {
			if nodeName, _, _ = unstructured.NestedString(obj.Object, "spec", "nodeName"); len(nodeName) == 0 {
				continue // If the pod is not assigned to a node, we don't need to store it
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

		valueStrings = append(valueStrings, fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%s', '%s', '%s','%s')",
			obj.GetUID(),
			lo.FromPtr(ownerRef),
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			obj.GetNamespace(),
			obj.GetName(),
			nodeName,
			serviceID,
			smcommon.GetDeletePhase(obj),
		))
	}

	if len(valueStrings) == 0 {
		return nil // Nothing to insert
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(` ON CONFLICT("group", version, kind, namespace, name) DO UPDATE SET
	  uid = excluded.uid,
	  parent_uid = excluded.parent_uid,
	  node = excluded.node,
	  service_id = excluded.service_id,
      delete_phase = excluded.delete_phase
	`)

	_, err := in.db.ExecContext(in.ctx, sb.String())
	return err
}

func (in *DatabaseStore) SaveComponentAttributes(obj client.ComponentChildAttributes, args ...any) error {
	if len(args) != 3 {
		args = []any{nil, nil, nil}
	}

	_, err := in.db.ExecContext(in.ctx, setComponent,
		append([]interface{}{
			obj.UID,
			lo.FromPtr(obj.ParentUID),
			lo.FromPtr(obj.Group),
			obj.Version,
			obj.Kind,
			lo.FromPtr(obj.Namespace),
			obj.Name,
			NewComponentState(obj.State),
			true,
		}, args...)...,
	)
	return err
}

func (in *DatabaseStore) SyncServiceComponents(serviceID string, resources []unstructured.Unstructured) error {
	// Get the list of all components for the service from the store.
	components, err := in.GetServiceComponents(serviceID, false)
	if err != nil {
		return err
	}

	// Create a set of keys for an easy lookup.
	componentKeys := containers.NewSet[smcommon.Key]()
	for _, component := range components {
		componentKeys.Add(component.Key())
	}

	// Create a set of keys for an easy lookup and an array of resources that are not in the store yet.
	// Skip resources that do not have an apply phase, i.e., resources with skip or invalid phase.
	resourceKeys := containers.NewSet[smcommon.Key]()
	resourcesToSave := make([]unstructured.Unstructured, 0, len(resources))
	for _, resource := range resources {
		resourceKey := smcommon.NewKeyFromUnstructured(resource)
		resourceKeys.Add(resourceKey)

		if !componentKeys.Has(resourceKey) && smcommon.HasApplyPhase(resource) {
			resourcesToSave = append(resourcesToSave, resource)
		}
	}

	// Check if all components that were not applied yet are still present in the resources.
	// Those that are not present anymore should be deleted from the store.
	componentsToDelete := containers.NewSet[smcommon.StoreKey]()
	for _, component := range components {
		if !resourceKeys.Has(component.Key()) {
			componentsToDelete.Add(component.StoreKey())
		}
	}

	if err = in.DeleteUnsyncedComponentsByKeys(componentsToDelete); err != nil {
		return err
	}

	// Save resources to the store.
	return in.SaveUnsyncedComponents(resourcesToSave)
}

func (in *DatabaseStore) DeleteUnsyncedComponentsByKeys(objects containers.Set[smcommon.StoreKey]) error {
	if len(objects) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`DELETE FROM component WHERE ("group", version, kind, namespace, name) IN (`)

	valueStrings := make([]string, 0, len(objects))
	args := make([]interface{}, 0, len(objects)*5)

	for obj := range objects {
		valueStrings = append(valueStrings, "(?,?,?,?,?)")
		args = append(args, obj.GVK.Group, obj.GVK.Version, obj.GVK.Kind, obj.Namespace, obj.Name)
	}

	sb.WriteString(strings.Join(valueStrings, ","))
	sb.WriteString(") AND applied = 0")

	_, err := in.db.ExecContext(in.ctx, sb.String(), args...)
	return err
}

func (in *DatabaseStore) GetComponentAttributes(serviceID string, isDeleting bool) ([]client.ComponentAttributes, error) {
	// If service is being deleted we can ignore children and only applied components should be returned.
	// Logic to remove already deleted hook resources from the result is not needed in that case.
	// Service is considered as deleted when all its resources have been removed.
	if isDeleting {
		components, err := in.GetServiceComponents(serviceID, true)
		if err != nil {
			return nil, err
		}

		return components.ComponentAttributes(), nil
	}

	// Fetch components with their children.
	components, err := in.GetServiceComponentsWithChildren(serviceID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get service components: %w", err)
	}

	// Exclude non-existing hook components with a deletion policy that have reached their desired state.
	hooks, err := in.GetHookComponents(serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hook components: %w", err)
	}

	keyToHookComponent := make(map[smcommon.Key]smcommon.HookComponent)
	for _, hook := range hooks {
		keyToHookComponent[hook.StoreKey().Key()] = hook
	}

	attributes := make([]client.ComponentAttributes, 0, len(components))
	for _, component := range components {
		componentKey := smcommon.NewStoreKeyFromComponentAttributes(component).Key()
		if hook, ok := keyToHookComponent[componentKey]; ok && hook.HadDesiredState() {
			continue
		}

		attributes = append(attributes, component)
	}

	return attributes, nil
}

func (in *DatabaseStore) SetServiceChildren(serviceID, parentUID string, keys []smcommon.StoreKey) (int, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	now := time.Now()
	defer func() {
		klog.V(log.LogLevelDebug).InfoS("saved components in batch",
			"count", len(keys),
			"duration", time.Since(now),
		)
	}()

	updatedCount := 0
	// Begin transaction
	tx, err := in.db.BeginTx(in.ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt := `
		UPDATE component
		SET parent_uid = ?, service_id = ?
		WHERE "group" = ? AND version = ? AND kind = ? AND namespace = ? AND name = ?
		RETURNING 1
`

	for _, key := range keys {
		gvk := key.GVK
		var one int
		err := tx.QueryRowContext(in.ctx, stmt,
			parentUID,
			serviceID,
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			key.Namespace,
			key.Name,
		).Scan(&one)

		if err == nil {
			updatedCount++
		} else if err != sql.ErrNoRows {
			return updatedCount, err
		}
	}

	if err := tx.Commit(); err != nil {
		return updatedCount, err
	}

	return updatedCount, nil
}

func (in *DatabaseStore) GetAppliedComponent(obj unstructured.Unstructured) (result *smcommon.Component, err error) {
	gvk := obj.GroupVersionKind()

	row := in.db.QueryRowContext(in.ctx, getAppliedComponent, obj.GetName(), obj.GetNamespace(), gvk.Group, gvk.Version, gvk.Kind)

	var uid, group, version, kind, namespace, name string
	var parentUID, manifestSHA, transientManifestSHA, applySHA, serverSHA, serviceID sql.NullString
	var status sql.NullInt32
	var manifest bool

	err = row.Scan(&uid, &group, &version, &kind, &namespace, &name, &status, &parentUID, &manifestSHA, &transientManifestSHA, &applySHA, &serverSHA, &serviceID, &manifest)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	result = &smcommon.Component{
		UID:                  uid,
		Group:                group,
		Version:              version,
		Kind:                 kind,
		Namespace:            namespace,
		Name:                 name,
		Status:               ComponentState(status.Int32).String(),
		ParentUID:            parentUID.String,
		ManifestSHA:          manifestSHA.String,
		TransientManifestSHA: transientManifestSHA.String,
		ApplySHA:             applySHA.String,
		ServerSHA:            serverSHA.String,
		ServiceID:            serviceID.String,
		Manifest:             manifest,
	}
	return result, nil
}

func (in *DatabaseStore) GetAppliedComponentByUID(uid types.UID) (result *client.ComponentChildAttributes, err error) {
	row := in.db.QueryRowContext(in.ctx, getAppliedComponentByUID, uid)

	var rUID, version, kind, name string
	var group, namespace, parentUID sql.NullString
	var status sql.NullInt32

	err = row.Scan(&rUID, &group, &version, &kind, &namespace, &name, &status, &parentUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	result = &client.ComponentChildAttributes{
		UID:       rUID,
		Group:     lo.EmptyableToPtr(group.String),
		Version:   version,
		Kind:      kind,
		Namespace: lo.EmptyableToPtr(namespace.String),
		Name:      name,
		State:     ComponentState(status.Int32).Attribute(),
		ParentUID: lo.EmptyableToPtr(parentUID.String),
	}
	return result, nil
}

func (in *DatabaseStore) GetAppliedComponentsByGVK(gvk schema.GroupVersionKind) (result []smcommon.Component, err error) {
	rows, err := in.db.QueryContext(in.ctx, getAppliedComponentsByGVK, gvk.Group, gvk.Version, gvk.Kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uid, group, version, kind, namespace, name string
		var serverSHA, deletePhase sql.NullString
		var manifest bool
		if err := rows.Scan(&uid, &group, &version, &kind, &namespace, &name, &serverSHA, &deletePhase, &manifest); err != nil {
			return nil, err
		}
		result = append(result, smcommon.Component{
			UID:         uid,
			Group:       group,
			Version:     version,
			Kind:        kind,
			Namespace:   namespace,
			Name:        name,
			ServerSHA:   serverSHA.String,
			DeletePhase: deletePhase.String,
			Manifest:    manifest,
		})
	}

	return result, rows.Err()
}

func (in *DatabaseStore) DeleteComponent(key smcommon.StoreKey) error {
	return in.deleteComponent(key)
}

func (in *DatabaseStore) deleteComponent(key smcommon.StoreKey) error {
	_, err := in.db.ExecContext(in.ctx, `DELETE FROM component WHERE "group" = ? AND version = ? AND kind = ? AND namespace = ? AND name = ?`,
		key.GVK.Group, key.GVK.Version, key.GVK.Kind, key.Namespace, key.Name)
	return err
}

func (in *DatabaseStore) DeleteComponents(group, version, kind string) error {
	_, err := in.db.ExecContext(in.ctx, `DELETE FROM component WHERE "group" = ? AND version = ? AND kind = ?`,
		group, version, kind)
	return err
}

func (in *DatabaseStore) GetServiceComponents(serviceID string, onlyApplied bool) (smcommon.Components, error) {
	var sb strings.Builder
	sb.WriteString(`SELECT uid, parent_uid, "group", version, kind, name, namespace, health, delete_phase, manifest, applied
	FROM component WHERE service_id = ?`)
	if onlyApplied {
		sb.WriteString(" AND applied = 1")
	}

	// Return only if created from an original manifest set
	// of a service or if it doesn't have any parent.
	// Introduced to filter out components that have copied annotations
	// from parents but are not part of the original manifest set.
	sb.WriteString(` AND (manifest = 1 OR (parent_uid IS NULL OR parent_uid = ''))`)

	rows, err := in.db.QueryContext(in.ctx, sb.String(), serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]smcommon.Component, 0)
	for rows.Next() {
		var uid, group, version, kind, name, namespace string
		var parentUID, deletePhase sql.NullString
		var health sql.NullInt32
		var manifest, applied bool

		if err := rows.Scan(&uid, &parentUID, &group, &version, &kind, &name, &namespace, &health, &deletePhase, &manifest, &applied); err != nil {
			return nil, err
		}

		result = append(result, smcommon.Component{
			UID:         uid,
			ParentUID:   parentUID.String,
			Group:       group,
			Version:     version,
			Kind:        kind,
			Name:        name,
			Namespace:   namespace,
			Status:      lo.Ternary(applied, ComponentState(health.Int32), ComponentStatePending).String(),
			ServiceID:   serviceID,
			DeletePhase: deletePhase.String,
			Manifest:    manifest,
		})
	}

	return result, rows.Err()
}

func (in *DatabaseStore) GetServiceComponentsWithChildren(serviceID string, onlyApplied bool) ([]client.ComponentAttributes, error) {
	var sb strings.Builder
	// service_components CTE selects top-level components belonging to the service.
	// Only returns components that are part of the original manifest or have no parent.
	sb.WriteString(`
		WITH service_components AS (
			SELECT uid, "group", version, kind, name, namespace, health, applied
			FROM component 
			WHERE service_id = ?`)
	if onlyApplied {
		sb.WriteString(" AND applied = 1")
	}
	sb.WriteString(` AND (manifest = 1 OR (parent_uid IS NULL OR parent_uid = ''))
		),`)

	// component_children CTE recursively finds all descendants of service components.
	// root_component_uid tracks which service component each child belongs to.
	// level tracks recursion depth and stops at 4.
	sb.WriteString(`
		component_children AS (
			SELECT 
				sc.uid as root_component_uid,
				child.uid,
				child."group",
				child.version,
				child.kind,
				child.namespace,
				child.name,
				child.health,
                child.applied,
				child.parent_uid,
				1 as level
			FROM service_components sc
			JOIN component child ON child.parent_uid = sc.uid AND child.parent_uid != ''
			
			UNION ALL
			
			SELECT 
				cc.root_component_uid,
				c.uid,
				c."group",
				c.version,
				c.kind,
				c.namespace,
				c.name,
				c.health,
				c.applied,
				c.parent_uid,
				cc.level + 1
			FROM component_children cc
			JOIN component c ON c.parent_uid = cc.uid AND c.parent_uid != ''
			WHERE cc.level < 4
		)`)

	// Final SELECT combines components and children into one result set.
	// row_type distinguishes between service components and their children.
	// root_component_uid is used to attach children to the correct parent component.
	sb.WriteString(`
		SELECT 
			'component' as row_type,
			sc.uid,
			sc."group",
			sc.version,
			sc.kind,
			sc.name,
			sc.namespace,
			sc.health,
			sc.applied,
			'' as parent_uid,
			'' as root_component_uid
		FROM service_components sc
		UNION ALL
		SELECT 
			'child' as row_type,
			cc.uid,
			cc."group",
			cc.version,
			cc.kind,
			cc.name,
			cc.namespace,
			cc.health,
			cc.applied,
			cc.parent_uid,
			cc.root_component_uid
		FROM component_children cc
		WHERE cc.root_component_uid != ''
	`)

	rows, err := in.db.QueryContext(in.ctx, sb.String(), serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service components with children: %w", err)
	}
	defer rows.Close()

	componentMap := make(map[string]*client.ComponentAttributes)      // Map to store component attributes by store key
	childrenMap := make(map[string][]client.ComponentChildAttributes) // Map to store children by root component UID

	for rows.Next() {
		var rowType, uid, version, kind, name string
		var group, namespace, parentUID, rootComponentUID sql.NullString
		var health sql.NullInt32
		var applied bool

		if err := rows.Scan(&rowType, &uid, &group, &version, &kind, &name, &namespace, &health, &applied, &parentUID, &rootComponentUID); err != nil {
			return nil, fmt.Errorf("failed to scan service components with children: %w", err)
		}

		state := lo.Ternary(applied, ComponentState(health.Int32), ComponentStatePending).Attribute()
		key := smcommon.NewKey(group.String, version, kind, namespace.String, name)

		if rowType == "component" {
			componentMap[key.String()] = &client.ComponentAttributes{
				UID:       lo.ToPtr(uid),
				Synced:    true,
				Group:     group.String,
				Version:   version,
				Kind:      kind,
				Name:      name,
				Namespace: namespace.String,
				State:     state,
			}
		} else {
			// Child row
			child := client.ComponentChildAttributes{
				UID:       uid,
				Group:     lo.EmptyableToPtr(group.String),
				Version:   version,
				Kind:      kind,
				Namespace: lo.EmptyableToPtr(namespace.String),
				Name:      name,
				State:     state,
				ParentUID: lo.EmptyableToPtr(parentUID.String),
			}
			childrenMap[rootComponentUID.String] = append(childrenMap[rootComponentUID.String], child)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate service components with children: %w", err)
	}

	// Build result with children attached
	result := make([]client.ComponentAttributes, 0, len(componentMap))
	for _, attr := range componentMap {
		attr.Children = make([]*client.ComponentChildAttributes, 0)
		if attr.UID != nil && *attr.UID != "" {
			if children, ok := childrenMap[*attr.UID]; ok {
				attr.Children = lo.ToSlicePtr(children)
			}
		}
		result = append(result, *attr)
	}

	return result, nil
}

func (in *DatabaseStore) GetComponentInsights() (result []client.ClusterInsightComponentAttributes, err error) {
	rows, err := in.db.QueryContext(in.ctx, failedComponents)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var uid, version, kind, name string
		var group, namespace sql.NullString
		if err := rows.Scan(&uid, &group, &version, &kind, &namespace, &name); err != nil {
			return nil, err
		}
		result = append(result, client.ClusterInsightComponentAttributes{
			Group:     lo.ToPtr(group.String),
			Version:   version,
			Kind:      kind,
			Namespace: lo.EmptyableToPtr(namespace.String),
			Name:      name,
			Priority:  InsightComponentPriority(name, namespace.String, kind),
		})
	}

	return result, rows.Err()
}

func (in *DatabaseStore) GetComponentCounts() (nodeCount, namespaceCount int64, err error) {
	row := in.db.QueryRowContext(in.ctx, serverCounts)
	err = row.Scan(&nodeCount, &namespaceCount)
	return
}

func (in *DatabaseStore) GetNodeStatistics() ([]*client.NodeStatisticAttributes, error) {
	rows, err := in.db.QueryContext(in.ctx, nodeStatistics)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*client.NodeStatisticAttributes, 0)
	for rows.Next() {
		var node string
		var pendingPods int64
		if err := rows.Scan(&node, &pendingPods); err != nil {
			return nil, err
		}
		result = append(result, &client.NodeStatisticAttributes{
			Name:        lo.ToPtr(node),
			PendingPods: &pendingPods,
			Health:      NodeStatisticHealth(pendingPods),
		})
	}
	return result, rows.Err()
}

func (in *DatabaseStore) GetHealthScore() (int64, error) {
	var ratio int64
	err := in.db.QueryRowContext(in.ctx, clusterHealthScore).Scan(&ratio)
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

	_, err = in.db.ExecContext(in.ctx, query,
		sha,       // SET value
		gvk.Group, // WHERE clause
		gvk.Version,
		gvk.Kind,
		obj.GetNamespace(),
		obj.GetName(),
	)
	return err
}

func (in *DatabaseStore) CommitTransientSHA(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	_, err := in.db.ExecContext(in.ctx, commitTransientSHA,
		gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters
	)
	return err
}

func (in *DatabaseStore) SyncAppliedResource(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	sha, err := utils.HashResource(obj)
	if err != nil {
		return err
	}

	_, err = in.db.ExecContext(in.ctx, `
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
	`,
		sha,                                                                 // Apply SHA.
		sha,                                                                 // Server SHA.
		gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters.
	)
	return err
}

func (in *DatabaseStore) ExpireSHA(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	_, err := in.db.ExecContext(in.ctx, expireSHA,
		gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(), // WHERE clause parameters
	)
	return err
}

func (in *DatabaseStore) SetComponentUnsynced(obj unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	_, err := in.db.ExecContext(in.ctx, setComponentUnsynced,
		gvk.Group, gvk.Version, gvk.Kind, obj.GetNamespace(), obj.GetName(),
	)
	return err
}

func (in *DatabaseStore) Expire(serviceID string) error {
	_, err := in.db.ExecContext(in.ctx, expire, serviceID)
	return err
}

func (in *DatabaseStore) ExpireOlderThan(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl).Unix()

	_, err := in.db.ExecContext(in.ctx, expireOlderThan, cutoff)
	return err
}

func (in *DatabaseStore) maybeSaveHookComponent(resource unstructured.Unstructured, state client.ComponentState, serviceID string) {
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

	if _, err := in.db.ExecContext(in.ctx, setHookComponent,
		gvk.Group,
		gvk.Version,
		gvk.Kind,
		resource.GetNamespace(),
		resource.GetName(),
		resource.GetUID(),
		NewComponentState(&state),
		serviceID,
		smcommon.GetPhaseHookDeletePolicy(resource),
	); err != nil {
		klog.V(log.LogLevelMinimal).ErrorS(err, "failed to save hook", "resource", resource)
	}
}

func (in *DatabaseStore) maybeSaveHookComponents(resources []unstructured.Unstructured) {
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
			fmt.Sprintf("('%s','%s','%s','%s','%s','%s','%d','%s','%s')", gvk.Group, gvk.Version, gvk.Kind,
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

	if _, err := in.db.ExecContext(in.ctx, sb.String()); err != nil {
		klog.V(log.LogLevelMinimal).ErrorS(err, "failed to save hook", "resources", resources)
	}
}

func (in *DatabaseStore) GetHookComponents(serviceID string) ([]smcommon.HookComponent, error) {
	rows, err := in.db.QueryContext(in.ctx, `SELECT "group", version, kind, namespace, name, uid, status, manifest_sha, delete_policies FROM hook_component WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]smcommon.HookComponent, 0)
	for rows.Next() {
		var group, version, kind, namespace, name, uid, deletePolicies string
		var manifestSHA sql.NullString
		var status sql.NullInt32
		if err := rows.Scan(&group, &version, &kind, &namespace, &name, &uid, &status, &manifestSHA, &deletePolicies); err != nil {
			return nil, err
		}
		result = append(result, smcommon.HookComponent{
			Group:          group,
			Version:        version,
			Kind:           kind,
			Namespace:      namespace,
			Name:           name,
			UID:            uid,
			Status:         ComponentState(status.Int32).String(),
			ManifestSHA:    manifestSHA.String,
			ServiceID:      serviceID,
			DeletePolicies: smcommon.SplitHookDeletePolicy(deletePolicies),
		})
	}

	return result, rows.Err()
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

	gvk := manifest.GroupVersionKind()

	_, err = in.db.ExecContext(in.ctx, setHookComponentWithManifestSHA,
		gvk.Group,
		gvk.Version,
		gvk.Kind,
		manifest.GetNamespace(),
		manifest.GetName(),
		appliedResource.GetUID(),
		NewComponentState(common.ToStatus(&appliedResource)),
		manifestSHA,
		serviceID,
	)
	return err
}

func (in *DatabaseStore) ExpireHookComponents(serviceID string) error {
	_, err := in.db.ExecContext(in.ctx, `DELETE FROM hook_component WHERE service_id = ?`, serviceID)
	return err
}

func (in *DatabaseStore) Shutdown() error {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.db != nil {
		if err := in.db.Close(); err != nil {
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
