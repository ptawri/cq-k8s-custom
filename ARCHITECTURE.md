# Architecture: CloudQuery Kubernetes Plugin

## Overview

This is a CloudQuery **source plugin** that discovers and monitors Kubernetes resources across multiple clusters. It integrates with CloudQuery's gRPC plugin framework and persists data to PostgreSQL.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    CloudQuery CLI                           │
│                  (cloudquery sync)                          │
└────────────────────┬────────────────────────────────────────┘
                     │
                     │ gRPC (plugin protocol)
                     ▼
┌─────────────────────────────────────────────────────────────┐
│        Plugin Server (cmd/plugin/main.go)                   │
│  - Implements SourceClient interface                        │
│  - Serves via CloudQuery SDK v4 serve.Plugin()             │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
    ┌────────┐  ┌────────┐  ┌─────────┐
    │  Sync  │  │ Tables │  │ Close   │
    │ Method │  │ Method │  │ Method  │
    └────────┘  └────────┘  └─────────┘
        │            │
        │ (plugin/source_client.go - 276 lines)
        │
        ├─ Load Config (database_url, contexts, resources)
        ├─ Discover K8s Contexts from ~/.kube/config
        ├─ Iterate each context
        │   ├─ Fetch Namespaces
        │   ├─ Fetch Pods
        │   ├─ Fetch Deployments
        │   ├─ Fetch Services
        │   └─ Fetch CustomResourceDefinitions (CRDs)
        │
        └─ Store Data in PostgreSQL (internal/db.go)
           └─ Upsert with composite key: (context_name, uid)
