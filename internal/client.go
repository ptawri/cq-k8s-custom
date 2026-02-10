package internal

import (
	"context"
	"fmt"
	"path/filepath"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	Clientset              *kubernetes.Clientset
	ApiextensionsClientset apiextensionsclientset.Interface
	id                     string
	context                string
}

// ID returns the unique identifier for this client
func (c *Client) ID() string {
	return c.id
}

// Close closes the client connection
func (c *Client) Close(ctx context.Context) error {
	return nil
}

// NewForContext creates a new Kubernetes client for a specific context
func NewForContext(ctx context.Context, kubeContext string) (*Client, error) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: kubeContext,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiextensionsClientset, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset:              clientset,
		ApiextensionsClientset: apiextensionsClientset,
		id:                     fmt.Sprintf("k8s-%s", kubeContext),
		context:                kubeContext,
	}, nil
}

// New creates a new Kubernetes client for the default context
func New(ctx context.Context) (*Client, error) {
	return NewForContext(ctx, "")
}

// GetAvailableContexts returns all available Kubernetes contexts
func GetAvailableContexts() ([]string, error) {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfig
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	config, err := clientConfig.RawConfig()
	if err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(config.Contexts))
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts, nil
}
