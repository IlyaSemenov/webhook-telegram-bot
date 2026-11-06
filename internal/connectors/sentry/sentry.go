package sentry

import (
	"fmt"
	"html"
	"strings"

	"webhook-telegram-bot/internal/telegram"
)

// Maximum lengths of the texts forwarded to Telegram, in characters.
// Sentry texts are arbitrary, while a Telegram message is limited to 4096 characters.
const (
	maxTitleLength   = 300
	maxDetailLength  = 200
	maxProjectLength = 100
)

// SentryService turns Sentry webhooks into Telegram messages.
type SentryService struct {
	telegramSvc telegram.MessageSender
}

// NewSentryService creates the service that sends messages through telegramSvc.
func NewSentryService(telegramSvc telegram.MessageSender) *SentryService {
	return &SentryService{
		telegramSvc: telegramSvc,
	}
}

// HandleEvent sends the message for a webhook of the given Sentry-Hook-Resource.
// Other resources are accepted without a message, because Sentry counts failed deliveries against the whole integration.
func (s *SentryService) HandleEvent(chatID int64, resource string, payload []byte, includeProject bool) error {
	switch resource {
	case "event_alert":
		return s.handleEventAlert(chatID, payload, includeProject)
	case "metric_alert":
		return s.handleMetricAlert(chatID, payload, includeProject)
	case "issue":
		return s.handleIssueEvent(chatID, payload, includeProject)
	default:
		return nil
	}
}

// formatText returns text on a single line, shortened to maxLength characters and escaped for Telegram HTML.
func formatText(text string, maxLength int) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > maxLength {
		text = string(runes[:maxLength]) + "…"
	}
	return html.EscapeString(text)
}

// formatHeadline returns the first line of a message: the emoji, the project if any, and the title linked to url.
func formatHeadline(emoji, project, title, url string) string {
	var headline strings.Builder

	headline.WriteString(emoji + " ")
	if project != "" {
		headline.WriteString(fmt.Sprintf("<b>%s</b>: ", formatText(project, maxProjectLength)))
	}

	if url == "" {
		headline.WriteString(formatText(title, maxTitleLength))
	} else {
		headline.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(url), formatText(title, maxTitleLength)))
	}

	return headline.String()
}
