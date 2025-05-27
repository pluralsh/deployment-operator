package db

const (
	createTables = `
		CREATE TABLE IF NOT EXISTS Component (
			id INTEGER PRIMARY KEY,
			parent_uid TEXT,
			uid TEXT UNIQUE,
			"group" TEXT,
			version TEXT,
			kind TEXT, 
			namespace TEXT,
			name TEXT,
			health INT,
			FOREIGN KEY(parent_uid) REFERENCES Component(uid)
		);
		CREATE INDEX IF NOT EXISTS idx_parent ON Component(parent_uid);
		CREATE INDEX IF NOT EXISTS idx_uid ON Component(uid);

		CREATE TABLE IF NOT EXISTS Pod (
			id INTEGER PRIMARY KEY,
			name TEXT,
			namespace TEXT,
			uid TEXT UNIQUE,
			node TEXT,
			createdAt TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_pod_uid ON Pod(uid);
	`

	setComponent = `
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
			"group" = excluded."group",
			version = excluded.version,
			kind = excluded.kind,
			namespace = excluded.namespace,
			name = excluded.name,
			health = excluded.health
	`

	componentChildren = `
		WITH RECURSIVE descendants AS (
			SELECT uid, "group", version, kind, namespace, name, health, parent_uid, 1 as level
			FROM Component 
			WHERE parent_uid = ?
			
			UNION ALL
			
			SELECT c.uid, c."group", c.version, c.kind, c.namespace, c.name, c.health, c.parent_uid, d.level + 1
			FROM descendants d
			JOIN Component c ON c.parent_uid = d.uid
			WHERE d.level < 4
		)
		SELECT uid, "group", version, kind, namespace, name, health, parent_uid
		FROM descendants
	`

	clusterHealthScore = `SELECT CAST(AVG(health = 0) * 100 as INTEGER) as score FROM Component`

	setPod = `
		INSERT INTO Pod (
			name,
			namespace,
			uid,
			node,
			createdAt
		) VALUES (
			?,
			?,
			?,
			?,
			?
		) ON CONFLICT(uid) DO UPDATE SET
			name = excluded.name,
			namespace = excluded.namespace,
			node = excluded.node,
			createdAt = excluded.createdAt
	`

	nodeStatistics = `
	SELECT node, COUNT(*)
	FROM Pod
	WHERE datetime(createdAt) < datetime('now', '-5 minutes')
	GROUP BY node
	`
)
