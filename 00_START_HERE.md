# 🚀 CloudQuery Kubernetes Plugin - START HERE

Welcome! This is a complete, production-ready CloudQuery source plugin for monitoring Kubernetes clusters.

## What This Does

- **Discovers** all Kubernetes resources (pods, deployments, services, namespaces, CRDs)
- **Supports multiple clusters** (dev, prod, or any kubeconfig contexts)
- **Persists to PostgreSQL** on every CloudQuery sync
- **Integrates with CloudQuery** for enterprise data integration workflows

## Quick Setup (5 minutes)

### 1. Prerequisites
```zsh
# Check everything is ready
which cloudquery           # CloudQuery CLI
minikube profile list      # Verify dev/prod running
kubectl config get-contexts # Verify K8s access
```

### 2. Start PostgreSQL
```zsh
# Option A: Homebrew (macOS)
brew services start postgresql

# Option B: Docker
docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres

# Create database
createdb k8s -U postgres
```

### 3. Build & Run
```zsh
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom

# Build plugin (one-time)
go build -o ./bin/plugin ./cmd/plugin

# Run sync
cloudquery sync cloudquery_test.yml

# Verify data in Postgres
psql -U postgres k8s -c "SELECT COUNT(*), context_name FROM k8s_pods GROUP BY context_name;"
```

✅ **That's it!** Your Kubernetes data is now in Postgres.

---

## 📚 Documentation

Start with these based on what you need:

| Document | Purpose | Read Time |
|----------|---------|-----------|
| **QUICKSTART.md** | Get running in 5 minutes | 3 min |
| **ARCHITECTURE.md** | Understand how it works | 10 min |
| **TESTING.md** | Comprehensive test guide | 8 min |
| **README.md** | Build & deployment | 5 min |
| **PLUGIN_SUMMARY.md** | Feature overview | 3 min |
| **PROJECT_STATUS.md** | Current status & roadmap | 5 min |

---

## 🎯 What's Included

### Core Components ✅
- **Plugin Binary** (82MB) — Ready to deploy
- **Source Code** (700+ lines of Go)
  - Multi-cluster K8s client
  - CloudQuery plugin framework integration
  - PostgreSQL persistence layer
  - Configuration management
- **CloudQuery Config** — Example `cloudquery_test.yml`
- **Complete Documentation** — 6 guides covering everything

### Supported Resources
1. Namespaces
2. Pods
3. Deployments
4. Services
5. Custom Resource Definitions (CRDs)

### Supported Clusters
- Dev (minikube profile)
- Prod (minikube profile)
- Any Kubernetes context in `~/.kube/config`

---

## 🔧 Configuration

### Default Config (both clusters, all resources)
```yaml
# cloudquery_test.yml
sources:
  - name: k8s-custom
    path: ./bin/plugin
    spec:
      database_url: postgres://postgres:postgres@localhost:5432/k8s
      contexts: [dev, prod]
      resources: [namespaces, pods, deployments, services, crds]
```

### Customize for Your Needs
Edit the config to filter:
- **Contexts**: only specific clusters
- **Resources**: only certain resource types
- **Database**: different PostgreSQL instance

---

## 📊 How It Works

```
User runs: cloudquery sync cloudquery_test.yml
                    ↓
    Plugin loads config & connects to PostgreSQL
                    ↓
    For each cluster (dev, prod):
      For each resource type (pods, deployments, ...):
        - Fetch from Kubernetes API
        - Store in PostgreSQL
                    ↓
    Sync complete, data ready for querying
```

---

## 🧪 Testing

### Verify Everything Works
1. Run the sync: `cloudquery sync cloudquery_test.yml`
2. Check for errors in output
3. Query the data:
   ```sql
   SELECT table_name FROM information_schema.tables 
   WHERE table_name LIKE 'k8s_%';
   ```

### Troubleshooting
- **PostgreSQL won't connect?** → Check it's running and database exists
- **Kubernetes errors?** → Verify contexts: `kubectl config get-contexts`
- **Plugin won't build?** → Run `go mod tidy`

See **TESTING.md** for comprehensive troubleshooting.

---

## 📁 Project Structure

```
cq-k8s-custom/
├── bin/plugin                 # Compiled binary (run this)
├── cmd/plugin/main.go         # Entry point for CloudQuery
├── plugin/
│   ├── source_client.go       # Main sync logic (276 lines) ⭐
│   ├── plugin.go              # CloudQuery registration
│   └── resources_tables.go    # Table schemas
├── internal/
│   ├── client.go              # Kubernetes client
│   └── db.go                  # PostgreSQL layer
├── 00_START_HERE.md           # This file
├── QUICKSTART.md              # 5-minute guide
├── ARCHITECTURE.md            # Deep technical dive
├── TESTING.md                 # Test procedures
├── README.md                  # Build instructions
├── PLUGIN_SUMMARY.md          # Feature summary
└── cloudquery_test.yml        # Example config
```

---

## ✨ Next Steps

### For Quick Testing
1. Follow **QUICKSTART.md** (3 pages)
2. Run `cloudquery sync cloudquery_test.yml`
3. Query Postgres to verify data

### For Understanding the Design
1. Read **ARCHITECTURE.md** for system overview
2. Browse the code in `plugin/source_client.go` (main logic)
3. Check `internal/db.go` for how data is stored

### For Production Deployment
1. Review **TESTING.md** for comprehensive validation
2. Customize `cloudquery_test.yml` for your clusters
3. Set up credentials/secrets management for database URL
4. Integrate with your CloudQuery Hub account

---

## 🚀 You're Ready!

```zsh
# One command to get started:
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom && \
go build -o ./bin/plugin ./cmd/plugin && \
cloudquery sync cloudquery_test.yml
```

Then query your Kubernetes data:
```sql
psql -U postgres k8s
# Inside psql:
SELECT * FROM k8s_pods LIMIT 5;
SELECT context_name, COUNT(*) FROM k8s_namespaces GROUP BY context_name;
```

---

## 💡 Quick Reference

| Task | Command |
|------|---------|
| Build plugin | `go build -o ./bin/plugin ./cmd/plugin` |
| Run sync | `cloudquery sync cloudquery_test.yml` |
| Check Postgres | `psql -U postgres k8s` |
| View K8s contexts | `kubectl config get-contexts` |
| Check minikube clusters | `minikube profile list` |
| Debug sync | `cloudquery sync cloudquery_test.yml --log-level debug` |

---

## 📞 Support

- **Build issues?** → Run `go mod tidy` then rebuild
- **PostgreSQL issues?** → Check `TESTING.md` prerequisites section
- **Kubernetes issues?** → Verify contexts and permissions
- **Config issues?** → Review `cloudquery_test.yml` example
- **Deep dive?** → Read `ARCHITECTURE.md`

---

**Ready? Start with QUICKSTART.md or run your first sync now!** 🎉
