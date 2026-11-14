package telegram

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"webhook-telegram-bot/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestNewPipelineStorageMongo runs against the MongoDB database in MONGO_TEST_URL, dropping its collection "pipelines", and is skipped without it.
func TestNewPipelineStorageMongo(t *testing.T) {
	mongoURL := os.Getenv("MONGO_TEST_URL")
	if mongoURL == "" {
		t.Skip("MONGO_TEST_URL is not set")
	}
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })
	u, err := url.Parse(mongoURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collection := client.Database(strings.TrimPrefix(u.Path, "/")).Collection("pipelines")
	if err := collection.Drop(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pipelineStorage, err := NewPipelineStorage(ctx, storage.NewStore(mongoURL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := pipelineStorage.SavePipeline(ctx, &Pipeline{PipelineUpdateKey: "key", MessageID: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// MongoDB expires documents only by a date field
	count, err := collection.CountDocuments(ctx, bson.M{"updated_at": bson.M{"$type": "date"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("documents with a date in updated_at = %d, want 1", count)
	}

	cursor, err := collection.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var indexes []bson.M
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, index := range indexes {
		if index["name"] == "updated_at_1" {
			// The Privacy Policy promises 24 hours
			if seconds, _ := index["expireAfterSeconds"].(int32); seconds != 86400 {
				t.Errorf("expireAfterSeconds = %v, want 86400", index["expireAfterSeconds"])
			}
			return
		}
	}
	t.Errorf("indexes = %v, want a TTL index on updated_at", indexes)
}

func TestCreatePipelineUpdateKey(t *testing.T) {
	t.Run("produces deterministic hash", func(t *testing.T) {
		key1 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 123)
		key2 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 123)
		if key1 != key2 {
			t.Errorf("same inputs produced different keys: %s != %s", key1, key2)
		}
	})

	t.Run("different URLs produce different keys", func(t *testing.T) {
		key1 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 123)
		key2 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/2", 123)
		if key1 == key2 {
			t.Errorf("different URLs produced same key: %s", key1)
		}
	})

	t.Run("different chat IDs produce different keys", func(t *testing.T) {
		key1 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 123)
		key2 := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 456)
		if key1 == key2 {
			t.Errorf("different chat IDs produced same key: %s", key1)
		}
	})

	t.Run("key is hex-encoded SHA256", func(t *testing.T) {
		key := CreatePipelineUpdateKey("https://gitlab.com/repo/-/pipelines/1", 123)
		// SHA256 produces 32 bytes = 64 hex characters
		if len(key) != 64 {
			t.Errorf("key length = %d, want 64", len(key))
		}
	})

	t.Run("empty URL works", func(t *testing.T) {
		key := CreatePipelineUpdateKey("", 0)
		if key == "" {
			t.Errorf("empty inputs should still produce a key")
		}
	})
}

func TestSavePipeline(t *testing.T) {
	ctx := context.Background()
	pipelineStorage, err := NewPipelineStorage(ctx, storage.NewStore(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, final := range []bool{false, true} {
		key := CreatePipelineUpdateKey(testPipelineURL, 1)
		if err := pipelineStorage.SavePipeline(ctx, &Pipeline{PipelineUpdateKey: key, MessageID: 1, Final: final}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Versions before the final flag fail to read a record with a field they do not know
		record := map[string]any{"pipeline_update_key": key}
		if err := pipelineStorage.collection.Get(ctx, record); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := record["final"]; ok != final {
			t.Errorf("Final = %v: field stored = %v, want %v", final, ok, final)
		}
	}
}
