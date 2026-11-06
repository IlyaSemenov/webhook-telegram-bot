package sentry

import (
	"encoding/json"
	"errors"
)

func (s *SentryService) handleIssueEvent(chatID int64, payload []byte, includeProject bool) error {
	var event struct {
		Action string `json:"action"`
		Data   struct {
			Issue struct {
				Title   string `json:"title"`
				WebURL  string `json:"web_url"`
				Project struct {
					Slug string `json:"slug"`
				} `json:"project"`
			} `json:"issue"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	var emoji string
	var action string
	switch event.Action {
	case "created":
		emoji = "🆕"
		action = "created"
	case "resolved":
		emoji = "✅"
		action = "resolved"
	case "unresolved":
		emoji = "🔄"
		action = "unresolved"
	// Sentry documents the action as "archived" but also sends the older "ignored"
	case "ignored", "archived":
		emoji = "🙈"
		action = "archived"
	default:
		return nil
	}

	issue := event.Data.Issue
	if issue.Title == "" {
		return errors.New("issue has no title")
	}

	var project string
	if includeProject {
		project = issue.Project.Slug
	}

	return s.telegramSvc.SendMessage(chatID, formatHeadline(emoji, project, issue.Title, issue.WebURL)+" "+action+".")
}
