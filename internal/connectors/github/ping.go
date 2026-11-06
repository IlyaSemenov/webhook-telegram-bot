package github

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"webhook-telegram-bot/internal/telegram"
)

func (s *GitHubService) handlePingEvent(chatID int64, payload []byte, includeProject bool) error {
	var event struct {
		Zen        string `json:"zen"`
		HookID     int    `json:"hook_id"`
		Repository struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Build message
	var message strings.Builder

	message.WriteString("✅ ")
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Repository.FullName)))
	}

	message.WriteString(fmt.Sprintf(
		"Webhook configured for %s.",
		telegram.FormatLink(event.Repository.HTMLURL, html.EscapeString(event.Repository.FullName)),
	))

	return s.telegramSvc.SendMessage(chatID, message.String())
}
