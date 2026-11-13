package telegram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"webhook-telegram-bot/internal/storage"

	"github.com/gorilla/mux"
)

// WebhookURLNotice is the advice that the /webhook commands show with the URL.
const WebhookURLNotice = "Keep this URL private: anyone who has it can send messages to this chat."

// WebhookURLErrorText is the reply of the /webhook commands when the URL cannot be issued.
const WebhookURLErrorText = "❌ Failed to get the webhook URL. Please try again later."

// legacyWarningText is the warning about a rejected request to a URL without a token.
const legacyWarningText = "⚠️ A webhook request with an outdated URL was rejected. " +
	"Replace the URL with the one from /webhook in every webhook that sends events through this bot to this chat."

// legacyWarningInterval is the shortest interval between warnings about rejected URLs without a token to one chat from one bot.
const legacyWarningInterval = 24 * time.Hour

// maxWebhookBodySize is the largest accepted webhook body, equal to the 25 MB cap of GitHub webhook payloads.
const maxWebhookBodySize = 25 << 20

// ReadWebhookBody returns the body of a webhook request, or responds with an error and returns false.
func ReadWebhookBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodySize))
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
		}
		return nil, false
	}
	return body, true
}

// createWebhookToken returns the secret token of the webhook URL of a chat issued at issuedAt.
func createWebhookToken(secretKey string, botID string, chatID int64, issuedAt int64) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	fmt.Fprintf(mac, "webhook:%s:%d:%d", botID, chatID, issuedAt)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// GetChatWebhookURL returns the webhook URL of the chat, creating the record of the chat if it has none.
func (s *TelegramService) GetChatWebhookURL(ctx context.Context, chatID int64) (string, error) {
	chat, err := s.chatStorage.GetOrCreateChat(ctx, chatID, s.botId)
	if err != nil {
		return "", fmt.Errorf("failed to get the chat record: %w", err)
	}
	token := createWebhookToken(s.secretKey, s.botId, chatID, chat.WebhookIssuedAt)
	return fmt.Sprintf("%s/%s/%d/%s", s.baseURL, s.botId, chatID, token), nil
}

// AuthorizeWebhook returns next wrapped in the check of the webhook URL, which has the chatID and the optional token route variables.
// A URL with a token is accepted if the token matches the chat record, and makes the chat accept only such URLs from this bot.
// A URL without a token is accepted only while the chat accepts both kinds from this bot; see storage.WebhookAuthAny.
func (s *TelegramService) AuthorizeWebhook(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		chatID, err := ParseChatID(vars["chatID"])
		if err != nil {
			http.Error(w, "Failed to parse chatID from URL", http.StatusBadRequest)
			return
		}

		var accepted bool
		if token, hasToken := vars["token"]; hasToken {
			accepted, err = s.acceptWebhookToken(r.Context(), chatID, token)
		} else {
			accepted, err = s.acceptLegacyWebhook(r.Context(), chatID)
		}
		if err != nil {
			log.Printf("Failed to check %s webhook URL of chat %d: %v", s.botId, chatID, err)
			http.Error(w, "Failed to check webhook URL", http.StatusInternalServerError)
			return
		}
		if !accepted {
			log.Printf("Rejected %s webhook request for chat %d", s.botId, chatID)
			http.Error(w, "Invalid webhook URL", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// acceptWebhookToken reports whether token matches the record of the chat, and makes a matching chat accept only URLs with a token.
func (s *TelegramService) acceptWebhookToken(ctx context.Context, chatID int64, token string) (bool, error) {
	chat, err := s.chatStorage.GetChat(ctx, chatID, s.botId)
	if err != nil || chat == nil {
		return false, err
	}
	want := createWebhookToken(s.secretKey, s.botId, chatID, chat.WebhookIssuedAt)
	if !hmac.Equal([]byte(token), []byte(want)) {
		return false, nil
	}

	if chat.WebhookAuth == storage.WebhookAuthAny {
		if err := s.chatStorage.UpgradeWebhookAuth(ctx, chat); err != nil {
			return false, fmt.Errorf("failed to accept only URLs with a token: %w", err)
		}
	}
	return true, nil
}

// acceptLegacyWebhook reports whether the chat accepts a URL without a token, and warns a chat that has switched to tokens.
// A chat without a record accepts it if it used any bot before URLs had tokens, and then gets a record.
func (s *TelegramService) acceptLegacyWebhook(ctx context.Context, chatID int64) (bool, error) {
	chat, err := s.chatStorage.GetChat(ctx, chatID, s.botId)
	if err != nil {
		return false, err
	}
	if chat == nil {
		chat, err = s.chatStorage.CreateLegacyChat(ctx, chatID, s.botId)
		if err != nil || chat == nil {
			return false, err
		}
	}

	switch chat.WebhookAuth {
	case storage.WebhookAuthAny:
		return true, nil
	case storage.WebhookAuthUpgraded:
		s.warnAboutLegacyWebhook(ctx, chat)
	}
	return false, nil
}

// warnAboutLegacyWebhook claims and sends a warning about a rejected URL without a token.
func (s *TelegramService) warnAboutLegacyWebhook(ctx context.Context, chat *storage.Chat) {
	claimed, err := s.chatStorage.ClaimLegacyWarning(ctx, chat, legacyWarningInterval)
	if err != nil {
		log.Printf("Failed to claim the legacy webhook warning of %s chat %d: %v", s.botId, chat.ChatID, err)
		return
	}
	if claimed {
		s.SendMessageOrLogError(chat.ChatID, legacyWarningText)
	}
}
