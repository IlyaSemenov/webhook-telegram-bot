package github

import (
	"fmt"

	"webhook-telegram-bot/internal/services/telegram"
)

type GitHubService struct {
	telegramSvc telegram.MessageSender
}

func NewGitHubService(telegramSvc telegram.MessageSender) *GitHubService {
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
