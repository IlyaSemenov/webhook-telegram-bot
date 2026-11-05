package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"webhook-telegram-bot/internal/config"

	"gocloud.dev/docstore"
	// Import all docstore drivers once for the entire package
	_ "gocloud.dev/docstore/awsdynamodb"
	_ "gocloud.dev/docstore/memdocstore"
	_ "gocloud.dev/docstore/mongodocstore"
)

// Storage provides centralized access to all storage types
type Storage struct {
	ChatStorage     *ChatStorage
	PipelineStorage *PipelineStorage
}

// NewStorage creates a new centralized storage instance
func NewStorage(ctx context.Context) (*Storage, error) {
	chatStorage, err := NewChatStorage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chat storage: %w", err)
	}

	pipelineStorage, err := NewPipelineStorage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pipeline storage: %w", err)
	}

	return &Storage{
		ChatStorage:     chatStorage,
		PipelineStorage: pipelineStorage,
	}, nil
}

// getConnectionString returns the appropriate connection string for a collection
func getConnectionString(collectionName string, keyField string, sortKey string) string {
	base := config.Global.StorageConnectionStringBase

	if strings.HasPrefix(base, "dynamodb://") {
		// DynamoDB format
		suffix := fmt.Sprintf("-%s?partition_key=%s", collectionName, keyField)
		if sortKey != "" {
			suffix += fmt.Sprintf("&sort_key=%s", sortKey)
		}
		return base + suffix
	}

	if strings.HasPrefix(base, "mongo://") {
		return fmt.Sprintf("%s/%s?id_field=%s", base, collectionName, keyField)
	}

	// Default fallback
	return fmt.Sprintf("mem://%s/%s", collectionName, keyField)
}

// openCollection opens a docstore collection with the appropriate connection string
func openCollection(ctx context.Context, collectionName string, keyField string, sortKey string) (*docstore.Collection, error) {
	connectionString := getConnectionString(collectionName, keyField, sortKey)
	return docstore.OpenCollection(ctx, connectionString)
}

// Close closes all storage connections and returns the first error encountered.
func (s *Storage) Close() error {
	var firstErr error

	// Define all closable storages in a slice
	closers := []io.Closer{
		s.ChatStorage,
		s.PipelineStorage,
		// Add more storages here as needed
	}

	// Close each and track the first error
	for _, closer := range closers {
		if closer != nil {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}
