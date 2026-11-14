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

// SendOrUpdatePipelineMessage updates an existing pipeline message or creates a new one
func (m *PipelineMessenger) SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string) error {
	ctx := context.Background()
	pipelineUpdateKey := CreatePipelineUpdateKey(pipelineURL, chatID)

	// Get existing pipeline mapping
	pipeline, err := m.storage.GetPipeline(ctx, pipelineUpdateKey)
	if err != nil {
		return err
	}

	if pipeline == nil {
		// Pipeline not found, send new message
		msg, err := m.telegram.SendMessageWithResult(chatID, text)
		if err != nil {
			return err
		}

		// Store pipeline mapping
		pipeline := &Pipeline{
			PipelineUpdateKey: pipelineUpdateKey,
			MessageID:         msg.ID,
		}
		return m.storage.SavePipeline(ctx, pipeline)
	} else {
		// Update the existing message
		if err := m.telegram.UpdateMessage(chatID, pipeline.MessageID, text); err != nil {
			return err
		}
		// Update the mapping timestamp
		return m.storage.SavePipeline(ctx, pipeline)
	}
}