```

## File Organization

### Entry Point
- **`cmd/plugin/main.go`** (16 lines)
  - Serves the plugin via CloudQuery SDK
  - No business logic; purely orchestration
  - Invoked by `cloudquery sync` command

### Plugin Registration
- **`plugin/plugin.go`** (14 lines)
  - Registers plugin name, version, and SourceClient factory
  - Returns `plugin.Plugin` for CloudQuery framework

### Core Logic
- **`plugin/source_client.go`** (276 lines) ⭐ **NEW**
  - Implements `plugin.SourceClient` interface:
    - `NewSourceClient(ctx, logger, spec)` — Constructor with config loading
    - `Tables(ctx, options)` — Returns all table schemas
    - `Sync(ctx, options, res)` — Main sync entry point
    - `Close(ctx)` — Cleanup
  - Config parsing: JSON spec from CloudQuery or env vars
  - Per-resource sync helpers: `syncNamespaces()`, `syncPods()`, etc.
  - Sends `SyncMessage` (migrate table, insert rows) to CloudQuery

### Schema Definitions
- **`plugin/resources_tables.go`** (OLD pattern, still used)
  - Defines table schemas for CloudQuery:
    - `k8s_namespaces`
    - `k8s_pods`
    - `k8s_deployments`
    - `k8s_services`
    - `k8s_custom_resources`

- **`plugin/*_resolver.go`** (OLD pattern)
  - Legacy resolvers (not used by new architecture)
  - Can be removed in future refactor

### Kubernetes Client
- **`internal/client.go`** (83 lines)
  - Multi-context support: `NewForContext(ctx, kubeContext string)`
  - Discovers available contexts: `GetAvailableContexts()`
  - Provides Kubernetes clientset + API extensions client

### Data Persistence
- **`internal/db.go`** (100+ lines)
  - PostgreSQL connection management (pgx v5)
  - Schema creation with composite primary keys: `(context_name, uid)`
  - Upsert methods for each resource type:
    - `UpsertNamespace()`, `UpsertPod()`, `UpsertDeployment()`, `UpsertService()`, `UpsertCRD()`
  - ON CONFLICT DO UPDATE for idempotent updates

## Data Model

### Schema Structure
All tables follow this pattern:

```sql
CREATE TABLE k8s_<resource_type> (
    context_name TEXT NOT NULL,
    uid TEXT NOT NULL,
    name TEXT,
    namespace TEXT,
    created_at TIMESTAMP,
    -- resource-specific fields
    PRIMARY KEY (context_name, uid)
);
```

Example: `k8s_pods`
- Composite key: `(context_name, uid)` ensures per-cluster uniqueness
- `context_name`: "dev", "prod", etc.
- `uid`: Kubernetes object UUID
- Supports multi-cluster deployments without conflicts

## Configuration

### Option 1: JSON Spec (CloudQuery)
```yaml
sources:
  - name: k8s-custom
    spec:
      database_url: postgres://user:pass@localhost:5432/k8s?sslmode=disable
      contexts:
        - dev
        - prod
      resources:
        - namespaces
        - pods
        - deployments
        - services
        - crds
```

### Option 2: Environment Variables (Testing)
```zsh
export DATABASE_URL="postgres://user:pass@localhost:5432/k8s?sslmode=disable"
export K8S_CONTEXTS="dev,prod"
export K8S_RESOURCES="namespaces,pods,deployments,services,crds"
```

## Sync Workflow

1. **CloudQuery CLI** invokes plugin via gRPC
2. **Plugin loads config** from spec or env vars
3. **For each filtered context:**
   - Create Kubernetes client for that context
   - For each filtered resource type:
     - Fetch resources from Kubernetes API
     - Convert to CloudQuery row format
     - Send `SyncMigrateTable` message (schema)
     - Send `SyncInsert` message (rows)
     - Store to Postgres via `db.Store.Upsert*()`
4. **CloudQuery receives messages** and applies to destinations
5. **Plugin closes** connection cleanly

## Dependencies

### Go Modules
- `github.com/cloudquery/plugin-sdk/v4` (v4.94.0) — gRPC plugin framework
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `k8s.io/client-go` (v0.35.0) — Kubernetes API client
- `k8s.io/apiextensions-apiserver` — CRD discovery
- `github.com/rs/zerolog` — Structured logging

### External Services
- **PostgreSQL** — Data persistence (composite-key schema)
- **Kubernetes clusters** — Resource discovery (via kubeconfig)
- **~/.kube/config** — Kubernetes context configuration

## Testing

See `TESTING.md` for:
- Prerequisites setup
- CloudQuery sync execution
- Postgres verification
- Filtering tests
- Troubleshooting

## Extension Ideas

1. **More resource types:**
   - StatefulSets, DaemonSets, Jobs, CronJobs
   - ConfigMaps, Secrets, RBAC roles
   - PVs, PVCs, Ingresses, NetworkPolicies

2. **Enhanced metadata:**
   - Resource labels/annotations
   - Owner references (pod → deployment → statefulset)
   - Resource events (create, update, delete)

3. **Query optimization:**
   - Incremental sync (only changed resources)
   - Watch mode (stream changes instead of full sync)
   - Caching layer for expensive discoveries

4. **Multi-cloud:**
   - EKS, GKE, AKS cluster discovery
   - Cross-cloud resource correlation

## Performance Notes

- **Namespace isolation:** Composite key `(context_name, uid)` allows any number of clusters
- **Upsert efficiency:** ON CONFLICT DO UPDATE is atomic and efficient
- **API rate limiting:** Respects Kubernetes API quotas
- **Postgres connection pooling:** Uses pgx connection pool for concurrent operations

## Security Considerations

1. **kubeconfig** — Must have read access to `~/.kube/config`
2. **RBAC** — Requires read permissions on all monitored resources
3. **Database credentials** — Pass via `database_url` or `DATABASE_URL` env var (consider secrets management)
4. **Network** — PostgreSQL connection should be over TLS in production

## Debugging

### Enable verbose logging:
```zsh
cloudquery sync config.yml --log-level debug
```

### Check plugin logs:
```zsh
./bin/plugin 2>&1 | head -100
```

### Verify Kubernetes access:
```zsh
kubectl --context=dev get all -A
kubectl --context=prod get all -A
```

### Query Postgres directly:
```zsh
psql -U postgres k8s -c "SELECT * FROM k8s_namespaces LIMIT 5;"
```
