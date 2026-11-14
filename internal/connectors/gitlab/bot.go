package gitlab

import (
	"context"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// GitLabTelegramService is a Telegram service for GitLab notifications
type GitLabTelegramService struct {
	*telegram.TelegramService
	*telegram.PipelineMessenger
}

// NewGitLabTelegramService creates a new GitLab Telegram service
func NewGitLabTelegramService(ctx context.Context, deps app.Deps) (*GitLabTelegramService, error) {
	s, err := telegram.NewTelegramService(botID, deps.Token, deps.Settings, deps.ChatStorage)
	if err != nil {
		return nil, err
	}

	pipelineMessenger, err := telegram.NewPipelineMessenger(ctx, s, deps.Store)
	if err != nil {
		return nil, err
	}

	gs := &GitLabTelegramService{
		TelegramService:   s,
		PipelineMessenger: pipelineMessenger,
	}

	s.RegisterCommandHandler("start", gs.handleStartCommand)
	s.RegisterCommandHandler("help", gs.handleHelpCommand)
	s.RegisterWebhookCommand(sourceName, webhookSetupText, formatAgentPrompt)

	return gs, nil
}

// sourceName is the name of the source in the /webhook reply.
const sourceName = "GitLab"

// webhookSetupText is the instructions for adding the webhook URL to GitLab, shown by the /webhook command.
const webhookSetupText = "<b>How to set up:</b>\n\n" +
	"1. Go to your GitLab project\n" +
	"2. Click on Settings → Webhooks\n" +
	"3. Click 'Add new webhook'\n" +
	"4. Paste the URL above in the 'URL' field\n" +
	"5. Select the events you want to receive:\n" +
	"   • Push events\n" +
	"   • Issues events\n" +
	"   • Merge request events\n" +
	"   • Pipeline events\n" +
	"6. Click 'Add webhook'\n\n" +
	"Use the 'Test' button to test the webhook.\n\n" +
	"<b>Optional parameters:</b>\n\n" +
	"• <code>?project=1</code> — include project name in messages\n" +
	"• <code>?branch=main&amp;branch=develop</code> — filter events by one or more branches"

// formatAgentPrompt returns the prompt for an AI agent that adds the webhook URL to GitLab with the glab CLI, shown by the /webhook command.
func formatAgentPrompt(webhookURL string) string {
	return "Add a webhook to the GitLab project in the current directory with the glab CLI. " +
		"If there is no project here, ask me which one to use.\n\n" +
		"- URL: " + webhookURL + "\n" +
		"- Triggers: push, issues, merge request, and pipeline events\n\n" +
		"Before creating the webhook, ask me whether to add query parameters to the URL:\n" +
		"- project=1 adds the project name to each message.\n" +
		"- branch=<name> limits pushes, pipelines, and merge requests to this branch; repeat it for several branches."
}

var (
	profileDescription = "Monitors GitLab repo activity via webhooks and sends updates to Telegram."
	whatCanThisBotDo   = "Generate a GitLab webhook URL, connect it to your repo, and receive events directly in Telegram."
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
			Description: "Get or reissue your unique GitLab webhook URL",
		},
	}
)

// InitBot initializes the GitLab Telegram bot
func (s *GitLabTelegramService) InitBot() error {
	if err := s.TelegramService.InitBot(); err != nil {
		return err
	}
	s.SetProfileDescription(profileDescription)
	s.SetWhatCanThisBotDo(whatCanThisBotDo)
	s.SetCommands(commands)
	return nil
}

// handleStartCommand handles the /start command
func (s *GitLabTelegramService) handleStartCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "👋 Welcome to GitLab Watch Bot!\n\n" +
		"I can forward GitLab webhook events to this chat.\n\n" +
		"Use /webhook to get your unique webhook URL."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}

// handleHelpCommand handles the /help command
func (s *GitLabTelegramService) handleHelpCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	text := "📚 <b>Available Commands</b>\n\n" +
		"• /start - Start the bot\n" +
		"• /help - Show this help message\n" +
		"• /webhook - Get or reissue your unique GitLab webhook URL\n\n" +
		"To set up webhooks, use the appropriate command and add the URL to your repository's webhook settings."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}
