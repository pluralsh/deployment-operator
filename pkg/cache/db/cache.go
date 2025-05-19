package db

import (
	"context"

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
	CacheModeMemory CacheMode = "file:memory:?mode=memory"
)

type ComponentCache struct {
	poolSize int
	mode     CacheMode

	pool *sqlitex.Pool
}

func (in *ComponentCache) Children(uid string) (result []client.ComponentChildAttributes, err error) {
	conn, err := in.pool.Take(context.Background())
	if err != nil {
		return result, err
	}

	defer in.pool.Put(conn)

	query := `
		SELECT uid, group, version, kind, namespace, name, health
		FROM Component 
		WHERE parent = (
			SELECT id FROM Component WHERE uid = ?
		)
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
		INSERT OR REPLACE INTO Component (
			uid,
			parent,
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
		)
	`

	return sqlitex.ExecuteTransient(conn, query, &sqlitex.ExecOptions{
		Args: []interface{}{
			component.UID,
			component.ParentUID,
			lo.FromPtr(component.Group),
			component.Version,
			component.Kind,
			lo.FromPtr(component.Namespace),
			component.Name,
			lo.FromPtr(component.State),
		},
	})
}

func (in *ComponentCache) init() (*ComponentCache, error) {
	pool, err := sqlitex.NewPool(string(in.mode), sqlitex.PoolOptions{
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
			parent INTEGER,
			uid TEXT,
			'group' TEXT,
			version TEXT,
			kind TEXT, 
			namespace TEXT,
			'name' TEXT,
			health TEXT,
			FOREIGN KEY(parent) REFERENCES Component(id)
		);
		CREATE INDEX IF NOT EXISTS idx_parent ON Component(parent);
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
