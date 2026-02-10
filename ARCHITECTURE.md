# Architecture: CloudQuery Kubernetes Plugin

## Overview

This is a CloudQuery v6 **source plugin** that discovers and monitors Kubernetes resources across multiple clusters. It emits Apache Arrow records as `SyncInsert` messages through CloudQuery's message pipeline for destination plugins to persist.

## System Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    CloudQuery CLI v6                             │
│                  (cloudquery sync)                               │
└────────────────────┬─────────────────────────────────────────────┘
                     │
                     │ gRPC (plugin protocol v3)
                     ▼
┌──────────────────────────────────────────────────────────────────┐
│   CloudQuery Source Plugin: k8s-custom (./bin/plugin)            │
│                                                                  │
│  Implements CloudQuery SourceClient interface:                  │
│  - Sync(ctx, SyncOptions, chan<- SyncMessage)                  │
│  - Tables(ctx) -> []*schema.Table                               │
│  - Close(ctx)                                                   │
└────────────────────┬─────────────────────────────────────────────┘
                     │
                     │ Emit message.SyncInsert with Arrow records
                     │ (RecordBuilder pattern with Timestamp_ns types)
                     ▼
        ┌────────────────────────────┐
        │  Kubernetes Client         │
        │  (Multi-context discovery) │
        │                            │
        │  For each context:         │
        │  - Fetch Clusters          │
        │  - Fetch Namespaces        │
        │  - Fetch Pods              │
        │  - Fetch Deployments       │
        │  - Fetch Services          │
        │  - Fetch CRDs              │
        └────────────────────────────┘
                     │
                     │ SyncInsert messages with Arrow records
                     │ (RecordBatch via RecordBuilder)
                     ▼
┌──────────────────────────────────────────────────────────────────┐
│         CloudQuery Destination: PostgreSQL v8.14.0               │
│                                                                  │
│  - Consumes SyncInsert messages                                 │
│  - Deserializes Arrow records                                   │
│  - Applies schema migration (forced mode)                       │
│  - Persists rows to tables                                      │
└──────────────────────────────────────────────────────────────────┘
                     │
                     ▼
            ┌──────────────────┐
            │   PostgreSQL     │
            │   Database       │
            │   (k8s schema)   │
            └──────────────────┘
```

## Message Pipeline (CloudQuery v6)

The plugin uses CloudQuery v6's message-based architecture:

```go
// Source plugin emits messages:
res <- &message.SyncInsert{
    Record: recordBuilder.NewRecord()  // Apache Arrow RecordBatch
}

// CloudQuery CLI receives messages and routes to destination
// Destination plugin (PostgreSQL) receives and persists
```

## Data Serialization (Apache Arrow)

Each resource is converted to an Apache Arrow record:

```go
table := NamespacesTable()  // Get schema.Table with Arrow types
bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
defer bldr.Release()

// Append field values with proper type builders
bldr.Field(idx).(*array.StringBuilder).Append(value)
bldr.Field(idx).(*array.TimestampBuilder).Append(arrow.Timestamp(time.UnixNano()))
bldr.Field(idx).(*types.UUIDBuilder).Append(uuid.Parse(...))

// Emit as SyncInsert message
res <- &message.SyncInsert{Record: bldr.NewRecord()}
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
- **`plugin/source_client.go`** (433 lines) ⭐ **NOW WITH SYNCINSERT**
  - Implements `plugin.SourceClient` interface:
    - `NewSourceClient(ctx, logger, spec)` — Constructor with YAML/JSON config loading
    - `Tables(ctx, options)` — Returns all table schemas with Arrow types
    - `Sync(ctx, options, res)` — Main sync entry point emitting SyncInsert messages
    - `Close(ctx)` — Cleanup
  - **SyncInsert Message Emission:**
    - `syncCluster()` — Emits cluster metadata records
    - `syncNamespaces()` — Emits namespace records with UUID builder
    - `syncPods()` — Emits pod records with timestamps
    - `syncDeployments()` — Emits deployment records with replica counts
    - `syncServices()` — Emits service records with types and IPs
    - `syncCRDs()` — Emits custom resource definition records
  - **Arrow Record Building:**
    - Uses `array.NewRecordBuilder()` with proper schema
    - Type-safe builders: `UUIDBuilder`, `StringBuilder`, `Int64Builder`, `BooleanBuilder`, `TimestampBuilder`
    - Emits `&message.SyncInsert{Record: bldr.NewRecord()}` for each resource
  - Config parsing: YAML/JSON spec from CloudQuery or env vars
  - CloudQuery v6 format: Reads from `kind: source` config files

