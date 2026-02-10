# Kubernetes CloudQuery Custom Plugin Summary

## Overview
This plugin connects to all Kubernetes contexts found in your local kubeconfig and enumerates resources per context. It currently targets both standard Kubernetes resources and CRDs, and prints results per context to the console.

## What It Currently Does
- Discovers all kubeconfig contexts and iterates through them.
- Connects to each context and queries resources.
- Handles unavailable contexts gracefully by printing connection errors.

## Resources Collected
- **Namespaces**: name and status.
- **Pods**: name, namespace, and phase.
- **Deployments**: name, namespace, ready/desired replicas.
- **Services**: name, namespace, type, and cluster IP.
- **CRDs (CustomResourceDefinitions)**: name, kind, and scope.

## Tables Exposed (CloudQuery)
- `k8s_namespaces`
- `k8s_pods`
- `k8s_deployments`
- `k8s_services`
- `k8s_custom_resources`

## Summary
This plugin is a multi-cluster Kubernetes source that reads all contexts from kubeconfig and extracts core and custom resources. It is already wired to the CloudQuery plugin SDK and can be extended with additional resource tables or deeper schema for existing resources.

## Extension Ideas
You can extend this plugin to include:
- **Workloads**: StatefulSets, DaemonSets, Jobs, CronJobs, ReplicaSets
- **Configuration**: ConfigMaps, Secrets, ResourceQuotas, LimitRanges
- **Networking**: Ingresses, NetworkPolicies, Endpoints, EndpointSlices
- **Storage**: PersistentVolumes, PersistentVolumeClaims, StorageClasses
- **Security/Policy**: PodSecurityPolicies (if enabled), RBAC roles/rolebindings, service accounts
- **Cluster Resources**: Nodes, API Services, PriorityClasses
- **CRDs Data**: Custom resources (not just CRDs) by dynamically discovering GVRs and querying instances
- **Metadata**: labels, annotations, owner references, and managed fields
- **Metrics**: integrate metrics-server or Prometheus endpoints for live usage data

## Notes
- For multi-cluster environments, consider adding filtering by context name via flags or config.
- You can switch from console output to direct CloudQuery ingestion once you wire table resolvers fully into the SDK’s source plugin flow.
