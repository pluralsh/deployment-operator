package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	cache *ComponentCache
)

const (
	defaultPoolSize = 50
	defaultMode     = CacheModeMemory
)

type CacheMode string

const (
	CacheModeMemory CacheMode = "file::memory:?mode=memory&cache=shared"
	CacheModeFile   CacheMode = "file"
)

type ComponentCache struct {
	poolSize int
	mode     CacheMode
	filePath string

	pool *sqlitex.Pool
}

func (in *ComponentCache) Children(uid string) (result []client.ComponentChildAttributes, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}

	defer in.pool.Put(conn)

	query := `
		WITH RECURSIVE descendants AS (
			SELECT uid, 'group', version, kind, namespace, name, health, 1 as level
			FROM Component 
			WHERE parent_uid = (SELECT uid FROM Component WHERE uid = ?)
	
			UNION ALL
	
			SELECT c.uid, c.'group', c.version, c.kind, c.namespace, c.name, c.health, d.level + 1
			FROM Component c
			JOIN descendants d ON c.parent_uid = d.uid
			WHERE d.level < 4
		)
		SELECT uid, 'group', version, kind, namespace, name, health
		FROM descendants
	`

	err = sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []interface{}{uid},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, client.ComponentChildAttributes{
				UID:       stmt.ColumnText(0),
				ParentUID: &uid,
				Group:     lo.ToPtr(stmt.ColumnText(1)),
				Version:   stmt.ColumnText(2),
				Kind:      stmt.ColumnText(3),
				Namespace: lo.ToPtr(stmt.ColumnText(4)),
				Name:      stmt.ColumnText(5),
				State:     lo.ToPtr(client.ComponentState(stmt.ColumnText(6))),
			})
			return nil
		},
	})

	return result, err
}

func (in *ComponentCache) Set(component client.ComponentChildAttributes) error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer in.pool.Put(conn)

	query := `
		INSERT INTO Component (
			uid,
			parent_uid,
			'group',
			version,
			kind, 
			namespace,
			name,
			health
		) VALUES (
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
		) ON CONFLICT(uid) DO UPDATE SET
			parent_uid = excluded.parent_uid,
			'group' = excluded.'group',
			version = excluded.version,
			kind = excluded.kind,
			namespace = excluded.namespace,
			name = excluded.name,
			health = excluded.health
	`

	return sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []interface{}{
			component.UID,
			lo.FromPtr(component.ParentUID),
			lo.FromPtr(component.Group),
			component.Version,
			component.Kind,
			lo.FromPtr(component.Namespace),
			component.Name,
			lo.FromPtr(component.State),
		},
	})
}

// Close closes the connection pool and cleans up temporary file if necessary
func (in *ComponentCache) Close() error {
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

		// If the file was in a temp directory we created, remove that too
		if dir := filepath.Dir(in.filePath); strings.HasPrefix(dir, os.TempDir()) {
			return os.RemoveAll(dir)
		}
	}

	cache = nil

	return nil
}

func (in *ComponentCache) init() (*ComponentCache, error) {
	connectionString := ""

	if in.mode == CacheModeFile {
		if len(in.filePath) == 0 {
			tempDir, err := os.MkdirTemp("", "component-cache-*")
			if err != nil {
				return in, err
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
		return in, err
	}

	in.pool = pool
	if err = in.initTable(); err != nil {
		return in, err
	}

	return in, nil
}

func (in *ComponentCache) initTable() error {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return err
	}

	defer in.pool.Put(conn)
	query := `
		CREATE TABLE IF NOT EXISTS Component (
			id INTEGER PRIMARY KEY,
			parent_uid TEXT,
			uid TEXT UNIQUE,
			'group' TEXT,
			version TEXT,
			kind TEXT, 
			namespace TEXT,
			'name' TEXT,
			health TEXT,
			FOREIGN KEY(parent_uid) REFERENCES Component(uid)
		);
		CREATE INDEX IF NOT EXISTS idx_parent ON Component(parent_uid);
		CREATE INDEX IF NOT EXISTS idx_uid ON Component(uid);
	`

	return sqlitex.ExecuteScript(conn, query, nil)
}

type Option func(*ComponentCache)

func WithPoolSize(size int) Option {
	return func(in *ComponentCache) {
		in.poolSize = size
	}
}

func WithMode(mode CacheMode) Option {
	return func(in *ComponentCache) {
		in.mode = mode
	}
}

func WithFilePath(path string) Option {
	return func(in *ComponentCache) {
		in.filePath = path
	}
}

func NewComponentCache(args ...Option) (*ComponentCache, error) {
	if cache != nil {
		return cache, nil
	}

	cache = &ComponentCache{
		poolSize: defaultPoolSize,
		mode:     defaultMode,
	}

	for _, arg := range args {
		arg(cache)
	}

	return cache.init()
}
