package telegram

import (
	"context"
	"testing"

	"webhook-telegram-bot/internal/storage"
)

const testPipelineURL = "https://example.com/pipelines/1"

// newPipelineMessenger returns a pipeline messenger over new in-memory storage that sends messages to a fake Telegram Bot API.
func newPipelineMessenger(t *testing.T) (*PipelineMessenger, *telegramServer) {
	t.Helper()
	service, _, server := newWebhookService(t)
	messenger, err := NewPipelineMessenger(context.Background(), service, storage.NewStore(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return messenger, server
}

func TestSendOrUpdatePipelineMessage(t *testing.T) {
	t.Run("sends a message and edits it on later events", func(t *testing.T) {
		messenger, server := newPipelineMessenger(t)

		for _, text := range []string{"running", "still running"} {
			if err := messenger.SendOrUpdatePipelineMessage(1, testPipelineURL, text, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		checkRequests(t, server, "sendMessage", "editMessageText")
	})

	t.Run("keeps the final status", func(t *testing.T) {
		messenger, server := newPipelineMessenger(t)

		for _, update := range []struct {
			text  string
			final bool
		}{
			{"running", false},
			{"success", true},
			{"running", false},
			{"failure", true},
		} {
			if err := messenger.SendOrUpdatePipelineMessage(1, testPipelineURL, update.text, update.final); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		checkRequests(t, server, "sendMessage", "editMessageText")
		if text := server.requests[1].form.Get("text"); text != "success" {
			t.Errorf("text = %q, want %q", text, "success")
		}
	})
}

func TestFinishPipelineMessage(t *testing.T) {
	t.Run("edits an existing message", func(t *testing.T) {
		messenger, server := newPipelineMessenger(t)

		if err := messenger.SendOrUpdatePipelineMessage(1, testPipelineURL, "running", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := messenger.FinishPipelineMessage(1, testPipelineURL, "skipped"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		checkRequests(t, server, "sendMessage", "editMessageText")
	})

	t.Run("sends nothing without a message, nor on a later event", func(t *testing.T) {
		messenger, server := newPipelineMessenger(t)

		if err := messenger.FinishPipelineMessage(1, testPipelineURL, "skipped"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := messenger.SendOrUpdatePipelineMessage(1, testPipelineURL, "running", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		checkRequests(t, server)
	})
}
