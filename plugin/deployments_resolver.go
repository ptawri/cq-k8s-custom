package plugin

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Genos0820/cq-k8s-custom/internal"
)

func fetchDeployments(
	ctx context.Context,
	meta schema.ClientMeta,
	parent *schema.Resource,
	res chan<- interface{},
) error {
	client := meta.(*internal.Client).Clientset

	deployments, err := client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		res <- map[string]interface{}{
			"id":         string(deployment.UID),
			"name":       deployment.Name,
			"namespace":  deployment.Namespace,
			"replicas":   deployment.Status.Replicas,
			"ready":      deployment.Status.ReadyReplicas,
			"created_at": deployment.CreationTimestamp.Time,
		}
	}

	return nil
}
