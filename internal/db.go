package internal

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS k8s_clusters (
	cluster_uid TEXT PRIMARY KEY,
	context_name TEXT,
	cluster_name TEXT NOT NULL,
	server TEXT,
	ca_file TEXT,
	insecure_skip_verify BOOLEAN DEFAULT FALSE,
	namespace TEXT DEFAULT 'default',
	kubernetes_version TEXT,
	node_count INTEGER,
	synced_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS k8s_namespaces (
	cluster_uid TEXT NOT NULL,
	context_name TEXT,
	uid TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (cluster_uid, uid),
	FOREIGN KEY (cluster_uid) REFERENCES k8s_clusters(cluster_uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS k8s_pods (
	cluster_uid TEXT NOT NULL,
	context_name TEXT,
	uid TEXT NOT NULL,
	namespace TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (cluster_uid, uid),
	FOREIGN KEY (cluster_uid) REFERENCES k8s_clusters(cluster_uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS k8s_deployments (
	cluster_uid TEXT NOT NULL,
	context_name TEXT,
	uid TEXT NOT NULL,
	namespace TEXT NOT NULL,
	name TEXT NOT NULL,
	replicas INTEGER NOT NULL,
	ready INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (cluster_uid, uid),
	FOREIGN KEY (cluster_uid) REFERENCES k8s_clusters(cluster_uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS k8s_services (
	cluster_uid TEXT NOT NULL,
	context_name TEXT,
	uid TEXT NOT NULL,
	namespace TEXT NOT NULL,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	cluster_ip TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (cluster_uid, uid),
	FOREIGN KEY (cluster_uid) REFERENCES k8s_clusters(cluster_uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS k8s_crds (
	cluster_uid TEXT NOT NULL,
	context_name TEXT,
	uid TEXT NOT NULL,
	name TEXT NOT NULL,
	group_name TEXT NOT NULL,
	kind TEXT NOT NULL,
	plural TEXT NOT NULL,
	scope TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (cluster_uid, uid),
	FOREIGN KEY (cluster_uid) REFERENCES k8s_clusters(cluster_uid) ON DELETE CASCADE
);
`)
	return err
}

func (s *Store) UpsertCluster(ctx context.Context, clusterUID, contextName, clusterName, server, caFile string, insecureSkipVerify bool, namespace, kubernetesVersion string, nodeCount int64) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_clusters (cluster_uid, context_name, cluster_name, server, ca_file, insecure_skip_verify, namespace, kubernetes_version, node_count, synced_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (cluster_uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	cluster_name = EXCLUDED.cluster_name,
	server = EXCLUDED.server,
	ca_file = EXCLUDED.ca_file,
	insecure_skip_verify = EXCLUDED.insecure_skip_verify,
	namespace = EXCLUDED.namespace,
	kubernetes_version = EXCLUDED.kubernetes_version,
	node_count = EXCLUDED.node_count,
	synced_at = EXCLUDED.synced_at,
	updated_at = EXCLUDED.updated_at;
`, clusterUID, contextName, clusterName, server, caFile, insecureSkipVerify, namespace, kubernetesVersion, nodeCount, now, now, now)
	return err
}

func (s *Store) UpsertNamespace(ctx context.Context, clusterUID, contextName, uid, name, status string, createdAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_namespaces (cluster_uid, context_name, uid, name, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (cluster_uid, uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	name = EXCLUDED.name,
	status = EXCLUDED.status,
	created_at = EXCLUDED.created_at;
`, clusterUID, contextName, uid, name, status, createdAt)
	return err
}

func (s *Store) UpsertPod(ctx context.Context, clusterUID, contextName, uid, namespace, name, status string, createdAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_pods (cluster_uid, context_name, uid, namespace, name, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (cluster_uid, uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	namespace = EXCLUDED.namespace,
	name = EXCLUDED.name,
	status = EXCLUDED.status,
	created_at = EXCLUDED.created_at;
`, clusterUID, contextName, uid, namespace, name, status, createdAt)
	return err
}

func (s *Store) UpsertDeployment(ctx context.Context, clusterUID, contextName, uid, namespace, name string, replicas, ready int32, createdAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_deployments (cluster_uid, context_name, uid, namespace, name, replicas, ready, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (cluster_uid, uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	namespace = EXCLUDED.namespace,
	name = EXCLUDED.name,
	replicas = EXCLUDED.replicas,
	ready = EXCLUDED.ready,
	created_at = EXCLUDED.created_at;
`, clusterUID, contextName, uid, namespace, name, replicas, ready, createdAt)
	return err
}

func (s *Store) UpsertService(ctx context.Context, clusterUID, contextName, uid, namespace, name, serviceType, clusterIP string, createdAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_services (cluster_uid, context_name, uid, namespace, name, type, cluster_ip, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (cluster_uid, uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	namespace = EXCLUDED.namespace,
	name = EXCLUDED.name,
	type = EXCLUDED.type,
	cluster_ip = EXCLUDED.cluster_ip,
	created_at = EXCLUDED.created_at;
`, clusterUID, contextName, uid, namespace, name, serviceType, clusterIP, createdAt)
	return err
}

func (s *Store) UpsertCRD(ctx context.Context, clusterUID, contextName, uid, name, groupName, kind, plural, scope string, createdAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO k8s_crds (cluster_uid, context_name, uid, name, group_name, kind, plural, scope, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (cluster_uid, uid)
DO UPDATE SET context_name = EXCLUDED.context_name,
	name = EXCLUDED.name,
	group_name = EXCLUDED.group_name,
	kind = EXCLUDED.kind,
	plural = EXCLUDED.plural,
	scope = EXCLUDED.scope,
	created_at = EXCLUDED.created_at;
`, clusterUID, contextName, uid, name, groupName, kind, plural, scope, createdAt)
	return err
}
