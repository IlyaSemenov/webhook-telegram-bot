package gitlab

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"webhook-telegram-bot/internal/storage"

	"gocloud.dev/docstore"
	"gocloud.dev/gcerrors"
)

// Pipeline represents a GitLab pipeline with its associated Telegram message
type Pipeline struct {
	PipelineUpdateKey string    `docstore:"pipeline_update_key"` // Partition Key (S) - hash of pipeline URL + chat ID
	MessageID         int       `docstore:"message_id"`          // Telegram message ID
	CreatedAt         time.Time `docstore:"created_at"`
	UpdatedAt         time.Time `docstore:"updated_at"`
	ExpiresAt         int64     `docstore:"expires_at"` // TTL timestamp in epoch seconds
}

// PipelineStorage handles pipeline persistence
type PipelineStorage struct {
	collection *docstore.Collection
}

// NewPipelineStorage creates a new pipeline storage instance
func NewPipelineStorage(ctx context.Context, store *storage.Store) (*PipelineStorage, error) {
	collection, err := store.OpenCollection(ctx, "pipelines", "pipeline_update_key", "")
	if err != nil {
		return nil, err
	}

	return &PipelineStorage{
		collection: collection,
	}, nil
}

// CreatePipelineUpdateKey creates a composite hash from pipeline URL and chat ID
func CreatePipelineUpdateKey(pipelineURL string, chatID int64) string {
	data := fmt.Sprintf("%s:%d", pipelineURL, chatID)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// SavePipeline saves or replaces a pipeline (Put operation only)
func (s *PipelineStorage) SavePipeline(ctx context.Context, pipeline *Pipeline) error {
	now := time.Now()
	if pipeline.CreatedAt.IsZero() {
		pipeline.CreatedAt = now
	}
	pipeline.UpdatedAt = now
	pipeline.ExpiresAt = now.Add(time.Hour * 24).Unix()
	return s.collection.Put(ctx, pipeline)
}

// GetPipeline retrieves a pipeline by its update key with distributed lock logic
func (s *PipelineStorage) GetPipeline(ctx context.Context, pipelineUpdateKey string) (*Pipeline, error) {
	pipeline := &Pipeline{PipelineUpdateKey: pipelineUpdateKey}
	startTime := time.Now()
	maxWait := 15 * time.Second

	for {
		// Check if we've exceeded max wait time
		if time.Since(startTime) > maxWait {
			log.Printf("Timeout waiting for pipeline %s", pipelineUpdateKey)
			// Let's pretend the pipeline doesn't exist
			return nil, nil
		}

		// Check if context was cancelled (e.g., HTTP request timeout)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Try to fetch the pipeline
		err := s.collection.Get(ctx, pipeline)
		if err == nil {
			if pipeline.MessageID != 0 {
				// Valid record found
				return pipeline, nil
			}
			// Record is locked; wait and retry
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// Handle "Not Found" case (try to create lock)
		if gcerrors.Code(err) == gcerrors.NotFound {
			now := time.Now()
			lockPipeline := &Pipeline{
				PipelineUpdateKey: pipelineUpdateKey,
				MessageID:         0, // 0 means locked
				CreatedAt:         now,
				UpdatedAt:         now,
				ExpiresAt:         now.Add(10 * time.Second).Unix(),
			}

			err = s.collection.Create(ctx, lockPipeline)
			if err == nil {
				// Successfully created lock, return nil since the actual record didn't exist
				return nil, nil
			}

			if gcerrors.Code(err) == gcerrors.AlreadyExists {
				// Another process created the record; delay before retry to prevent tight-looping
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Other Create errors
			return nil, err
		}

		// Other Get errors
		return nil, err
	}
}
