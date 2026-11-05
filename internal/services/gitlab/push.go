package gitlab

import (
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"

	"webhook-telegram-bot/internal/services/telegram"
)

func (s *GitLabService) handlePushEvent(chatID int64, payload []byte, branchFilters []string, includeProject bool) error {
	var event struct {
		Ref      string `json:"ref"`
		Before   string `json:"before"`
		After    string `json:"after"`
		UserName string `json:"user_name"`
		Project  struct {
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
		} `json:"project"`
		Commits []struct {
			ID        string `json:"id"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
			URL       string `json:"url"`
			Author    struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commits"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Extract branch name from ref
	branch := strings.TrimPrefix(event.Ref, "refs/heads/")

	// If none of the branch filters match the current branch, skip this event
	if len(branchFilters) > 0 && !slices.Contains(branchFilters, branch) {
		return nil
	}

	// Build message
	var message strings.Builder

	// Check if this is a branch deletion event (after hash is all zeros)
	isBranchDeletion := event.After == "0000000000000000000000000000000000000000"

	// Write emoji based on event type
	if isBranchDeletion {
		message.WriteString("🗑️ ")
	} else {
		message.WriteString("🚀 ")
	}

	// Add project name if requested
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Project.Name)))
	}

	// Write event-specific message
	if isBranchDeletion {
		message.WriteString(fmt.Sprintf(
			"<b>%s</b> deleted branch <code>%s</code>",
			html.EscapeString(event.UserName),
			html.EscapeString(branch),
		))
	} else {
		// Regular push event
		message.WriteString(fmt.Sprintf(
			"<b>%s</b> pushed to <code>%s</code>",
			html.EscapeString(event.UserName),
			html.EscapeString(branch),
		))

		// Add commit information
		if len(event.Commits) > 0 {
			message.WriteString(":\n")
			for _, commit := range event.Commits {
				message.WriteString(fmt.Sprintf(
					"👉 <b>%s</b>: %s\n",
					html.EscapeString(commit.Author.Name),
					telegram.FormatCommitLink(commit.Message, commit.URL),
				))
			}
		}
	}

	return s.telegramSvc.SendMessage(chatID, message.String())
}
