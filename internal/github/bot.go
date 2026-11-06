package github

import (
	"context"
	"fmt"
	"html"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type GitHubTelegramService struct {
	*telegram.TelegramService
}

func NewGitHubTelegramService(_ context.Context, deps app.Deps) (*GitHubTelegramService, error) {
	s, err := telegram.NewTelegramService(botID, deps.Token, deps.Settings, deps.ChatStorage)
	if err != nil {
		return nil, err
	}

	gs := &GitHubTelegramService{
		TelegramService: s,
	}

	s.RegisterCommandHandler("start", gs.handleStartCommand)
	s.RegisterCommandHandler("help", gs.handleHelpCommand)
	s.RegisterCommandHandler("webhook", gs.handleWebhookCommand)

	return gs, nil
}

var (
	profileDescription = "Monitors GitHub repo activity via webhooks and sends updates to Telegram."
	whatCanThisBotDo   = "Generate a GitHub webhook URL, connect it to your repo, and receive events directly in Telegram."
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
			Description: "Get your unique GitHub webhook URL",
		},
	}
)

// InitBot initializes the GitLab Telegram bot
func (s *GitHubTelegramService) InitBot() error {
	if err := s.TelegramService.InitBot(); err != nil {
		return err
	}
	s.SetProfileDescription(profileDescription)
	s.SetWhatCanThisBotDo(whatCanThisBotDo)
	s.SetCommands(commands)
	return nil
}

// handleStartCommand handles the /start command
func (s *GitHubTelegramService) handleStartCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "👋 Welcome to GitHub Watch Bot!\n\n" +
		"I can forward GitHub webhook events to this chat.\n\n" +
		"Use /webhook to get your unique webhook URL."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}

// handleHelpCommand handles the /help command
func (s *GitHubTelegramService) handleHelpCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "📚 <b>Available Commands</b>\n\n" +
		"• /start - Start the bot\n" +
		"• /help - Show this help message\n" +
		"• /webhook - Get your unique GitHub webhook URL\n\n" +
		"To set up webhooks, use the appropriate command and add the URL to your repository's webhook settings."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}

// handleGitHubCommand handles the /github command
func (s *GitHubTelegramService) handleWebhookCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	webhookURL := s.GetChatWebhookURL(update.Message.Chat.ID)

	// Create response message
	text := fmt.Sprintf("🔗 <b>Your GitHub Webhook URL</b>\n\n<code>%s</code>\n\n", webhookURL) +
		"<b>How to set up:</b>\n\n" +
		"1. Go to your GitHub repository\n" +
		"2. Click on Settings → Webhooks → Add webhook\n" +
		"3. Paste the URL above in the 'Payload URL' field\n" +
		"4. Set Content type to 'application/json'\n" +
		"5. Select the events you want to receive\n" +
		"6. Click 'Add webhook'\n\n" +
		"You'll receive a confirmation message when the webhook is set up correctly.\n\n" +
		"<b>Optional parameters:</b>\n\n" +
		"• <code>" + html.EscapeString("?project=1") + "</code> — include project name in messages\n" +
		"• <code>" + html.EscapeString("?branch=main&branch=develop") + "</code> — filter events by one or more branches"

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}
