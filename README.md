# CloudQuery Kubernetes Custom Plugin

## Overview
This is a custom CloudQuery source plugin that queries multiple Kubernetes contexts from your kubeconfig and lists core resources plus CRDs. It currently prints results to stdout and exposes tables to CloudQuery.

## Resources Supported
- Namespaces
- Pods
- Deployments
- Services
- CustomResourceDefinitions (CRDs)

## Tables Exposed
- `k8s_namespaces`
- `k8s_pods`
- `k8s_deployments`
- `k8s_services`
- `k8s_custom_resources`

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

This plugin now integrates with CloudQuery and can be used via the `cloudquery` CLI. Create a `config.yml`:

```yaml
sources:
  - name: k8s-custom
    path: local
    registry: local
    spec:
      database_url: postgres://user:password@localhost:5432/k8s?sslmode=disable
      contexts:
        - dev
        - prod
      resources:
        - namespaces
        - pods
        - deployments
        - services
        - crds

destinations:
  - name: postgres
    path: cloudquery/postgresql
    spec:
      connection_string: postgres://user:password@localhost:5432/k8s?sslmode=disable
```

Then sync:

```zsh
cloudquery sync config.yml
```

This will run the plugin and automatically persist Kubernetes data to Postgres.

## Notes
- The plugin discovers all Kubernetes contexts from `~/.kube/config` and iterates through them.
- Contexts that are not running will print connection errors and continue.

## Extension Ideas
- Workloads: StatefulSets, DaemonSets, Jobs, CronJobs
- Config: ConfigMaps, Secrets, ResourceQuotas, LimitRanges
- Networking: Ingresses, NetworkPolicies, EndpointSlices
- Storage: PVs, PVCs, StorageClasses
- Security: RBAC roles/rolebindings, ServiceAccounts
- CRDs: enumerate custom resources (not only CRDs) via dynamic client
