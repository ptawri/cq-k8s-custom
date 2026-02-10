package plugin

import (
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/types"
)

func ClustersTable() *schema.Table {
	return &schema.Table{
		Name: "k8s_clusters",
		Columns: []schema.Column{
			{Name: "context_name", Type: arrow.BinaryTypes.String, PrimaryKey: true},
			{Name: "cluster_name", Type: arrow.BinaryTypes.String},
			{Name: "server", Type: arrow.BinaryTypes.String},
			{Name: "ca_file", Type: arrow.BinaryTypes.String},
			{Name: "insecure_skip_verify", Type: arrow.FixedWidthTypes.Boolean},
			{Name: "namespace", Type: arrow.BinaryTypes.String},
			{Name: "kubernetes_version", Type: arrow.BinaryTypes.String},
			{Name: "node_count", Type: arrow.PrimitiveTypes.Int64},
			{Name: "synced_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
			{Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
			{Name: "updated_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
		},
	}
}

func PodsTable() *schema.Table {
	return &schema.Table{
		Name:     "k8s_pods",
		Resolver: fetchPods,
		Columns: []schema.Column{
			{Name: "id", Type: types.ExtensionTypes.UUID, PrimaryKey: true},
			{Name: "name", Type: arrow.BinaryTypes.String},
			{Name: "namespace", Type: arrow.BinaryTypes.String},
			{Name: "status", Type: arrow.BinaryTypes.String},
			{Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
		},
	}
}

func DeploymentsTable() *schema.Table {
	return &schema.Table{
		Name:     "k8s_deployments",
		Resolver: fetchDeployments,
		Columns: []schema.Column{
			{Name: "id", Type: types.ExtensionTypes.UUID, PrimaryKey: true},
			{Name: "name", Type: arrow.BinaryTypes.String},
			{Name: "namespace", Type: arrow.BinaryTypes.String},
			{Name: "replicas", Type: arrow.PrimitiveTypes.Int64},
			{Name: "ready", Type: arrow.PrimitiveTypes.Int64},
			{Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
		},
	}
}

func ServicesTable() *schema.Table {
	return &schema.Table{
		Name:     "k8s_services",
		Resolver: fetchServices,
		Columns: []schema.Column{
			{Name: "id", Type: types.ExtensionTypes.UUID, PrimaryKey: true},
			{Name: "name", Type: arrow.BinaryTypes.String},
			{Name: "namespace", Type: arrow.BinaryTypes.String},
			{Name: "type", Type: arrow.BinaryTypes.String},
			{Name: "cluster_ip", Type: arrow.BinaryTypes.String},
			{Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
		},
	}
}

func CustomResourcesTable() *schema.Table {
	return &schema.Table{
		Name:     "k8s_custom_resources",
		Resolver: fetchCustomResources,
		Columns: []schema.Column{
			{Name: "id", Type: types.ExtensionTypes.UUID, PrimaryKey: true},
			{Name: "name", Type: arrow.BinaryTypes.String},
			{Name: "group", Type: arrow.BinaryTypes.String},
			{Name: "kind", Type: arrow.BinaryTypes.String},
			{Name: "plural", Type: arrow.BinaryTypes.String},
			{Name: "scope", Type: arrow.BinaryTypes.String},
			{Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ns},
		},
	}
}
