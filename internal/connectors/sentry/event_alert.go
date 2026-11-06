package sentry

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// eventProjectPattern extracts the project slug from the API URL of an event, the only place an event alert has it.
var eventProjectPattern = regexp.MustCompile(`/projects/[^/]+/([^/]+)/events/`)

func (s *SentryService) handleEventAlert(chatID int64, payload []byte, includeProject bool) error {
	var alert struct {
		Data struct {
			Event struct {
				Title       string `json:"title"`
				Culprit     string `json:"culprit"`
				Level       string `json:"level"`
				Environment string `json:"environment"`
				URL         string `json:"url"`
				WebURL      string `json:"web_url"`
			} `json:"event"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &alert); err != nil {
		return err
	}

	event := alert.Data.Event
	if event.Title == "" {
		return errors.New("event alert has no title")
	}

	var emoji string
	switch event.Level {
	case "fatal":
		emoji = "💥"
	case "error":
		emoji = "🔴"
	case "warning":
		emoji = "🟡"
	case "info":
		emoji = "🔵"
	case "debug":
		emoji = "⚪"
	default:
		emoji = "❔"
	}

	var project string
	if includeProject {
		if match := eventProjectPattern.FindStringSubmatch(event.URL); match != nil {
			project = match[1]
		}
	}

	message := formatHeadline(emoji, project, event.Title, event.WebURL)

	var details []string
	if culprit := formatText(event.Culprit, maxDetailLength); culprit != "" {
		details = append(details, "<code>"+culprit+"</code>")
	}
	if environment := formatText(event.Environment, maxDetailLength); environment != "" {
		details = append(details, environment)
	}
	if len(details) > 0 {
		message += "\n" + strings.Join(details, " · ")
	}

	return s.telegramSvc.SendMessage(chatID, message)
}
