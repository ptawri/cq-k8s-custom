package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/Genos0820/cq-k8s-custom/internal"
	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	DatabaseURL string   `json:"database_url"`
	Contexts    []string `json:"contexts"`
	Resources   []string `json:"resources"`
}

type SourceClient struct {
	logger         zerolog.Logger
	store          *internal.Store
	contextFilter  map[string]struct{}
	resourceFilter map[string]struct{}
}

func NewSourceClient(ctx context.Context, logger zerolog.Logger, spec any) (plugin.SourceClient, error) {
	cfg, err := loadConfig(spec)
	if err != nil {
		return nil, err
	}

	store, err := internal.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureSchema(ctx); err != nil {
		store.Close()
		return nil, err
	}

	return &SourceClient{
		logger:         logger,
		store:          store,
		contextFilter:  sliceToSet(cfg.Contexts),
		resourceFilter: sliceToSet(cfg.Resources),
	}, nil
}

func (c *SourceClient) Close(ctx context.Context) error {
	if c.store != nil {
		c.store.Close()
	}
	return nil
}

func (c *SourceClient) Tables(ctx context.Context, options plugin.TableOptions) (schema.Tables, error) {
	return schema.Tables{
		ClustersTable(),
		NamespacesTable(),
		PodsTable(),
		DeploymentsTable(),
		ServicesTable(),
		CustomResourcesTable(),
	}, nil
}

func (c *SourceClient) Sync(ctx context.Context, options plugin.SyncOptions, res chan<- message.SyncMessage) error {
	tables, err := c.Tables(ctx, plugin.TableOptions{})
	if err != nil {
		return err
	}

	for _, table := range tables {
		res <- &message.SyncMigrateTable{Table: table}
	}

	contexts, err := internal.GetAvailableContexts()
	if err != nil {
		return err
	}
	for _, contextName := range contexts {
		if !isSelected(c.contextFilter, contextName) {
			continue
		}

		client, err := internal.NewForContext(ctx, contextName)
		if err != nil {
			c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to create client")
			continue
		}

		// Store cluster information
		config := client.Config
		server := ""
		caFile := ""
		insecureSkipVerify := false
		clusterName := contextName
		namespace := "default"
		kubernetesVersion := ""
		nodeCount := int64(0)

		if contextCluster, contextNamespace, err := internal.GetContextDetails(contextName); err != nil {
			c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to read context details")
		} else {
			if contextCluster != "" {
				clusterName = contextCluster
			}
			if contextNamespace != "" {
				namespace = contextNamespace
			}
		}

		if config != nil && config.Host != "" {
			server = config.Host
			if config.CAFile != "" {
				caFile = config.CAFile
			}
			insecureSkipVerify = config.Insecure
		}

		if versionInfo, err := client.Clientset.Discovery().ServerVersion(); err != nil {
			c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to read kubernetes version")
		} else if versionInfo != nil {
			kubernetesVersion = versionInfo.GitVersion
		}

		if nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
			c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to list nodes")
		} else {
			nodeCount = int64(len(nodes.Items))
		}

		if err := c.store.UpsertCluster(ctx, contextName, clusterName, server, caFile, insecureSkipVerify, namespace, kubernetesVersion, nodeCount); err != nil {
			c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to store cluster info")
		}

		if c.shouldSyncResource("namespaces", "k8s_namespaces", options) {
			if err := c.syncNamespaces(ctx, client, contextName); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync namespaces")
			}
		}

		if c.shouldSyncResource("pods", "k8s_pods", options) {
			if err := c.syncPods(ctx, client, contextName); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync pods")
			}
		}

		if c.shouldSyncResource("deployments", "k8s_deployments", options) {
			if err := c.syncDeployments(ctx, client, contextName); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync deployments")
			}
		}

		if c.shouldSyncResource("services", "k8s_services", options) {
			if err := c.syncServices(ctx, client, contextName); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync services")
			}
		}

		if c.shouldSyncResource("crds", "k8s_custom_resources", options) {
			if err := c.syncCRDs(ctx, client, contextName); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync CRDs")
			}
		}

		_ = client.Close(ctx)
	}

	return nil
}

func (c *SourceClient) shouldSyncResource(resourceName, tableName string, options plugin.SyncOptions) bool {
	if !isSelected(c.resourceFilter, resourceName) {
		return false
	}
	if len(options.Tables) == 0 && len(options.SkipTables) == 0 {
		return true
	}
	return plugin.MatchesTable(tableName, options.Tables, options.SkipTables)
}

func (c *SourceClient) syncNamespaces(ctx context.Context, client *internal.Client, contextName string) error {
	namespaces, err := client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		if err := c.store.UpsertNamespace(ctx, contextName, string(ns.UID), ns.Name, string(ns.Status.Phase), ns.CreationTimestamp.Time); err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncPods(ctx context.Context, client *internal.Client, contextName string) error {
	pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if err := c.store.UpsertPod(ctx, contextName, string(pod.UID), pod.Namespace, pod.Name, string(pod.Status.Phase), pod.CreationTimestamp.Time); err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncDeployments(ctx context.Context, client *internal.Client, contextName string) error {
	deployments, err := client.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		if err := c.store.UpsertDeployment(ctx, contextName, string(deployment.UID), deployment.Namespace, deployment.Name, deployment.Status.Replicas, deployment.Status.ReadyReplicas, deployment.CreationTimestamp.Time); err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncServices(ctx context.Context, client *internal.Client, contextName string) error {
	services, err := client.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, service := range services.Items {
		if err := c.store.UpsertService(ctx, contextName, string(service.UID), service.Namespace, service.Name, string(service.Spec.Type), service.Spec.ClusterIP, service.CreationTimestamp.Time); err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncCRDs(ctx context.Context, client *internal.Client, contextName string) error {
	crds, err := client.ApiextensionsClientset.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, crd := range crds.Items {
		if err := c.store.UpsertCRD(ctx, contextName, string(crd.UID), crd.Name, crd.Spec.Group, crd.Spec.Names.Kind, crd.Spec.Names.Plural, string(crd.Spec.Scope), crd.CreationTimestamp.Time); err != nil {
			return err
		}
	}
	return nil
}

func loadConfig(spec any) (Config, error) {
	cfg := Config{}
	if spec != nil {
		b, err := json.Marshal(spec)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is not set")
	}

	if len(cfg.Contexts) == 0 {
		cfg.Contexts = parseList(os.Getenv("K8S_CONTEXTS"))
	}
	if len(cfg.Resources) == 0 {
		cfg.Resources = parseList(os.Getenv("K8S_RESOURCES"))
	}

	return cfg, nil
}

func parseList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func sliceToSet(values []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func isSelected(filter map[string]struct{}, value string) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[value]
	return ok
}