### Schema Definitions
- **`plugin/resources_tables.go`** (87 lines)
  - Defines CloudQuery table schemas with Apache Arrow types:
    - `ClustersTable()` — context_name (PK), cluster metadata, timestamps
    - `NamespacesTable()` — UUID (PK), name, status, created_at
    - `PodsTable()` — UUID (PK), name, namespace, status, timestamp
    - `DeploymentsTable()` — UUID (PK), name, namespace, replicas (Int64), ready (Int64), timestamp
    - `ServicesTable()` — UUID (PK), name, namespace, type, cluster_ip, timestamp
    - `CustomResourcesTable()` — UUID (PK), name, group, kind, plural, scope, timestamp
  - **Arrow Type Mapping:**
    - UUIDs: `types.ExtensionTypes.UUID` → `types.UUIDBuilder`
    - Strings: `arrow.BinaryTypes.String` → `array.StringBuilder`
    - Integers: `arrow.PrimitiveTypes.Int64` → `array.Int64Builder`
    - Booleans: `arrow.FixedWidthTypes.Boolean` → `array.BooleanBuilder`
    - Timestamps: `arrow.FixedWidthTypes.Timestamp_ns` → `array.TimestampBuilder`

- **`plugin/namespaces.go`** (35 lines)
  - Namespace table schema with proper Arrow types
  - Referenced by `syncNamespaces()` for record building

- **`plugin/*_resolver.go`** (Legacy, deprecated)
  - Old resolvers not used with SyncInsert architecture
  - Can be removed in future cleanup

### Kubernetes Client
- **`internal/client.go`** (83 lines)
  - Multi-context support: `NewForContext(ctx, kubeContext string)`
  - Discovers available contexts: `GetAvailableContexts()`
  - Provides Kubernetes clientset + API extensions client

### Data Persistence (Deprecated)
- **`internal/db.go`** (163 lines) — Legacy direct database layer
  - Now bypassed by CloudQuery destination plugin
  - PostgreSQL connection management (pgx v5)
  - Kept for backward compatibility
  - No longer called by SyncInsert architecture

## Data Model

### Cluster Metadata
```sql
CREATE TABLE k8s_clusters (
  context_name TEXT PRIMARY KEY,
  cluster_name TEXT NOT NULL,
  server TEXT,
  ca_file TEXT,
  insecure_skip_verify BOOLEAN DEFAULT FALSE,
  namespace TEXT DEFAULT 'default',
  kubernetes_version TEXT,
  node_count INTEGER,
  synced_at TIMESTAMP,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

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

## Configuration (CloudQuery v6 Format)

### Source Configuration (cloudquery_sync.yml)
```yaml
kind: source
spec:
  name: k8s-custom
  registry: local
  path: ./bin/plugin
  destinations:
    - postgres
  spec:
    database_url: postgres://user:pass@localhost:5432/k8s?sslmode=disable
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

### Destination Configuration (cloudquery_destination.yml)
```yaml
kind: destination
spec:
  name: postgres
  registry: cloudquery
  path: cloudquery/postgresql
  version: "v8.14.0"
  migrate_mode: forced
  spec:
    connection_string: postgres://user:pass@localhost:5432/k8s?sslmode=disable
```

### Environment Variables (Testing)
```zsh
export DATABASE_URL="postgres://user:pass@localhost:5432/k8s?sslmode=disable"
export K8S_CONTEXTS="dev,prod"
export K8S_RESOURCES="clusters,namespaces,pods,deployments,services,crds"
```

## Sync Workflow (CloudQuery v6 SyncInsert)

1. **CloudQuery CLI** invokes plugin via gRPC with `Sync(ctx, SyncOptions, chan SyncMessage)`

2. **Plugin initializes:**
   - Parses YAML/JSON config from `kind: source` spec
   - Discovers Kubernetes contexts from `~/.kube/config`

3. **For each context (filtered by config):**
   - Create Kubernetes client via `internal.NewForContext()`
   - Get cluster metadata (server, version, node count)

4. **For each resource type (filtered by config):**
   - Fetch from Kubernetes API
   - For each resource:
     ```go
     // Build Arrow record
     bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
     bldr.Field(...).(*array.StringBuilder).Append(...)
     bldr.Field(...).(*types.UUIDBuilder).Append(...)
     bldr.Field(...).(*array.TimestampBuilder).Append(...)
     
     // Emit SyncInsert message
     res <- &message.SyncInsert{Record: bldr.NewRecord()}
     ```

5. **CloudQuery CLI receives messages:**
   - Deserializes Arrow records
   - Routes to destination plugin (PostgreSQL)

6. **PostgreSQL destination:**
   - Creates tables if needed (forced migration)
   - Inserts/updates rows from Arrow records
   - Applies ON CONFLICT DO UPDATE logic

7. **Plugin closes** cleanly after all contexts and resources processed

## Data Model

### Cluster Metadata
```sql
CREATE TABLE k8s_clusters (
  context_name TEXT PRIMARY KEY,
  cluster_name TEXT,
  server TEXT,
  ca_file TEXT,
  insecure_skip_verify BOOLEAN,
  namespace TEXT,
  kubernetes_version TEXT,
  node_count INTEGER,
  synced_at TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE,
  updated_at TIMESTAMP WITH TIME ZONE
);
```

