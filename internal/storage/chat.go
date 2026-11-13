package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"gocloud.dev/docstore"
	"gocloud.dev/gcerrors"
)

// Kinds of webhook URLs that the chat accepts for one bot.
const (
	// WebhookAuthAny accepts URLs with and without a token.
	// A chat that used the bot before URLs had tokens, or that used another bot then, has it.
	WebhookAuthAny = ""
	// WebhookAuthToken accepts only URLs with a token.
	WebhookAuthToken = "token"
	// WebhookAuthUpgraded accepts only URLs with a token since the first request with a token to a chat with WebhookAuthAny.
	WebhookAuthUpgraded = "upgraded"
)

// Chat represents a Telegram chat where the bot has been added
type Chat struct {
	ChatID          int64     `docstore:"chat_id"`  // Partition Key (N)
	BotType         string    `docstore:"bot_type"` // Sort Key (S)
	CreatedAt       time.Time `docstore:"created_at"`
	UpdatedAt       time.Time `docstore:"updated_at"`
	WebhookAuth     string    `docstore:"webhook_auth"`      // Kind of accepted webhook URLs, one of the WebhookAuth constants
	WebhookIssuedAt int64     `docstore:"webhook_issued_at"` // Issue time of the webhook URL in Unix seconds; zero for records created before tokens
	LegacyWarnedAt  time.Time `docstore:"legacy_warned_at"`  // Time of the last warning about a rejected URL without a token

	// DocstoreRevision is the docstore revision, which makes an update fail if the record changed since it was read.
	DocstoreRevision interface{}
}

// ChatStorage handles chat persistence
type ChatStorage struct {
	collection *docstore.Collection
}

// OpenChatCollection opens the collection of chats, keyed by the chat and the bot.
func OpenChatCollection(ctx context.Context, store *Store) (*docstore.Collection, error) {
	return store.OpenCollection(ctx, "chats", "chat_id", "bot_type")
}

// NewChatStorage creates chat storage over a collection opened by OpenChatCollection.
func NewChatStorage(collection *docstore.Collection) *ChatStorage {
	return &ChatStorage{
		collection: collection,
	}
}

// SaveChat records that the bot has delivered a message to the chat, creating the record of the chat if it has none.
func (s *ChatStorage) SaveChat(ctx context.Context, chat *Chat) error {
	key := &Chat{ChatID: chat.ChatID, BotType: chat.BotType}
	err := s.collection.Update(ctx, key, docstore.Mods{"updated_at": time.Now()})
	// In DynamoDB, a missing record gives FailedPrecondition instead of NotFound
	if code := gcerrors.Code(err); code == gcerrors.NotFound || code == gcerrors.FailedPrecondition {
		_, err = s.createChat(ctx, chat.ChatID, chat.BotType, false)
	}
	return err
}

// GetChat returns the record of the chat, or nil if the chat has none.
func (s *ChatStorage) GetChat(ctx context.Context, chatID int64, botType string) (*Chat, error) {
	chat := &Chat{ChatID: chatID, BotType: botType}
	if err := s.collection.Get(ctx, chat); err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, nil
		}
		return nil, err
	}
	return chat, nil
}

// GetOrCreateChat returns the record of the chat, creating it if the chat has none.
func (s *ChatStorage) GetOrCreateChat(ctx context.Context, chatID int64, botType string) (*Chat, error) {
	chat, err := s.GetChat(ctx, chatID, botType)
	if err != nil || chat != nil {
		return chat, err
	}
	return s.createChat(ctx, chatID, botType, false)
}

// CreateLegacyChat creates the record of a chat that has none, if the chat used any bot before URLs had tokens.
// It returns nil without creating a record if the chat did not.
func (s *ChatStorage) CreateLegacyChat(ctx context.Context, chatID int64, botType string) (*Chat, error) {
	return s.createChat(ctx, chatID, botType, true)
}

// createChat creates the record of a chat that has none and returns the stored record, which is another one if it was created concurrently.
// The record accepts URLs without a token if the chat used any bot before URLs had tokens.
// With onlyLegacy, it returns nil without creating a record if the chat did not; otherwise it returns a record or an error.
func (s *ChatStorage) createChat(ctx context.Context, chatID int64, botType string, onlyLegacy bool) (*Chat, error) {
	legacy, err := s.hasLegacyHistory(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if onlyLegacy && !legacy {
		return nil, nil
	}

	now := time.Now()
	chat := &Chat{
		ChatID:          chatID,
		BotType:         botType,
		CreatedAt:       now,
		UpdatedAt:       now,
		WebhookAuth:     WebhookAuthAny,
		WebhookIssuedAt: now.Unix(),
	}
	if !legacy {
		chat.WebhookAuth = WebhookAuthToken
	}

	err = s.collection.Create(ctx, chat)
	if gcerrors.Code(err) == gcerrors.AlreadyExists {
		existing, err := s.GetChat(ctx, chatID, botType)
		if err == nil && existing == nil {
			// The concurrent record is not readable yet or was deleted meanwhile
			err = fmt.Errorf("the record of chat %d of bot %s was created concurrently but is not found", chatID, botType)
		}
		return existing, err
	}
	if err != nil {
		return nil, err
	}
	return chat, nil
}

// hasLegacyHistory reports whether the chat used any bot before URLs had tokens, which its records keep as WebhookAuthAny or WebhookAuthUpgraded.
func (s *ChatStorage) hasLegacyHistory(ctx context.Context, chatID int64) (bool, error) {
	iter := s.collection.Query().Where("chat_id", "=", chatID).Get(ctx)
	defer iter.Stop()

	for {
		var chat Chat
		err := iter.Next(ctx, &chat)
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if chat.WebhookAuth == WebhookAuthAny || chat.WebhookAuth == WebhookAuthUpgraded {
			return true, nil
		}
	}
}

// UpgradeWebhookAuth makes the chat accept only webhook URLs with a token.
func (s *ChatStorage) UpgradeWebhookAuth(ctx context.Context, chat *Chat) error {
	key := &Chat{ChatID: chat.ChatID, BotType: chat.BotType}
	return s.collection.Update(ctx, key, docstore.Mods{"webhook_auth": WebhookAuthUpgraded})
}

// ClaimLegacyWarning reports whether the chat may be warned now about a rejected URL without a token, and records the warning if so.
// The chat may not be warned if it was warned less than interval ago, or if its record changed since it was read,
// which happens when concurrent requests claim the warning.
func (s *ChatStorage) ClaimLegacyWarning(ctx context.Context, chat *Chat, interval time.Duration) (bool, error) {
	now := time.Now()
	if now.Sub(chat.LegacyWarnedAt) < interval {
		return false, nil
	}

	err := s.collection.Update(ctx, chat, docstore.Mods{"legacy_warned_at": now})
	// A changed revision gives FailedPrecondition, or NotFound in MongoDB, as does a deleted record
	if code := gcerrors.Code(err); code == gcerrors.FailedPrecondition || code == gcerrors.NotFound {
		return false, nil
	}
	return err == nil, err
}

func (s *ChatStorage) DeleteChat(ctx context.Context, chat *Chat) error {
	return s.collection.Delete(ctx, chat)
}
