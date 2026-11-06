package gitlab

import (
	"context"
	"strings"
	"testing"

	"webhook-telegram-bot/internal/storage"
	"webhook-telegram-bot/internal/telegram"
)

func TestNewGitLabTelegramService(t *testing.T) {
	ctx := context.Background()
	// The connection string is an invalid URL, so opening a collection fails without a network call.
	brokenStore := storage.NewStore("dynamodb://%")

	t.Run("without a token skips pipeline storage", func(t *testing.T) {
		svc, err := NewGitLabTelegramService(ctx, "", telegram.Settings{}, brokenStore, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != nil {
			t.Errorf("service = %v, want nil", svc)
		}
	})

	t.Run("with a token returns the pipeline storage error", func(t *testing.T) {
		_, err := NewGitLabTelegramService(ctx, "test-token", telegram.Settings{}, brokenStore, nil)
		if err == nil || !strings.Contains(err.Error(), "pipeline storage") {
			t.Errorf("error = %v, want a pipeline storage error", err)
		}
	})

	t.Run("with a token opens pipeline storage", func(t *testing.T) {
		svc, err := NewGitLabTelegramService(ctx, "test-token", telegram.Settings{}, storage.NewStore(""), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc == nil || svc.pipelineStorage == nil {
			t.Errorf("service = %v, want a service with pipeline storage", svc)
		}
	})
}
