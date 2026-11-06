package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"webhook-telegram-bot/internal/storage"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Settings holds the deployment settings shared by all Telegram bots.
type Settings struct {
	BaseURL   string
	SecretKey string
}

// TelegramService provides common functionality for Telegram bots
type TelegramService struct {
	botId              string // Internal bot ID (github or gitlab)
	bot                *bot.Bot
	chatStorage        *storage.ChatStorage
	baseURL            string
	webhookSecretToken string
}

func createWebhookSecretToken(secretKey string) string {
	return fmt.Sprintf("%x", sha256.Sum256(fmt.Appendf(nil, "%s:%s", secretKey, "telegram")))
}

// NewBTelegramService creates a new Telegram service
func NewTelegramService(botId string, token string, settings Settings, chatStorage *storage.ChatStorage) (*TelegramService, error) {
	webhookSecretToken := createWebhookSecretToken(settings.SecretKey)

	opts := []bot.Option{
		bot.WithSkipGetMe(), // We'll call getMe manually during Init
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			// Do nothing. The default implementation logs all unhandled update.
		}),
		bot.WithWebhookSecretToken(webhookSecretToken),
	}

	botInstance, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("Failed to create %s bot service: %w", botId, err)
	}

	return &TelegramService{
		bot:                botInstance,
		botId:              botId,
		chatStorage:        chatStorage,
		baseURL:            settings.BaseURL,
		webhookSecretToken: webhookSecretToken,
	}, nil
}

func (s *TelegramService) RegisterCommandHandler(command string, handler bot.HandlerFunc) {
	commandText := "/" + command
	commandTextPrefix := commandText + "@"
	matchFunc := func(update *models.Update) bool {
		if update.Message != nil {
			for _, e := range update.Message.Entities {
				if e.Offset == 0 {
					part := update.Message.Text[e.Offset : e.Offset+e.Length]
					if part == commandText || strings.HasPrefix(part, commandTextPrefix) {
						return true
					}
				}
			}
		}
		return false
	}
	s.bot.RegisterHandlerMatchFunc(matchFunc, handler)
}

// StartBot starts a Telegram bot
func (s *TelegramService) StartBot(ctx context.Context) {
	s.bot.StartWebhook(ctx)
}

// InitBot initializes a Telegram bot (in Lambda, this runs only once after deployment)
func (s *TelegramService) InitBot() error {
	// Get bot info
	ctx := context.Background()
	me, err := s.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get bot info for %s: %w", s.botId, err)
	}

	log.Printf("Init %s bot @%s", s.botId, me.Username)

	// Set up webhook
	webhookURL := s.baseURL + "/telegram/webhook/" + s.botId
	if err := s.SetWebhook(webhookURL); err != nil {
		return fmt.Errorf("Failed to set up %s bot webhook at %s: %w", s.botId, webhookURL, err)
	} else {
		log.Printf("Successfully set up %s bot webhook at %s", s.botId, webhookURL)
	}

	return nil
}

// SetWebhook sets up the webhook for the bot
func (s *TelegramService) SetWebhook(webhookURL string) error {
	ctx := context.Background()
	params := &bot.SetWebhookParams{
		URL:         webhookURL,
		SecretToken: s.webhookSecretToken,
	}
	_, err := s.bot.SetWebhook(ctx, params)
	return err
}

func (s *TelegramService) WebhookHandler() http.HandlerFunc {
	botWebhookHandler := s.bot.WebhookHandler()
	return func(res http.ResponseWriter, req *http.Request) {
		botWebhookHandler(res, req)
		// For now, always return success
		// TODO: somehow detect error coming from botWebhookHandler
		res.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(res).Encode(map[string]string{"status": "ok"}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func (s *TelegramService) GetChatWebhookURL(chatID int64) string {
	return fmt.Sprintf("%s/%s/%d", s.baseURL, s.botId, chatID)
}

// SetCommands sets the list of available commands for the bot
func (s *TelegramService) SetCommands(commands []models.BotCommand) {
	ctx := context.Background()
	params := &bot.SetMyCommandsParams{
		Commands: commands,
	}
	_, err := s.bot.SetMyCommands(ctx, params)
	if err != nil {
		log.Printf("Failed to set commands for %s bot: %v", s.botId, err)
	}
}

// SetWhatCanThisBotDo sets the "What can this bot do?" text ("description" in Telegram terms)
func (s *TelegramService) SetWhatCanThisBotDo(description string) {
	ctx := context.Background()
	params := &bot.SetMyDescriptionParams{
		Description: description,
	}
	_, err := s.bot.SetMyDescription(ctx, params)
	if err != nil {
		log.Printf("Failed to set profile description for %s bot: %v", s.botId, err)
	}
}

// SetProfileDescription sets the bot profile description ("short description" in Telegram terms)
func (s *TelegramService) SetProfileDescription(description string) {
	ctx := context.Background()
	params := &bot.SetMyShortDescriptionParams{
		ShortDescription: description,
	}
	_, err := s.bot.SetMyShortDescription(ctx, params)
	if err != nil {
		log.Printf("Failed to set short description for %s bot: %v", s.botId, err)
	}
}

// SendMessage sends a message to a Telegram chat
func (s *TelegramService) SendMessage(chatID int64, text string) error {
	_, err := s.SendMessageWithResult(chatID, text)
	return err
}

func (s *TelegramService) SendMessageOrLogError(chatID int64, text string) {
	if err := s.SendMessage(chatID, text); err != nil {
		log.Printf("Failed to send message from %s to chat %d: %v", s.botId, chatID, err)
	}
}

// SendMessageWithResult sends a message to a Telegram chat and returns the message info
func (s *TelegramService) SendMessageWithResult(chatID int64, text string) (*models.Message, error) {
	ctx := context.Background()
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	msg, err := s.bot.SendMessage(ctx, params)

	// Update chat status based on delivery result
	chat := &storage.Chat{
		ChatID:  chatID,
		BotType: s.botId,
	}

	if err == nil {
		// Save chat info (this will update existing or create new)
		if err := s.chatStorage.SaveChat(ctx, chat); err != nil {
			log.Printf("Failed to save chat info: %v", err)
		}
	} else if isBotBlockedError(err) {
		if err := s.chatStorage.DeleteChat(ctx, chat); err != nil {
			log.Printf("Failed to delete chat info: %v", err)
		}
	}

	return msg, err
}

// UpdateMessage updates an existing message in a Telegram chat
func (s *TelegramService) UpdateMessage(chatID int64, messageID int, text string) error {
	ctx := context.Background()
	params := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	}

	_, err := s.bot.EditMessageText(ctx, params)
	return err
}

// isBotBlockedError checks if the error indicates the bot was blocked or removed
func isBotBlockedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "bot was blocked") ||
		strings.Contains(errStr, "user is deactivated") ||
		strings.Contains(errStr, "chat not found") ||
		strings.Contains(errStr, "bot is not a member")
}
