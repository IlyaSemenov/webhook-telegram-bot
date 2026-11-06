package sentry

import (
	"context"
	"fmt"
	"html"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SentryTelegramService is the Telegram bot of the Sentry connector.
type SentryTelegramService struct {
	*telegram.TelegramService
}

// NewSentryTelegramService creates the Telegram bot and registers its commands.
func NewSentryTelegramService(_ context.Context, deps app.Deps) (*SentryTelegramService, error) {
	s, err := telegram.NewTelegramService(botID, deps.Token, deps.Settings, deps.ChatStorage)
	if err != nil {
		return nil, err
	}

	ss := &SentryTelegramService{
		TelegramService: s,
	}

	s.RegisterCommandHandler("start", ss.handleStartCommand)
	s.RegisterCommandHandler("help", ss.handleHelpCommand)
	s.RegisterCommandHandler("webhook", ss.handleWebhookCommand)

	return ss, nil
}

var (
	profileDescription = "Sends Sentry alerts and issue updates to Telegram via webhooks."
	whatCanThisBotDo   = "Generate a Sentry webhook URL, connect it to your organization, and receive alerts directly in Telegram."
	commands           = []models.BotCommand{
		{
			Command:     "start",
			Description: "Start the bot",
		},
		{
			Command:     "help",
			Description: "Show help information",
		},
		{
			Command:     "webhook",
			Description: "Get your unique Sentry webhook URL",
		},
	}
)

// InitBot initializes the Sentry Telegram bot.
func (s *SentryTelegramService) InitBot() error {
	if err := s.TelegramService.InitBot(); err != nil {
		return err
	}
	s.SetProfileDescription(profileDescription)
	s.SetWhatCanThisBotDo(whatCanThisBotDo)
	s.SetCommands(commands)
	return nil
}

func (s *SentryTelegramService) handleStartCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "👋 Welcome to Sentry Watch Bot!\n\n" +
		"I can forward Sentry alerts and issue updates to this chat.\n\n" +
		"Use /webhook to get your unique webhook URL."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}

func (s *SentryTelegramService) handleHelpCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "📚 <b>Available Commands</b>\n\n" +
		"• /start - Start the bot\n" +
		"• /help - Show this help message\n" +
		"• /webhook - Get your unique Sentry webhook URL\n\n" +
		"To set up webhooks, use the /webhook command and add the URL to an internal integration in Sentry."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}

func (s *SentryTelegramService) handleWebhookCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	webhookURL := s.GetChatWebhookURL(update.Message.Chat.ID)

	text := fmt.Sprintf("🔗 <b>Your Sentry Webhook URL</b>\n\n<code>%s</code>\n\n", webhookURL) +
		"<b>How to set up:</b>\n\n" +
		"1. In Sentry, go to Settings → Developer Settings and create a new internal integration\n" +
		"2. Paste the URL above in the 'Webhook URL' field\n" +
		"3. Turn on 'Alert Rule Action'\n" +
		"4. Save the integration\n" +
		"5. In an alert, add an action that sends a notification via this integration\n\n" +
		"To receive issue state changes instead of alerts, by default for all projects, set the 'Issue &amp; Event' permission to Read and subscribe to the 'issue' webhook.\n\n" +
		"<b>Optional parameters:</b>\n\n" +
		"• <code>" + html.EscapeString("?project=1") + "</code> — include project name in messages"

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}
