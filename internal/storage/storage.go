package storage

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gocloud.dev/docstore"
	_ "gocloud.dev/docstore/awsdynamodb"
	"gocloud.dev/docstore/driver"
	"gocloud.dev/docstore/memdocstore"
	"gocloud.dev/docstore/mongodocstore"
)

// Store opens docstore collections on the configured backend.
type Store struct {
	connectionStringBase string

	mongoMu       sync.Mutex
	mongoDatabase *mongo.Database // Connected on the first MongoDB collection
}

// NewStore creates a store for the given connection string base: a dynamodb:// URL with the table name prefix,
// a mongodb:// URL with the database name, or an empty string for the in-memory driver.
func NewStore(connectionStringBase string) *Store {
	return &Store{connectionStringBase: connectionStringBase}
}

// OpenCollection opens a docstore collection keyed by keyField, or by keyField and sortKey together if sortKey is not empty.
func (s *Store) OpenCollection(ctx context.Context, collectionName string, keyField string, sortKey string) (*docstore.Collection, error) {
	base := s.connectionStringBase

	switch {
	case strings.HasPrefix(base, "dynamodb://"):
		connectionString := fmt.Sprintf("%s-%s?partition_key=%s", base, collectionName, keyField)
		if sortKey != "" {
			connectionString += "&sort_key=" + sortKey
		}
		return docstore.OpenCollection(ctx, connectionString)

	case strings.HasPrefix(base, "mongodb://") || strings.HasPrefix(base, "mongodb+srv://"):
		return s.openMongoCollection(ctx, collectionName, keyField, sortKey)

	case base == "":
		if sortKey == "" {
			return memdocstore.OpenCollection(keyField, nil)
		}
		return memdocstore.OpenCollectionWithKeyFunc(createCompositeKeyFunc(keyField, sortKey), nil)

	default:
		return nil, fmt.Errorf("unsupported storage connection string %q", base)
	}
}

// openMongoCollection opens a MongoDB collection, whose _id is keyField or the composite key of keyField and sortKey.
func (s *Store) openMongoCollection(ctx context.Context, collectionName string, keyField string, sortKey string) (*docstore.Collection, error) {
	database, err := s.connectMongo(ctx)
	if err != nil {
		return nil, err
	}
	collection := database.Collection(collectionName)

	if sortKey == "" {
		return mongodocstore.OpenCollection(collection, keyField, nil)
	}
	if err := migrateMongoKeys(ctx, collection, keyField, sortKey); err != nil {
		return nil, fmt.Errorf("failed to migrate keys of MongoDB collection %s: %w", collectionName, err)
	}
	return mongodocstore.OpenCollectionWithIDFunc(collection, createCompositeKeyFunc(keyField, sortKey), nil)
}

// connectMongo returns the database from the connection string, connecting on the first call.
func (s *Store) connectMongo(ctx context.Context) (*mongo.Database, error) {
	s.mongoMu.Lock()
	defer s.mongoMu.Unlock()

	if s.mongoDatabase != nil {
		return s.mongoDatabase, nil
	}

	u, err := url.Parse(s.connectionStringBase)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MongoDB URL: %w", err)
	}
	databaseName := strings.TrimPrefix(u.Path, "/")
	if databaseName == "" {
		return nil, fmt.Errorf("MongoDB URL has no database name")
	}

	client, err := mongodocstore.Dial(ctx, s.connectionStringBase)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	s.mongoDatabase = client.Database(databaseName)
	return s.mongoDatabase, nil
}

// createCompositeKeyFunc returns the key function of a collection keyed by keyField and sortKey together,
// for drivers without sort keys.
func createCompositeKeyFunc(keyField string, sortKey string) func(docstore.Document) interface{} {
	return func(doc docstore.Document) interface{} {
		ddoc, err := driver.NewDocument(doc)
		if err != nil {
			return nil
		}
		key, err := ddoc.GetField(keyField)
		if err != nil {
			return nil
		}
		sort, err := ddoc.GetField(sortKey)
		if err != nil {
			return nil
		}
		return createCompositeKey(key, sort)
	}
}

// createCompositeKey returns the single key that stands for the pair of a key and a sort key.
func createCompositeKey(key interface{}, sort interface{}) string {
	return fmt.Sprintf("%v:%v", key, sort)
}

// migrateMongoKeys moves documents whose _id is the keyField value, as stored before the collection had a composite key,
// to the composite key of keyField and sortKey.
// A document whose composite key is already taken is a stale copy of the moved one, so it is deleted.
func migrateMongoKeys(ctx context.Context, collection *mongo.Collection, keyField string, sortKey string) error {
	cursor, err := collection.Find(ctx, bson.M{"_id": bson.M{"$not": bson.M{"$type": "string"}}})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	migrated := 0
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		oldID := doc["_id"]
		sort, ok := doc[sortKey]
		if !ok {
			return fmt.Errorf("document %v has no %s", oldID, sortKey)
		}

		doc[keyField] = oldID
		doc["_id"] = createCompositeKey(oldID, sort)
		if _, err := collection.InsertOne(ctx, doc); err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
		if _, err := collection.DeleteOne(ctx, bson.M{"_id": oldID}); err != nil {
			return err
		}
		migrated++
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	if migrated > 0 {
		log.Printf("Migrated %d documents of MongoDB collection %s to composite keys", migrated, collection.Name())
	}
	return nil
}
