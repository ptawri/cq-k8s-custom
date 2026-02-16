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
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateClusterUID creates a unique identifier for a cluster based on its API server address.
func generateClusterUID(client *internal.Client) string {
	if client != nil && client.Config != nil && client.Config.Host != "" {
		// Use a UUID5 hash of the API server address for uniqueness
		return uuid.NewSHA1(uuid.NameSpaceURL, []byte(client.Config.Host)).String()
	}
	// Fallback: random UUID
	return uuid.New().String()
}

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
	// Migration handled elsewhere
	if res != nil {
	} // Silence unused parameter warning

	if len(c.contextFilter) == 0 {
		// No contexts specified: use only the current context
		client, err := internal.New(ctx)
		if err != nil {
			return err
		}
		contextName := client.ID()
		clusterUID := generateClusterUID(client)

		if c.shouldSyncResource("clusters", "k8s_clusters", options) {
			if err := c.syncCluster(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync cluster")
			}
		}
		if c.shouldSyncResource("namespaces", "k8s_namespaces", options) {
			if err := c.syncNamespaces(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync namespaces")
			}
		}
		if c.shouldSyncResource("pods", "k8s_pods", options) {
			if err := c.syncPods(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync pods")
			}
		}
		if c.shouldSyncResource("deployments", "k8s_deployments", options) {
			if err := c.syncDeployments(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync deployments")
			}
		}
		if c.shouldSyncResource("services", "k8s_services", options) {
			if err := c.syncServices(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync services")
			}
		}
		if c.shouldSyncResource("crds", "k8s_custom_resources", options) {
			if err := c.syncCRDs(ctx, client, contextName, clusterUID); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync CRDs")
			}
		}
		_ = client.Close(ctx)
		return nil
	} else {
		// Contexts specified: use each one
		for contextName := range c.contextFilter {
			client, err := internal.NewForContext(ctx, contextName)
			if err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to create client")
				continue
			}
			clusterUID := generateClusterUID(client)

			if c.shouldSyncResource("clusters", "k8s_clusters", options) {
				if err := c.syncCluster(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync cluster")
				}
			}
			if c.shouldSyncResource("namespaces", "k8s_namespaces", options) {
				if err := c.syncNamespaces(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync namespaces")
				}
			}
			if c.shouldSyncResource("pods", "k8s_pods", options) {
				if err := c.syncPods(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync pods")
				}
			}
			if c.shouldSyncResource("deployments", "k8s_deployments", options) {
				if err := c.syncDeployments(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync deployments")
				}
			}
			if c.shouldSyncResource("services", "k8s_services", options) {
				if err := c.syncServices(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync services")
				}
			}
			if c.shouldSyncResource("crds", "k8s_custom_resources", options) {
				if err := c.syncCRDs(ctx, client, contextName, clusterUID); err != nil {
					c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync CRDs")
				}
			}
			_ = client.Close(ctx)
		}
		return nil
	}
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

func (c *SourceClient) syncCluster(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
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

	err := c.store.UpsertCluster(ctx, clusterUID, contextName, clusterName, server, caFile, insecureSkipVerify, namespace, kubernetesVersion, nodeCount)
	if err != nil {
		return err
	}
	return nil
}

func (c *SourceClient) syncNamespaces(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
	namespaces, err := client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		uid := string(ns.UID)
		name := ns.Name
		status := string(ns.Status.Phase)
		createdAt := ns.CreationTimestamp.Time
		err := c.store.UpsertNamespace(ctx, clusterUID, contextName, uid, name, status, createdAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncPods(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
	pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		uid := string(pod.UID)
		namespace := pod.Namespace
		name := pod.Name
		status := string(pod.Status.Phase)
		createdAt := pod.CreationTimestamp.Time
		err := c.store.UpsertPod(ctx, clusterUID, contextName, uid, namespace, name, status, createdAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncDeployments(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
	deployments, err := client.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		uid := string(deployment.UID)
		namespace := deployment.Namespace
		name := deployment.Name
		replicas := int32(deployment.Status.Replicas)
		ready := int32(deployment.Status.ReadyReplicas)
		createdAt := deployment.CreationTimestamp.Time
		err := c.store.UpsertDeployment(ctx, clusterUID, contextName, uid, namespace, name, replicas, ready, createdAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncServices(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
	services, err := client.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, service := range services.Items {
		uid := string(service.UID)
		namespace := service.Namespace
		name := service.Name
		serviceType := string(service.Spec.Type)
		clusterIP := service.Spec.ClusterIP
		createdAt := service.CreationTimestamp.Time
		err := c.store.UpsertService(ctx, clusterUID, contextName, uid, namespace, name, serviceType, clusterIP, createdAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *SourceClient) syncCRDs(ctx context.Context, client *internal.Client, contextName, clusterUID string) error {
	crds, err := client.ApiextensionsClientset.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, crd := range crds.Items {
		uid := string(crd.UID)
		name := crd.Name
		groupName := crd.Spec.Group
		kind := crd.Spec.Names.Kind
		plural := crd.Spec.Names.Plural
		scope := string(crd.Spec.Scope)
		createdAt := crd.CreationTimestamp.Time
		err := c.store.UpsertCRD(ctx, clusterUID, contextName, uid, name, groupName, kind, plural, scope, createdAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadConfig(spec any) (Config, error) {
	cfg := Config{}
	if spec == nil {
		return cfg, errors.New("spec is required")
	}

	switch value := spec.(type) {
	case string:
		if err := unmarshalConfigString(value, &cfg); err != nil {
			return cfg, err
		}
	case []byte:
		if err := unmarshalConfigBytes(value, &cfg); err != nil {
			return cfg, err
		}
	default:
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

func unmarshalConfigString(value string, cfg *Config) error {
	if err := unmarshalConfigBytes([]byte(value), cfg); err != nil {
		return err
	}
	return nil
}

func unmarshalConfigBytes(value []byte, cfg *Config) error {
	if len(value) == 0 {
		return errors.New("spec is empty")
	}
	if err := json.Unmarshal(value, cfg); err == nil {
		return nil
	}
	if err := yaml.Unmarshal(value, cfg); err == nil {
		return nil
	}
	return errors.New("spec must be valid JSON or YAML")
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
