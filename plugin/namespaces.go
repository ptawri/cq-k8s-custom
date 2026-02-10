package plugin

import (
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/types"
)

func NamespacesTable() *schema.Table {
	return &schema.Table{
		Name:        "k8s_namespaces",
		Description: "Kubernetes namespaces",
		Resolver:    fetchNamespaces,
		Columns: []schema.Column{
			{
				Name:       "id",
				Type:       types.ExtensionTypes.UUID,
				PrimaryKey: true,
			},
			{
				Name: "name",
				Type: arrow.BinaryTypes.String,
			},
			{
				Name: "status",
				Type: arrow.BinaryTypes.String,
			},
			{
				Name: "created_at",
				Type: arrow.FixedWidthTypes.Timestamp_ns,
			},
		},
	}
}
