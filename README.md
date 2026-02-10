# CloudQuery Kubernetes Custom Plugin

## Overview
This is a custom CloudQuery source plugin that queries multiple Kubernetes contexts from your kubeconfig and lists core resources plus CRDs. It currently prints results to stdout and exposes tables to CloudQuery.

## Resources Supported
- Cluster metadata (per context)
- Namespaces
- Pods
- Deployments
- Services
- CustomResourceDefinitions (CRDs)

## Tables Exposed
- `k8s_clusters`
- `k8s_namespaces`
- `k8s_pods`
- `k8s_deployments`
- `k8s_services`
- `k8s_custom_resources`

## Cluster Metadata
Each context is stored in `k8s_clusters` with server, CA file, default namespace, Kubernetes version, and node count.

## Build
```zsh
go mod tidy

go build -o ./bin/plugin ./cmd/plugin
```

## Run (Local)
```zsh
./bin/plugin
```

## Store Data in Postgres
Set a PostgreSQL connection string in `DATABASE_URL` before running the plugin. Tables are created automatically.

```zsh
export DATABASE_URL="postgres://user:password@localhost:5432/k8s?sslmode=disable"
./bin/plugin
```

## Run with CloudQuery

This plugin integrates with CloudQuery v6 via the `cloudquery` CLI. It emits Apache Arrow records as `SyncInsert` messages, allowing CloudQuery destination plugins to handle data persistence.

### Source Configuration

Create `cloudquery_sync.yml`:

```yaml
kind: source
spec:
  name: k8s-custom
  registry: local
  path: ./bin/plugin
  destinations:
    - postgres
  spec:
    database_url: postgres://user:password@localhost:5432/k8s?sslmode=disable
    contexts:
      - dev
      - prod
    resources:
      - clusters
      - namespaces
      - pods
      - deployments
      - services
      - crds
  tables:
    - k8s_clusters
    - k8s_namespaces
    - k8s_pods
    - k8s_deployments
    - k8s_services
    - k8s_custom_resources
```

### Destination Configuration

Create `cloudquery_destination.yml`:

```yaml
kind: destination
spec:
  name: postgres
  registry: cloudquery
  path: cloudquery/postgresql
  version: "v8.14.0"
  migrate_mode: forced
  spec:
    connection_string: postgres://user:password@localhost:5432/k8s?sslmode=disable
```

### Run Sync

```zsh
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml
```

The plugin emits Kubernetes resources as Arrow records through CloudQuery's message pipeline. The PostgreSQL destination plugin receives these messages and persists data to the database.

## Notes
- The plugin discovers all Kubernetes contexts from `~/.kube/config` and iterates through them.
- Contexts that are not running will print connection errors and continue.

## SQL Examples
List namespaces per cluster/context:

```sql
SELECT c.context_name,
       c.cluster_name,
       n.name AS namespace
FROM k8s_namespaces n
JOIN k8s_clusters c ON c.context_name = n.context_name
ORDER BY c.context_name, n.name;
```

## Extension Ideas
- Workloads: StatefulSets, DaemonSets, Jobs, CronJobs
- Config: ConfigMaps, Secrets, ResourceQuotas, LimitRanges
- Networking: Ingresses, NetworkPolicies, EndpointSlices
- Storage: PVs, PVCs, StorageClasses
- Security: RBAC roles/rolebindings, ServiceAccounts
- CRDs: enumerate custom resources (not only CRDs) via dynamic client
