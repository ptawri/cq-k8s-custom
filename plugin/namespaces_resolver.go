package plugin

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Genos0820/cq-k8s-custom/internal"
)

func fetchNamespaces(
	ctx context.Context,
	meta schema.ClientMeta,
	parent *schema.Resource,
	res chan<- interface{},
) error {

	client := meta.(*internal.Client).Clientset

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		res <- map[string]interface{}{
			"id":         string(ns.UID),
			"name":       ns.Name,
			"status":     string(ns.Status.Phase),
			"created_at": ns.CreationTimestamp.Time,
		}
	}

	return nil
}
