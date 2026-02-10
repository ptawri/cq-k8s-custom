package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/Genos0820/cq-k8s-custom/internal"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
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

		// Sync cluster information
		if c.shouldSyncResource("clusters", "k8s_clusters", options) {
			if err := c.syncCluster(ctx, client, contextName, res); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync cluster")
			}
		}

		if c.shouldSyncResource("namespaces", "k8s_namespaces", options) {
			if err := c.syncNamespaces(ctx, client, contextName, res); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync namespaces")
			}
		}

		if c.shouldSyncResource("pods", "k8s_pods", options) {
			if err := c.syncPods(ctx, client, contextName, res); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync pods")
			}
		}

		if c.shouldSyncResource("deployments", "k8s_deployments", options) {
			if err := c.syncDeployments(ctx, client, contextName, res); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync deployments")
			}
		}

		if c.shouldSyncResource("services", "k8s_services", options) {
			if err := c.syncServices(ctx, client, contextName, res); err != nil {
				c.logger.Warn().Err(err).Str("context", contextName).Msg("failed to sync services")
			}
		}

		if c.shouldSyncResource("crds", "k8s_custom_resources", options) {
			if err := c.syncCRDs(ctx, client, contextName, res); err != nil {
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

func (c *SourceClient) syncCluster(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
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

	table := ClustersTable()
	bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
	defer bldr.Release()

	now := time.Now()
	bldr.Field(table.Columns.Index("context_name")).(*array.StringBuilder).Append(contextName)
	bldr.Field(table.Columns.Index("cluster_name")).(*array.StringBuilder).Append(clusterName)
	bldr.Field(table.Columns.Index("server")).(*array.StringBuilder).Append(server)
	bldr.Field(table.Columns.Index("ca_file")).(*array.StringBuilder).Append(caFile)
	bldr.Field(table.Columns.Index("insecure_skip_verify")).(*array.BooleanBuilder).Append(insecureSkipVerify)
	bldr.Field(table.Columns.Index("namespace")).(*array.StringBuilder).Append(namespace)
	bldr.Field(table.Columns.Index("kubernetes_version")).(*array.StringBuilder).Append(kubernetesVersion)
	bldr.Field(table.Columns.Index("node_count")).(*array.Int64Builder).Append(nodeCount)
	bldr.Field(table.Columns.Index("synced_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(now.UnixNano()))
	bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(now.UnixNano()))
	bldr.Field(table.Columns.Index("updated_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(now.UnixNano()))

	res <- &message.SyncInsert{Record: bldr.NewRecord()}
	return nil
}

func (c *SourceClient) syncNamespaces(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
	namespaces, err := client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	table := NamespacesTable()
	for _, ns := range namespaces.Items {
		bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
		defer bldr.Release()

		uid, _ := uuid.Parse(string(ns.UID))
		bldr.Field(table.Columns.Index("id")).(*types.UUIDBuilder).Append(uid)
		bldr.Field(table.Columns.Index("name")).(*array.StringBuilder).Append(ns.Name)
		bldr.Field(table.Columns.Index("status")).(*array.StringBuilder).Append(string(ns.Status.Phase))
		bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(ns.CreationTimestamp.Time.UnixMicro()))

		res <- &message.SyncInsert{Record: bldr.NewRecord()}
	}
	return nil
}

func (c *SourceClient) syncPods(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
	pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	table := PodsTable()
	for _, pod := range pods.Items {
		bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
		defer bldr.Release()

		uid, _ := uuid.Parse(string(pod.UID))
		bldr.Field(table.Columns.Index("id")).(*types.UUIDBuilder).Append(uid)
		bldr.Field(table.Columns.Index("name")).(*array.StringBuilder).Append(pod.Name)
		bldr.Field(table.Columns.Index("namespace")).(*array.StringBuilder).Append(pod.Namespace)
		bldr.Field(table.Columns.Index("status")).(*array.StringBuilder).Append(string(pod.Status.Phase))
		bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(pod.CreationTimestamp.Time.UnixMicro()))

		res <- &message.SyncInsert{Record: bldr.NewRecord()}
	}
	return nil
}

func (c *SourceClient) syncDeployments(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
	deployments, err := client.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	table := DeploymentsTable()
	for _, deployment := range deployments.Items {
		bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
		defer bldr.Release()

		uid, _ := uuid.Parse(string(deployment.UID))
		bldr.Field(table.Columns.Index("id")).(*types.UUIDBuilder).Append(uid)
		bldr.Field(table.Columns.Index("name")).(*array.StringBuilder).Append(deployment.Name)
		bldr.Field(table.Columns.Index("namespace")).(*array.StringBuilder).Append(deployment.Namespace)
		bldr.Field(table.Columns.Index("replicas")).(*array.Int64Builder).Append(int64(deployment.Status.Replicas))
		bldr.Field(table.Columns.Index("ready")).(*array.Int64Builder).Append(int64(deployment.Status.ReadyReplicas))
		bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(deployment.CreationTimestamp.Time.UnixMicro()))

		res <- &message.SyncInsert{Record: bldr.NewRecord()}
	}
	return nil
}

func (c *SourceClient) syncServices(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
	services, err := client.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	table := ServicesTable()
	for _, service := range services.Items {
		bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
		defer bldr.Release()

		uid, _ := uuid.Parse(string(service.UID))
		bldr.Field(table.Columns.Index("id")).(*types.UUIDBuilder).Append(uid)
		bldr.Field(table.Columns.Index("name")).(*array.StringBuilder).Append(service.Name)
		bldr.Field(table.Columns.Index("namespace")).(*array.StringBuilder).Append(service.Namespace)
		bldr.Field(table.Columns.Index("type")).(*array.StringBuilder).Append(string(service.Spec.Type))
		bldr.Field(table.Columns.Index("cluster_ip")).(*array.StringBuilder).Append(service.Spec.ClusterIP)
		bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(service.CreationTimestamp.Time.UnixMicro()))

		res <- &message.SyncInsert{Record: bldr.NewRecord()}
	}
	return nil
}

func (c *SourceClient) syncCRDs(ctx context.Context, client *internal.Client, contextName string, res chan<- message.SyncMessage) error {
	crds, err := client.ApiextensionsClientset.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	table := CustomResourcesTable()
	for _, crd := range crds.Items {
		bldr := array.NewRecordBuilder(memory.DefaultAllocator, table.ToArrowSchema())
		defer bldr.Release()

		uid, _ := uuid.Parse(string(crd.UID))
		bldr.Field(table.Columns.Index("id")).(*types.UUIDBuilder).Append(uid)
		bldr.Field(table.Columns.Index("name")).(*array.StringBuilder).Append(crd.Name)
		bldr.Field(table.Columns.Index("group")).(*array.StringBuilder).Append(crd.Spec.Group)
		bldr.Field(table.Columns.Index("kind")).(*array.StringBuilder).Append(crd.Spec.Names.Kind)
		bldr.Field(table.Columns.Index("plural")).(*array.StringBuilder).Append(crd.Spec.Names.Plural)
		bldr.Field(table.Columns.Index("scope")).(*array.StringBuilder).Append(string(crd.Spec.Scope))
		bldr.Field(table.Columns.Index("created_at")).(*array.TimestampBuilder).Append(arrow.Timestamp(crd.CreationTimestamp.Time.UnixMicro()))

		res <- &message.SyncInsert{Record: bldr.NewRecord()}
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
