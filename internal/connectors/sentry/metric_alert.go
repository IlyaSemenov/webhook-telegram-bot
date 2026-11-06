package sentry

import (
	"encoding/json"
	"errors"
	"strings"
)

func (s *SentryService) handleMetricAlert(chatID int64, payload []byte, includeProject bool) error {
	var alert struct {
		Action string `json:"action"`
		Data   struct {
			DescriptionTitle string `json:"description_title"`
			DescriptionText  string `json:"description_text"`
			WebURL           string `json:"web_url"`
			MetricAlert      struct {
				Projects []string `json:"projects"`
			} `json:"metric_alert"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &alert); err != nil {
		return err
	}

	if alert.Data.DescriptionTitle == "" {
		return errors.New("metric alert has no title")
	}

	var emoji string
	switch alert.Action {
	case "critical":
		emoji = "🔴"
	case "warning":
		emoji = "🟡"
	case "resolved":
		emoji = "✅"
	default:
		emoji = "❔"
	}

	var project string
	if includeProject {
		project = strings.Join(alert.Data.MetricAlert.Projects, ", ")
	}

	message := formatHeadline(emoji, project, alert.Data.DescriptionTitle, alert.Data.WebURL)
	if text := formatText(alert.Data.DescriptionText, maxDetailLength); text != "" {
		message += "\n" + text
	}

	return s.telegramSvc.SendMessage(chatID, message)
}
