package gitlab

import (
	"context"
	"fmt"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// GitLabTelegramService is a Telegram service for GitLab notifications
type GitLabTelegramService struct {
	*telegram.TelegramService
	pipelineStorage *PipelineStorage
}

// NewGitLabTelegramService creates a new GitLab Telegram service
func NewGitLabTelegramService(ctx context.Context, deps app.Deps) (*GitLabTelegramService, error) {
	s, err := telegram.NewTelegramService(botID, deps.Token, deps.Settings, deps.ChatStorage)
	if err != nil {
		return nil, err
	}

	pipelineStorage, err := NewPipelineStorage(ctx, deps.Store)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pipeline storage: %w", err)
	}

	gs := &GitLabTelegramService{
		TelegramService: s,
		pipelineStorage: pipelineStorage,
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

// SendOrUpdatePipelineMessage updates an existing pipeline message or creates a new one
func (s *GitLabTelegramService) SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string) error {
	ctx := context.Background()
	pipelineUpdateKey := CreatePipelineUpdateKey(pipelineURL, chatID)

	// Get existing pipeline mapping
	pipeline, err := s.pipelineStorage.GetPipeline(ctx, pipelineUpdateKey)
	if err != nil {
		return err
	}

	if pipeline == nil {
		// Pipeline not found, send new message
		msg, err := s.SendMessageWithResult(chatID, text)
		if err != nil {
			return err
		}

		// Store pipeline mapping
		pipeline := &Pipeline{
			PipelineUpdateKey: pipelineUpdateKey,
			MessageID:         msg.ID,
		}
		return s.pipelineStorage.SavePipeline(ctx, pipeline)
	} else {
		// Update the existing message
		if err := s.UpdateMessage(chatID, pipeline.MessageID, text); err != nil {
			return err
		}
		// Update the mapping timestamp
		return s.pipelineStorage.SavePipeline(ctx, pipeline)
	}
}
