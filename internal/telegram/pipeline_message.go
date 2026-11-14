package telegram

import (
	"context"
	"fmt"

	"webhook-telegram-bot/internal/storage"
)

// PipelineMessenger sends one Telegram message per pipeline and edits it on later events.
type PipelineMessenger struct {
	telegram *TelegramService
	storage  *PipelineStorage
}

// NewPipelineMessenger creates a pipeline messenger that sends messages through s and keeps their IDs in store.
func NewPipelineMessenger(ctx context.Context, s *TelegramService, store *storage.Store) (*PipelineMessenger, error) {
	pipelineStorage, err := NewPipelineStorage(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pipeline storage: %w", err)
	}
	return &PipelineMessenger{telegram: s, storage: pipelineStorage}, nil
}

// SendOrUpdatePipelineMessage sends a pipeline message or edits the existing one unless it already shows the final status.
// Set final for the text of the final status, so that events delivered out of order do not replace it.
func (m *PipelineMessenger) SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string, final bool) error {
	return m.sendOrUpdatePipelineMessage(chatID, pipelineURL, text, final, false)
}

// FinishPipelineMessage edits an existing pipeline message to the final status unless it already shows one.
// Without a message, it sends nothing and keeps later events from sending one.
func (m *PipelineMessenger) FinishPipelineMessage(chatID int64, pipelineURL string, text string) error {
	return m.sendOrUpdatePipelineMessage(chatID, pipelineURL, text, true, true)
}

// sendOrUpdatePipelineMessage edits an existing pipeline message unless it already shows the final status,
// or sends a new one unless updateOnly is set, which requires final.
func (m *PipelineMessenger) sendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string, final bool, updateOnly bool) error {
	ctx := context.Background()
	pipelineUpdateKey := CreatePipelineUpdateKey(pipelineURL, chatID)

	pipeline, err := m.storage.GetPipeline(ctx, pipelineUpdateKey)
	if err != nil {
		return err
	}

	if pipeline == nil {
		if updateOnly {
			// Replace the lock that GetPipeline created with a record that keeps a later event from reporting an earlier status
			return m.storage.SavePipeline(ctx, &Pipeline{PipelineUpdateKey: pipelineUpdateKey, Final: final})
		}

		msg, err := m.telegram.SendMessageWithResult(chatID, text)
		if err != nil {
			return err
		}

		pipeline := &Pipeline{
			PipelineUpdateKey: pipelineUpdateKey,
			MessageID:         msg.ID,
			Final:             final,
		}
		return m.storage.SavePipeline(ctx, pipeline)
	}

	// An event delivered after the final one must not replace the final status or report an earlier one;
	// events handled concurrently with the final one can still do so
	if pipeline.Final {
		return nil
	}

	if err := m.telegram.UpdateMessage(chatID, pipeline.MessageID, text); err != nil {
		return err
	}
	pipeline.Final = final
	return m.storage.SavePipeline(ctx, pipeline)
}
