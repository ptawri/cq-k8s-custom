# PostgreSQL & Data Sync - Complete! ✅

## Status: All Kubernetes Data Successfully Synced to PostgreSQL

### Container Status
- **PostgreSQL Image**: postgres:15
- **Container Name**: cloudquery-postgres
- **Port**: 5434
- **Database**: k8s
- **Status**: Running ✅

### Data Synced

| Table | Dev Cluster | Prod Cluster | Total |
|-------|-------------|--------------|-------|
| **Namespaces** | 4 | 4 | **8** |
| **Pods** | 11 | 11 | **22** |
| **Deployments** | 2 | 3 | **5** |
| **Services** | 3 | 3 | **6** |
| **CRDs** | 0 | 0 | **0** |
| **TOTAL** | **20** | **21** | **41** |

### Quick Commands

#### Connect to Database
```bash
docker exec cloudquery-postgres psql -U postgres k8s
```

#### Check Table Schema
```bash
docker exec cloudquery-postgres psql -U postgres k8s -c "\d k8s_pods"
```

#### Query Examples
```sql
-- Count pods by cluster
SELECT context_name, COUNT(*) as pod_count FROM k8s_pods GROUP BY context_name;

-- All pods in kube-system
SELECT name, context_name FROM k8s_pods WHERE namespace = 'kube-system';

-- All deployments
SELECT context_name, name, namespace FROM k8s_deployments ORDER BY context_name;

-- Summary of all resources
SELECT 'Namespaces' as type, COUNT(*) FROM k8s_namespaces
UNION ALL
SELECT 'Pods', COUNT(*) FROM k8s_pods
UNION ALL
SELECT 'Deployments', COUNT(*) FROM k8s_deployments
UNION ALL
SELECT 'Services', COUNT(*) FROM k8s_services;
```

#### Sync Data Again
```bash
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom
./bin/test-sync  # Runs the plugin directly

# Or with CloudQuery:
cloudquery sync cloudquery_sync.yml
```

### Configuration File
**Location**: `cloudquery_sync.yml`

```yaml
sources:
  - name: k8s-custom
    path: ./bin/plugin
    registry: local
    spec:
      database_url: postgres://postgres:postgres@localhost:5434/k8s?sslmode=disable
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

### How It Works

1. **Plugin discovers** Kubernetes contexts from `~/.kube/config` (dev, prod)
2. **Connects to each cluster** via Kubernetes API
3. **Fetches all resources** (namespaces, pods, deployments, services, CRDs)
4. **Stores in PostgreSQL** with composite key `(context_name, uid)` for multi-cluster isolation
5. **Upserts prevent duplicates** - safe to run multiple times

### Database Schema

All tables use composite primary keys: `(context_name, uid)`

```sql
-- Example: k8s_pods table
CREATE TABLE k8s_pods (
    context_name TEXT NOT NULL,      -- 'dev' or 'prod'
    uid TEXT NOT NULL,               -- Kubernetes object UID
    name TEXT,                        -- Pod name
    namespace TEXT,                   -- Namespace
    phase TEXT,                       -- Pod phase (Running, Pending, etc)
    node_name TEXT,                   -- Node the pod is running on
    created_at TIMESTAMP,             -- Creation time
    PRIMARY KEY (context_name, uid)
);
```

### Next Steps

1. **Monitor changes**: Run `./bin/test-sync` on a schedule (cron, CI/CD, etc.)
2. **Build dashboards**: Use the data for reporting and visualization
3. **Integrate with CloudQuery**: Combine with AWS, GCP, Azure sources
4. **Query and analyze**: SQL queries on Kubernetes infrastructure state

### Files Created

- `cloudquery_sync.yml` - CloudQuery config (source only)
- `cmd/test-sync/main.go` - Standalone sync utility (runs plugin directly)
- `bin/test-sync` - Compiled sync binary

### Test Sync Program

Built a utility program that runs the plugin directly:

```bash
# Build
go build -o ./bin/test-sync ./cmd/test-sync/main.go

# Run
./bin/test-sync

# Output shows:
# - 5 tables discovered
# - Schema migration messages
# - Data fetched from both clusters
```

---

**Your Kubernetes infrastructure is now queryable in PostgreSQL!** 🎉