### Resource Tables
All tables follow this pattern:

```sql
CREATE TABLE k8s_<resource> (
    id UUID PRIMARY KEY,  -- Kubernetes object UID (Arrow ExtensionType)
    name TEXT,
    namespace TEXT,
    created_at TIMESTAMP WITH TIME ZONE,
    -- resource-specific fields
);
```

**Cluster Relationships:**
- Each namespace/pod/deployment/service belongs to a context (cluster)
- Query example:
  ```sql
  SELECT 
    c.context_name,
    c.kubernetes_version,
    n.name as namespace,
    p.name as pod
  FROM k8s_clusters c
  JOIN k8s_namespaces n ON c.context_name = n.context_name
  JOIN k8s_pods p ON n.name = p.namespace AND c.context_name = p.context_name
  ORDER BY c.context_name, n.name, p.name;
  ```

## Dependencies

### Go Modules
- `github.com/cloudquery/plugin-sdk/v4` (v4.94.1) — gRPC plugin framework with Arrow support
- `github.com/apache/arrow-go/v18` (v18.5.0) — Apache Arrow serialization
- `github.com/google/uuid` — UUID parsing for Arrow records
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `k8s.io/client-go` (v0.35.0) — Kubernetes API client
- `k8s.io/apiextensions-apiserver` — CRD discovery
- `github.com/rs/zerolog` — Structured logging
- `gopkg.in/yaml.v3` — YAML config parsing

### External Services
- **PostgreSQL v8.14.0+** — Destination for Arrow data (via cloudquery-postgresql plugin)
- **Kubernetes clusters** — Resource discovery (via kubeconfig)
- **~/.kube/config** — Kubernetes context configuration
- **CloudQuery CLI v6.34.0+** — Orchestration and message routing

## Implementation Details: SyncInsert Message Pattern

### Arrow Record Builder Flow
```go
// 1. Get table schema with Arrow types
table := PodsTable()  // Contains column definitions with arrow.FixedWidthTypes.Timestamp_ns, etc.

// 2. Create record builder with memory allocator
bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
defer bldr.Release()

// 3. Append each field with proper type builder
idx := table.Columns.Index("id")
bldr.Field(idx).(*types.UUIDBuilder).Append(uuid.Parse(...))

idx = table.Columns.Index("name")
bldr.Field(idx).(*array.StringBuilder).Append("pod-name")

idx = table.Columns.Index("created_at")
bldr.Field(idx).(*array.TimestampBuilder).Append(arrow.Timestamp(time.Now().UnixNano()))

// 4. Build and emit record
rec := bldr.NewRecord()
res <- &message.SyncInsert{Record: rec}
```

### Type Builder Mapping
| Arrow Type | Builder Class | Go Value |
|-----------|---------------|----------|
| String | `array.StringBuilder` | `string` |
| Int64 | `array.Int64Builder` | `int64` |
| Boolean | `array.BooleanBuilder` | `bool` |
| Timestamp_ns | `array.TimestampBuilder` | `arrow.Timestamp(int64)` |
| UUID (ExtensionType) | `types.UUIDBuilder` | `uuid.UUID` |

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

- **Arrow serialization:** Columnar format is efficient for bulk inserts
- **UUID handling:** ExtensionType with FixedSizeBinary backing
- **Timestamp precision:** Nanosecond precision (Timestamp_ns) for accuracy
- **Memory efficiency:** RecordBuilder defers cleanup via defer bldr.Release()
- **API rate limiting:** Respects Kubernetes API quotas
- **Batch processing:** Arrow records are naturally batched

## Security Considerations

1. **kubeconfig** — Must have read access to `~/.kube/config`
2. **RBAC** — Requires read permissions on all monitored resources
3. **Database credentials** — Pass via `cloudquery_destination.yml` (consider secrets management)
4. **Network** — PostgreSQL connection should be over TLS in production
5. **Plugin isolation** — CloudQuery plugin runs with minimal privileges

## Debugging

### Enable verbose logging:
```zsh
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml --log-level debug
```

### Check plugin output:
```zsh
./bin/plugin 2>&1 | head -50
```

### Verify Kubernetes access:
```zsh
kubectl --context=dev get all -A
kubectl --context=prod get all -A
```

### Query Postgres directly:
```zsh
psql -U postgres k8s -c "SELECT context_name, kubernetes_version, node_count FROM k8s_clusters;"
psql -U postgres k8s -c "SELECT name, namespace, status FROM k8s_pods LIMIT 5;"
```

### Check Arrow record serialization:
Monitor CloudQuery CLI output for `Resources: XX, Errors: 0` confirmation of successful message emission.
