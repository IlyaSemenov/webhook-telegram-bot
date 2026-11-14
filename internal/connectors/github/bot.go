package github

import (
	"context"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type GitHubTelegramService struct {
	*telegram.TelegramService
	*telegram.PipelineMessenger
}

func NewGitHubTelegramService(ctx context.Context, deps app.Deps) (*GitHubTelegramService, error) {
	s, err := telegram.NewTelegramService(botID, deps.Token, deps.Settings, deps.ChatStorage)
	if err != nil {
		return nil, err
	}

	pipelineMessenger, err := telegram.NewPipelineMessenger(ctx, s, deps.Store)
	if err != nil {
		return nil, err
	}

	gs := &GitHubTelegramService{
		TelegramService:   s,
		PipelineMessenger: pipelineMessenger,
	}

	s.RegisterCommandHandler("start", gs.handleStartCommand)
	s.RegisterCommandHandler("help", gs.handleHelpCommand)
	s.RegisterWebhookCommand(sourceName, webhookSetupText, formatAgentPrompt)

	return gs, nil
}

// sourceName is the name of the source in the /webhook reply.
const sourceName = "GitHub"

// webhookSetupText is the instructions for adding the webhook URL to GitHub, shown by the /webhook command.
const webhookSetupText = "<b>How to set up:</b>\n\n" +
	"1. Go to your GitHub repository\n" +
	"2. Click on Settings → Webhooks → Add webhook\n" +
	"3. Paste the URL above in the 'Payload URL' field\n" +
	"4. Set Content type to 'application/json'\n" +
	"5. Select the events you want to receive\n" +
	"6. Click 'Add webhook'\n\n" +
	"You'll receive a confirmation message when the webhook is set up correctly.\n\n" +
	"<b>Optional parameters:</b>\n\n" +
	"• <code>?project=1</code> — include project name in messages\n" +
	"• <code>?branch=main&amp;branch=develop</code> — filter events by one or more branches"

// formatAgentPrompt returns the prompt for an AI agent that adds the webhook URL to GitHub with the gh CLI, shown by the /webhook command.
func formatAgentPrompt(webhookURL string) string {
	return "Add a webhook to the GitHub repository in the current directory with the gh CLI. " +
		"If there is no repository here, ask me which one to use.\n\n" +
		"- Payload URL: " + webhookURL + "\n" +
		"- Content type: application/json\n" +
		"- Events: push, workflow_run\n\n" +
		"Before creating the webhook, ask me whether to add query parameters to the URL:\n" +
		"- project=1 adds the repository name to each message.\n" +
		"- branch=<name> limits pushes and workflow runs to this branch; repeat it for several branches."
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
			Description: "Get or reissue your unique GitHub webhook URL",
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
		"• /webhook - Get or reissue your unique GitHub webhook URL\n\n" +
		"To set up webhooks, use the appropriate command and add the URL to your repository's webhook settings."

	s.SendMessageOrLogError(update.Message.Chat.ID, text)
}
