package gitlab

import (
	"testing"
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
