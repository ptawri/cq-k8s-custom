package plugin

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Genos0820/cq-k8s-custom/internal"
)

func fetchServices(
	ctx context.Context,
	meta schema.ClientMeta,
	parent *schema.Resource,
	res chan<- interface{},
) error {
	client := meta.(*internal.Client).Clientset

	services, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, service := range services.Items {
		res <- map[string]interface{}{
			"id":         string(service.UID),
			"name":       service.Name,
			"namespace":  service.Namespace,
			"type":       string(service.Spec.Type),
			"cluster_ip": service.Spec.ClusterIP,
			"created_at": service.CreationTimestamp.Time,
		}
	}

	return nil
}
