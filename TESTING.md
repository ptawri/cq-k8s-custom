# Testing the CloudQuery Kubernetes Plugin

This document describes how to test the CloudQuery Kubernetes plugin end-to-end.

## Prerequisites

1. **CloudQuery CLI** installed
   ```zsh
   which cloudquery
   ```

2. **minikube** clusters running (dev and prod)
   ```zsh
   minikube profile list
   ```
   Expected output: Both `dev` and `prod` should have status `OK`

3. **PostgreSQL** running and accessible at `localhost:5432`
   ```zsh
   # macOS - start with Homebrew
   brew services start postgresql
   # or use Docker
   docker run -d -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:latest
   ```

4. **k8s database** created in PostgreSQL
   ```zsh
   createdb k8s -U postgres
   # or via Docker psql
   docker exec <postgres_container> createdb k8s -U postgres
   ```

## Test Steps

### 1. Build the Plugin

```zsh
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom
go build -o ./bin/plugin ./cmd/plugin
```

Verify the binary was created:
```zsh
ls -lh ./bin/plugin
```

### 2. Verify Kubernetes Contexts

Ensure both minikube clusters are available:
```zsh
kubectl config get-contexts
```

Expected output:
```
CURRENT   NAME      CLUSTER   AUTHINFO   NAMESPACE
          dev       minikube  minikube   
*         prod      minikube  minikube   
```

### 3. Verify Kubernetes Resources

Check that resources exist in both clusters:

```zsh
# Dev cluster
kubectl --context=dev get ns
kubectl --context=dev get pods -A
kubectl --context=dev get deployments -A
kubectl --context=dev get services -A

# Prod cluster
kubectl --context=prod get ns
kubectl --context=prod get pods -A
kubectl --context=prod get deployments -A
kubectl --context=prod get services -A
```

### 4. Test Plugin via CloudQuery Sync

The plugin is configured in `cloudquery_sync.yml` and `cloudquery_destination.yml`. Review them:

```zsh
cat cloudquery_sync.yml
cat cloudquery_destination.yml
```

Run the sync:
```zsh
cd /Users/prajjwaltawri/Desktop/k8cloudquery/cq-k8s-custom
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml --log-level debug
```

Monitor for:
- ✅ Plugin starts and connects
- ✅ Tables are discovered (k8s_namespaces, k8s_pods, k8s_deployments, k8s_services, k8s_custom_resources)
- ✅ Data is fetched from both `dev` and `prod` contexts
- ✅ Resources are inserted into PostgreSQL
- ✅ Sync completes successfully

### 5. Verify Data in PostgreSQL

After a successful sync, verify the data was persisted:

```zsh
# Connect to the k8s database
psql -U postgres k8s

# Check table contents
\dt k8s_*
SELECT COUNT(*), context_name FROM k8s_namespaces GROUP BY context_name;
SELECT COUNT(*), context_name FROM k8s_pods GROUP BY context_name;
SELECT COUNT(*), context_name FROM k8s_deployments GROUP BY context_name;
SELECT COUNT(*), context_name FROM k8s_services GROUP BY context_name;
SELECT COUNT(*), context_name FROM k8s_custom_resources GROUP BY context_name;

# Exit psql
\q
```

Expected output example:
```
k8s_namespaces data:
 count | context_name
-------+--------------
     4 | dev
     4 | prod
```

### 6. Test Filtering (Optional)

Edit `cloudquery_sync.yml` to test context/resource filtering:

```yaml
spec:
   name: k8s-custom
   registry: local
   path: ./bin/plugin
   spec:
      database_url: postgres://postgres:postgres@localhost:5432/k8s?sslmode=disable
      contexts:
         - dev    # Only sync dev cluster
      resources:
         - namespaces  # Only sync namespaces
         - pods
```

Re-run the sync:
```zsh
cloudquery sync cloudquery_sync.yml cloudquery_destination.yml
```

Verify only filtered resources are synced.

## Troubleshooting

### Plugin fails to start
```zsh
./bin/plugin
# Check for errors in output
```

### PostgreSQL connection errors
- Verify PostgreSQL is running: `brew services list` (macOS)
- Verify database exists: `createdb k8s -U postgres`
- Test connection: `psql -U postgres -h localhost -d k8s -c "SELECT 1;"`

### Kubernetes context errors
```zsh
kubectl config get-contexts
kubectl config use-context dev
kubectl cluster-info
```

### No data synced
- Check CloudQuery logs: `--log-level debug`
- Verify Kubernetes resources exist: `kubectl get all -A`
- Verify table schema was created: `psql -U postgres k8s -c "\dt k8s_*"`

## Notes

- Each sync run will INSERT or UPDATE resources in PostgreSQL
- Data is uniquely identified by `(context_name, uid)` to support multi-cluster scenarios
- Contexts are auto-discovered from `~/.kube/config` but can be filtered via config spec
- The plugin runs as a CloudQuery source and can be combined with other sources in one sync

## Next Steps

Once testing is complete:

1. Deploy the plugin to a CloudQuery Hub registry
2. Use in production CloudQuery syncs
3. Combine with other sources (AWS, GCP, Azure, etc.)
4. Monitor Kubernetes changes over time via Postgres querying
