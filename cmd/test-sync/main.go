package main

import (
	"context"
	"log"
	"os"

	cqplugin "github.com/Genos0820/cq-k8s-custom/plugin"
	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/plugin"
	"github.com/rs/zerolog"
)

func main() {
	ctx := context.Background()
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	config := map[string]interface{}{
		"database_url": "postgres://postgres:postgres@localhost:5434/k8s?sslmode=disable",
		"contexts":     []string{"dev", "prod"},
		"resources":    []string{"namespaces", "pods", "deployments", "services", "crds"},
	}

	client, err := cqplugin.NewSourceClient(ctx, logger, config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close(ctx)

	tables, err := client.Tables(ctx, plugin.TableOptions{})
	if err != nil {
		log.Fatalf("Failed to get tables: %v", err)
	}

	log.Printf("Found %d tables\n", len(tables))
	for _, t := range tables {
		log.Printf("  - %s\n", t.Name)
	}

	log.Println("Starting sync...")
	syncChan := make(chan message.SyncMessage, 100)
	go func() {
		if err := client.Sync(ctx, plugin.SyncOptions{}, syncChan); err != nil {
			log.Fatalf("Sync failed: %v", err)
		}
		close(syncChan)
	}()

	count := 0
	for msg := range syncChan {
		count++
		log.Printf("Message %d: %T\n", count, msg)
	}

	log.Printf("Sync complete! Processed %d messages\n", count)
}
