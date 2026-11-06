package storage

import (
	"context"
	"fmt"
	"strings"

	"gocloud.dev/docstore"
	// Import all docstore drivers once for the entire package
	_ "gocloud.dev/docstore/awsdynamodb"
	_ "gocloud.dev/docstore/memdocstore"
	_ "gocloud.dev/docstore/mongodocstore"
)

// Store opens docstore collections on the configured backend.
type Store struct {
	connectionStringBase string
}

// NewStore creates a store for the given connection string base; an empty base selects the in-memory driver.
func NewStore(connectionStringBase string) *Store {
	return &Store{connectionStringBase: connectionStringBase}
}

// getConnectionString returns the appropriate connection string for a collection
func (s *Store) getConnectionString(collectionName string, keyField string, sortKey string) string {
	base := s.connectionStringBase

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

// OpenCollection opens a docstore collection with the appropriate connection string
func (s *Store) OpenCollection(ctx context.Context, collectionName string, keyField string, sortKey string) (*docstore.Collection, error) {
	connectionString := s.getConnectionString(collectionName, keyField, sortKey)
	return docstore.OpenCollection(ctx, connectionString)
}
