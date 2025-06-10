package db

const (
	createTables = `
		CREATE TABLE IF NOT EXISTS component (
			id INTEGER PRIMARY KEY,
			parent_uid TEXT,
			uid TEXT UNIQUE,
			"group" TEXT,
			version TEXT,
			kind TEXT, 
			namespace TEXT,
			name TEXT,
			health INT,
			node TEXT,
			createdAt TIMESTAMP,
			FOREIGN KEY(parent_uid) REFERENCES component(uid)
		);
		CREATE INDEX IF NOT EXISTS idx_parent ON component(parent_uid);
		CREATE INDEX IF NOT EXISTS idx_uid ON component(uid);
	`

	setComponent = `
		INSERT INTO component (
			uid,
			parent_uid,
			'group',
			version,
			kind, 
			namespace,
			name,
			health,
		    node,
		    createdAt
		) VALUES (
			?,
			?,
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
			health = excluded.health,
			node = excluded.node,
			createdAt = excluded.createdAt
	`

	componentChildren = `
		WITH RECURSIVE descendants AS (
			SELECT uid, "group", version, kind, namespace, name, health, parent_uid, 1 as level
			FROM component 
			WHERE parent_uid = ?
			
			UNION ALL
			
			SELECT c.uid, c."group", c.version, c.kind, c.namespace, c.name, c.health, c.parent_uid, d.level + 1
			FROM descendants d
			JOIN component c ON c.parent_uid = d.uid
			WHERE d.level < 4
		)
		SELECT uid, "group", version, kind, namespace, name, health, parent_uid
		FROM descendants
	`

	clusterHealthScore = `SELECT CAST(AVG(health = 0) * 100 as INTEGER) as score FROM component`

	nodeStatistics = `
		SELECT node, COUNT(*)
		FROM component
		WHERE kind = 'Pod' AND createdAt <= strftime('%s', 'now', '-5 minutes')
		GROUP BY node
	`

	failedComponents = `
		WITH RECURSIVE component_chain AS (
			-- Start with parent components of specified kinds
			SELECT *, 1 as level, uid as root_uid
			FROM component 
			WHERE kind IN ('Deployment', 'StatefulSet', 'Ingress', 'DaemonSet', 'Certificate')
			
			UNION ALL
			
			-- Get children of components in the chain, carrying the root component UID
			SELECT c.*, cc.level + 1, cc.root_uid
			FROM component c
			JOIN component_chain cc ON c.parent_uid = cc.uid
			WHERE cc.level < 4
		),
		-- Find all failed components in the chain
		failed_roots AS (
			-- Get root UIDs where any component in the chain is failed
			SELECT DISTINCT root_uid
			FROM component_chain
			WHERE health = 2
		)
		-- Return both the failed components and their original parent components
		SELECT DISTINCT cc.uid, cc."group", cc.version, cc.kind, cc.namespace, cc.name
		FROM component_chain cc
		WHERE (cc.health = 2  -- The component itself is failed
		   OR cc.uid IN (    -- OR it's a direct parent of a failed component
			  SELECT parent_uid 
			  FROM component_chain 
			  WHERE health = 2 AND parent_uid IS NOT NULL
		   )
		   OR (cc.uid IN (  -- OR it's the original root component of a chain with failures
			  SELECT root_uid FROM failed_roots
		   ) AND cc.kind IN ('Deployment', 'StatefulSet', 'Ingress', 'DaemonSet', 'Certificate')))
           AND cc.kind IN ('Deployment', 'StatefulSet', 'Ingress', 'DaemonSet', 'Certificate')
	`
)
