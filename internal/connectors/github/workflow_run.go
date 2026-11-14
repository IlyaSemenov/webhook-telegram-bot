package github

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"webhook-telegram-bot/internal/telegram"
)

func (s *GitHubService) handleWorkflowRunEvent(chatID int64, payload []byte, branchFilters []string, includeProject bool) error {
	var event struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			Name       string `json:"name"`
			HTMLURL    string `json:"html_url"`
			RunAttempt int    `json:"run_attempt"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HeadBranch string `json:"head_branch"`
		} `json:"workflow_run"`
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Report runs that started and completed; a requested run may wait for a runner and may be skipped
	if event.Action != "in_progress" && event.Action != "completed" {
		return nil
	}

	// If none of the branch filters match the workflow run's branch, skip this event
	if len(branchFilters) > 0 && !slices.Contains(branchFilters, event.WorkflowRun.HeadBranch) {
		return nil
	}

	// Build message
	var message strings.Builder

	var emoji, status string
	if event.Action == "in_progress" {
		emoji = "▶️"
		status = "in progress"
	} else {
		switch event.WorkflowRun.Conclusion {
		case "success":
			emoji = "✅"
		case "failure":
			emoji = "❌"
		case "cancelled":
			emoji = "💨"
		case "skipped":
			emoji = "➖"
		default:
			emoji = "❔"
		}
		status = event.WorkflowRun.Conclusion
	}

	message.WriteString(emoji + " ")
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Repository.FullName)))
	}

	message.WriteString(fmt.Sprintf(
		"%s %s.",
		telegram.FormatLink(event.WorkflowRun.HTMLURL, html.EscapeString(event.WorkflowRun.Name)),
		html.EscapeString(status),
	))

	// Each attempt of a re-run gets its own message
	attemptURL := fmt.Sprintf("%s/attempts/%d", event.WorkflowRun.HTMLURL, event.WorkflowRun.RunAttempt)

	switch {
	case event.Action == "in_progress":
		return s.telegramSvc.SendOrUpdatePipelineMessage(chatID, attemptURL, message.String(), false)
	case event.WorkflowRun.Conclusion == "skipped":
		// A skipped run did nothing, so only a message about its start is updated
		return s.telegramSvc.FinishPipelineMessage(chatID, attemptURL, message.String())
	default:
		return s.telegramSvc.SendOrUpdatePipelineMessage(chatID, attemptURL, message.String(), true)
	}
}
