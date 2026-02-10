package main

import (
	"context"
	"os"

	pluginpkg "github.com/Genos0820/cq-k8s-custom/plugin"
	"github.com/cloudquery/plugin-sdk/v4/serve"
)

func main() {
	if err := serve.Plugin(pluginpkg.NewCQPlugin()).Serve(context.Background()); err != nil {
		os.Exit(1)
	}
}
