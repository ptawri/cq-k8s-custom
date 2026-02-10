# Quick Start Guide

Get the CloudQuery Kubernetes plugin running in 5 minutes.

## 1. Prerequisites

```zsh
# Verify CloudQuery CLI is installed
which cloudquery

# Verify minikube clusters are running
minikube profile list

# Verify Kubernetes contexts
kubectl config get-contexts

# Verify PostgreSQL is running (or start it)
brew services start postgresql
# or: docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:latest

# Create k8s database
createdb k8s -U postgres
```

## 2. Build the Plugin

```zsh
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom
go build -o ./bin/plugin ./cmd/plugin
```

Verify:
```zsh
ls -lh bin/plugin
```

## 3. Run Your First Sync

```zsh
# Option A: Using the repo configs (includes both dev and prod)
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml

# Option B: Create your own configs
cat > my_source.yml << 'EOF'
kind: source
spec:
  name: k8s-custom
  registry: local
  path: ./bin/plugin
  spec:
    database_url: postgres://postgres:postgres@localhost:5432/k8s?sslmode=disable
    contexts:
      - dev
      - prod
    resources:
      - namespaces
      - pods
EOF

cat > my_destination.yml << 'EOF'
kind: destination
spec:
  name: postgres
  registry: cloudquery
  path: postgresql
  spec:
    connection_string: postgres://postgres:postgres@localhost:5432/k8s?sslmode=disable
EOF

cloudquery sync my_source.yml my_destination.yml
```

## 4. Verify Data

```sql
-- Check what was synced
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'k8s_%';

-- Count resources by cluster
SELECT COUNT(*), context_name FROM k8s_namespaces GROUP BY context_name;
SELECT COUNT(*), context_name FROM k8s_pods GROUP BY context_name;
```

## 5. Next Steps

- **Test filtering:** Edit config to include only specific contexts/resources
- **Monitor changes:** Run sync again and see updates in Postgres
- **Explore data:** Use SQL queries to analyze Kubernetes state
- **Extend:** Add more resource types or destinations

## Troubleshooting

### Plugin won't build
```zsh
go mod tidy  # Update dependencies
```

### PostgreSQL connection error
```zsh
# Check if Postgres is running
brew services list | grep postgresql

# Test connection
psql -U postgres -c "SELECT 1"

# Verify database exists
createdb k8s -U postgres
```

### Kubernetes context not found
```zsh
# Verify contexts
kubectl config get-contexts

# Ensure both dev and prod exist
minikube profile list
```

### No data synced
- Add `--log-level debug` to sync command to see detailed logs
- Verify resources exist: `kubectl --context=dev get all -A`
- Check Postgres schema was created: `psql -U postgres k8s -c "\dt"`

## Documentation

- **README.md** — Overview and build/run instructions
- **ARCHITECTURE.md** — Deep dive into plugin architecture and design
- **TESTING.md** — Comprehensive testing guide
- **PLUGIN_SUMMARY.md** — Summary of plugin capabilities and extension ideas

## File Structure

```
cq-k8s-custom/
├── bin/
│   └── plugin              # Compiled binary (82MB)
├── cmd/
│   └── plugin/
│       └── main.go         # Entry point (serves CloudQuery plugin)
├── plugin/
│   ├── plugin.go           # Plugin registration
│   ├── source_client.go    # Main sync logic (CloudQuery SourceClient)
│   ├── resources_tables.go # Table schemas
│   └── *_resolver.go       # Resource fetchers (legacy)
├── internal/
│   ├── client.go           # Kubernetes client (multi-context)
│   └── db.go               # PostgreSQL persistence
├── go.mod / go.sum         # Dependencies
├── ARCHITECTURE.md         # System design
├── TESTING.md              # Test procedures
├── README.md               # Build/run guide
├── PLUGIN_SUMMARY.md       # Feature summary
├── cloudquery_sync.yml     # Source config
└── cloudquery_destination.yml # Destination config
```

## Common Commands

```zsh
# Build
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom && go build -o ./bin/plugin ./cmd/plugin

# Sync (default: dev + prod contexts, all resources)
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml

# Sync with debug output
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml --log-level debug

# Check synced data
psql -U postgres k8s -c "SELECT * FROM k8s_pods LIMIT 5;"

# List all k8s tables
psql -U postgres k8s -c "\dt k8s_*"
```

## What Gets Synced

By default, the plugin syncs:
- **Contexts:** dev, prod (from minikube)
- **Resources:**
  - Namespaces
  - Pods
  - Deployments
  - Services
  - Custom Resource Definitions (CRDs)

Tables created in PostgreSQL:
- `k8s_clusters`
- `k8s_namespaces`
- `k8s_pods`
- `k8s_deployments`
- `k8s_services`
- `k8s_custom_resources`

Each row includes:
- `context_name` — Which cluster (dev, prod, etc.)
- `uid` — Unique Kubernetes object ID
- `name`, `namespace` — Resource identifiers
- `created_at` — Creation timestamp
- Resource-specific fields (replicas, ports, etc.)

---

**Ready to sync?** Run `cloudquery sync cloudquery_sync.yml cloudquery_destination.yml` and watch your Kubernetes data flow into Postgres! 🚀
