package github

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"
)

func (s *GitHubService) handleWorkflowRunEvent(chatID int64, payload []byte, branchFilters []string, includeProject bool) error {
	var event struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			Name       string `json:"name"`
			HTMLURL    string `json:"html_url"`
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

	// Only notify on completed workflow runs
	if event.Action != "completed" {
		return nil
	}

	// If none of the branch filters match the workflow run's branch, skip this event
	if len(branchFilters) > 0 && !slices.Contains(branchFilters, event.WorkflowRun.HeadBranch) {
		return nil
	}

	// Build message
	var message strings.Builder

	// Add emoji based on conclusion
	var emoji string
	switch event.WorkflowRun.Conclusion {
	case "success":
		emoji = "✅"
	case "failure":
		emoji = "❌"
	case "cancelled":
		emoji = "💨"
	default:
		emoji = "❔"
	}

	message.WriteString(emoji + " ")
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Repository.FullName)))
	}

	message.WriteString(fmt.Sprintf(
		"<a href=\"%s\">%s</a> %s.",
		event.WorkflowRun.HTMLURL,
		html.EscapeString(event.WorkflowRun.Name),
		html.EscapeString(event.WorkflowRun.Conclusion),
	))

	return s.telegramSvc.SendMessage(chatID, message.String())
}
