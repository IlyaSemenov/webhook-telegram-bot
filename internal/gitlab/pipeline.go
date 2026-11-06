package gitlab

import (
	"cmp"
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"strings"
)

// PipelineEventData represents the parsed pipeline event data
type PipelineEventData struct {
	ObjectAttributes struct {
		ID             int    `json:"id"`
		Ref            string `json:"ref"`
		Status         string `json:"status"`
		DetailedStatus string `json:"detailed_status"`
		URL            string `json:"url"`
		Duration       int    `json:"duration"`
	} `json:"object_attributes"`
	Project struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	} `json:"project"`
	MergeRequest *struct {
		IID   int    `json:"iid"`
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"merge_request"`
	User struct {
		Name string `json:"name"`
	} `json:"user"`
	Builds []Build `json:"builds"`
}

type Build = struct {
	ID           int     `json:"id"`
	Stage        string  `json:"stage"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	AllowFailure bool    `json:"allow_failure"`
	Duration     float64 `json:"duration"`
}

func (s *GitLabService) handlePipelineEvent(chatID int64, payload []byte, branchFilters []string, includeProject bool) error {
	var event PipelineEventData

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	// Skip sending telegram update if the pipeline is skipped entirely
	if event.ObjectAttributes.Status == "skipped" {
		return nil
	}

	// If none of the branch filters match the pipeline ref, skip this event
	if len(branchFilters) > 0 && !slices.Contains(branchFilters, event.ObjectAttributes.Ref) {
		return nil
	}

	var message strings.Builder

	// Add emoji based on status
	var emoji string
	switch event.ObjectAttributes.Status {
	case "success":
		emoji = "✅"
	case "failed":
		emoji = "❌"
	case "running":
		emoji = "▶️"
	case "pending":
		emoji = "⏳"
	case "canceled":
		emoji = "💨"
	case "skipped":
		emoji = "➖"
	case "created":
		emoji = "◻️"
	case "waiting_for_resource":
		emoji = "🚦"
	case "preparing":
		emoji = "⚙️"
	case "manual":
		emoji = "✋"
	case "scheduled":
		emoji = "📅"
	default:
		emoji = "❔"
	}

	message.WriteString(emoji + " ")
	if includeProject {
		message.WriteString(fmt.Sprintf("<b>%s</b>: ", html.EscapeString(event.Project.Name)))
	}

	// Replace underscores with spaces in the status
	statusDisplay := strings.ReplaceAll(event.ObjectAttributes.Status, "_", " ")

	// Build the "for" part based on whether it's for a branch or MR
	var forPart string
	if event.MergeRequest != nil {
		forPart = fmt.Sprintf(
			"<a href=\"%s\">!%d %s</a>",
			event.MergeRequest.URL,
			event.MergeRequest.IID,
			html.EscapeString(event.MergeRequest.Title),
		)
	} else {
		forPart = fmt.Sprintf("<code>%s</code>", html.EscapeString(event.ObjectAttributes.Ref))
	}

	message.WriteString(fmt.Sprintf(
		"<a href=\"%s\">Pipeline #%d</a> %s for %s",
		event.ObjectAttributes.URL,
		event.ObjectAttributes.ID,
		html.EscapeString(statusDisplay),
		forPart,
	))

	// Add build information
	if len(event.Builds) > 0 {
		slices.SortFunc(event.Builds, func(a, b Build) int {
			return cmp.Compare(a.ID, b.ID)
		})
		message.WriteString(":\n")
		for _, build := range event.Builds {
			// Add emoji based on build status
			var buildEmoji string
			switch build.Status {
			case "success":
				buildEmoji = "✅"
			case "failed":
				if build.AllowFailure {
					buildEmoji = "⚠️"
				} else {
					buildEmoji = "❌"
				}
			case "running":
				buildEmoji = "▶️"
			case "pending":
				buildEmoji = "⏳"
			case "canceled":
				buildEmoji = "💨"
			case "canceling":
				buildEmoji = "⏹️"
			case "skipped":
				buildEmoji = "➖"
			case "created":
				buildEmoji = "◻️"
			case "manual":
				buildEmoji = "✋"
			case "preparing":
				buildEmoji = "⚙️"
			case "scheduled":
				buildEmoji = "📅"
			case "waiting_for_resource":
				buildEmoji = "🚦"
			default:
				buildEmoji = "❔"
			}

			// Format duration as string
			var durationStr string
			if build.Duration >= 1.0 {
				durationStr = fmt.Sprintf("%.0f seconds", build.Duration)
			} else if build.Duration > 0 {
				durationStr = fmt.Sprintf("%.1f seconds", build.Duration)
			} else {
				durationStr = ""
			}

			if durationStr != "" {
				message.WriteString(fmt.Sprintf(
					"%s <b>%s</b> (%s)\n",
					buildEmoji,
					html.EscapeString(build.Name),
					durationStr,
				))
			} else {
				message.WriteString(fmt.Sprintf(
					"%s <b>%s</b>\n",
					buildEmoji,
					html.EscapeString(build.Name),
				))
			}
		}
	}

	// Get pipeline URL for updating existing messages
	pipelineURL := event.ObjectAttributes.URL

	// Try to update existing message or create new one
	return s.telegramSvc.SendOrUpdatePipelineMessage(chatID, pipelineURL, message.String())
}
