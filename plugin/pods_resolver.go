package plugin

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Genos0820/cq-k8s-custom/internal"
)

func fetchPods(
	ctx context.Context,
	meta schema.ClientMeta,
	parent *schema.Resource,
	res chan<- interface{},
) error {
	client := meta.(*internal.Client).Clientset

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		res <- map[string]interface{}{
			"id":         string(pod.UID),
			"name":       pod.Name,
			"namespace":  pod.Namespace,
			"status":     string(pod.Status.Phase),
			"created_at": pod.CreationTimestamp.Time,
		}
	}

	return nil
}
