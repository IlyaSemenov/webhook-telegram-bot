package gitlab

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

func (s *GitLabService) handleIssueEvent(chatID int64, payload []byte, includeProject bool) error {
	var event struct {
		ObjectAttributes struct {
			ID          int    `json:"id"`
			IID         int    `json:"iid"`
			Title       string `json:"title"`
			Description string `json:"description"`
			State       string `json:"state"`
			Action      string `json:"action"`
			URL         string `json:"url"`
		} `json:"object_attributes"`
		Project struct {
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
		} `json:"project"`
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Only notify on known issue actions
	if event.ObjectAttributes.Action != "open" &&
		event.ObjectAttributes.Action != "close" &&
		event.ObjectAttributes.Action != "reopen" {
		return nil
	}

	// Build message
	var message strings.Builder

	// Add emoji based on action
	var emoji string
	var action string
	switch event.ObjectAttributes.Action {
	case "open":
		emoji = "🆕"
		action = "opened"
	case "close":
		emoji = "✅"
		action = "closed"
	case "reopen":
		emoji = "🔄"
		action = "reopened"
	default:
		emoji = "❔"
		action = event.ObjectAttributes.Action
	}

	message.WriteString(emoji + " ")
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Project.Name)))
	}

	message.WriteString(fmt.Sprintf(
		"<b>%s</b> %s <a href=\"%s\">#%d %s</a>.",
		html.EscapeString(event.User.Name),
		action,
		event.ObjectAttributes.URL,
		event.ObjectAttributes.IID,
		html.EscapeString(event.ObjectAttributes.Title),
	))

	return s.telegramSvc.SendMessage(chatID, message.String())
}
