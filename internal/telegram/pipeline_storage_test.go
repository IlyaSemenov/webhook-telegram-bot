package telegram

import (
	"context"
	"testing"

	"webhook-telegram-bot/internal/storage"
)

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
