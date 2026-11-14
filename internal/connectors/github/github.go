package github

import (
	"fmt"

	"webhook-telegram-bot/internal/telegram"
)

// PipelineMessageSender extends telegram.MessageSender with workflow run message updates.
type PipelineMessageSender interface {
	telegram.MessageSender
	SendOrUpdatePipelineMessage(chatID int64, pipelineURL string, text string, final bool) error
	FinishPipelineMessage(chatID int64, pipelineURL string, text string) error
}

type GitHubService struct {
	telegramSvc PipelineMessageSender
}

func NewGitHubService(telegramSvc PipelineMessageSender) *GitHubService {
	return &GitHubService{
		telegramSvc: telegramSvc,
	}
}

func (s *GitHubService) HandleEvent(chatID int64, eventType string, payload []byte, branchFilters []string, includeProject bool) error {
	switch eventType {
	case "ping":
		return s.handlePingEvent(chatID, payload, includeProject)
	case "push":
		return s.handlePushEvent(chatID, payload, branchFilters, includeProject)
	case "workflow_run":
		return s.handleWorkflowRunEvent(chatID, payload, branchFilters, includeProject)
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
}
