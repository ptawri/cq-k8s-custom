package plugin

import (
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/types"
)

func PodsTable() *schema.Table {
	return &schema.Table{
		Name:     "k8s_pods",
		Resolver: fetchPods,
		Columns: []schema.Column{
			{Name: "id", Type: types.ExtensionTypes.UUID, PrimaryKey: true},
			{Name: "name", Type: arrow.BinaryTypes.String},
			{Name: "namespace", Type: arrow.BinaryTypes.String},
			{Name: "status", Type: arrow.BinaryTypes.String},
			{Name: "created_at", Type: arrow.StructOf()},
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
			{Name: "replicas", Type: arrow.BinaryTypes.String},
			{Name: "ready", Type: arrow.BinaryTypes.String},
			{Name: "created_at", Type: arrow.StructOf()},
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
			{Name: "created_at", Type: arrow.StructOf()},
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
			{Name: "created_at", Type: arrow.StructOf()},
		},
	}
}
