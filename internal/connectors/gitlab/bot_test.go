package gitlab

import (
	"context"
	"strings"
	"testing"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/storage"
)

func TestNewGitLabTelegramService(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the pipeline storage error", func(t *testing.T) {
		// The connection string is an invalid URL, so opening a collection fails without a network call.
		_, err := NewGitLabTelegramService(ctx, app.Deps{Token: "test-token", Store: storage.NewStore("dynamodb://%")})
		if err == nil || !strings.Contains(err.Error(), "pipeline storage") {
			t.Errorf("error = %v, want a pipeline storage error", err)
		}
	})

	t.Run("creates the pipeline messenger", func(t *testing.T) {
		svc, err := NewGitLabTelegramService(ctx, app.Deps{Token: "test-token", Store: storage.NewStore("")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc.PipelineMessenger == nil {
			t.Errorf("pipeline messenger is nil")
		}
	})
}
