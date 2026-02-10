package plugin

import (
	"github.com/cloudquery/plugin-sdk/v4/plugin"
)

const (
	pluginName    = "k8s-custom"
	pluginVersion = "0.0.1"
)

func NewCQPlugin() *plugin.Plugin {
	return plugin.NewSourcePlugin(pluginName, pluginVersion, NewSourceClient)
}
