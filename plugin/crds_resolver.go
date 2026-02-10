package plugin

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Genos0820/cq-k8s-custom/internal"
)

func fetchCustomResources(
	ctx context.Context,
	meta schema.ClientMeta,
	parent *schema.Resource,
	res chan<- interface{},
) error {
	client := meta.(*internal.Client).ApiextensionsClientset

	// Get all CustomResourceDefinitions
	crds, err := client.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, crd := range crds.Items {
		res <- map[string]interface{}{
			"id":         string(crd.UID),
			"name":       crd.Name,
			"group":      crd.Spec.Group,
			"kind":       crd.Spec.Names.Kind,
			"plural":     crd.Spec.Names.Plural,
			"scope":      string(crd.Spec.Scope),
			"created_at": crd.CreationTimestamp.Time,
		}
	}

	return nil
}
