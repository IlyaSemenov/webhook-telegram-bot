package gitlab

import (
	"encoding/json"
	"log"
	"net/http"

	"webhook-telegram-bot/internal/telegram"

	"github.com/gorilla/mux"
)

type GitLabHandler struct {
	telegramSvc *GitLabTelegramService
	gitlabSvc   *GitLabService
}

func NewGitLabHandler(telegramSvc *GitLabTelegramService, gitlabSvc *GitLabService) *GitLabHandler {
	return &GitLabHandler{
		telegramSvc: telegramSvc,
		gitlabSvc:   gitlabSvc,
	}
}

func (h *GitLabHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Get chat ID from URL
	vars := mux.Vars(r)
	chatID, err := telegram.ParseChatID(vars["chatID"])
	if err != nil {
		http.Error(w, "Failed to parse chatID from URL", http.StatusBadRequest)
		return
	}

	// Get branch filters from repeated query parameters if present
	branchFilters := r.URL.Query()["branch"]
	// Check if project name should be included in messages
	includeProject := r.URL.Query().Get("project") != ""

	// Read request body
	body, ok := telegram.ReadWebhookBody(w, r)
	if !ok {
		return
	}

	// Get event type from GitLab headers
	eventType := r.Header.Get("X-Gitlab-Event")
	if eventType == "" {
		log.Printf("Missing X-Gitlab-Event header")
		http.Error(w, "Missing X-Gitlab-Event header", http.StatusBadRequest)
		return
	}

	if err := h.gitlabSvc.HandleEvent(chatID, eventType, body, branchFilters, includeProject); err != nil {
		log.Printf("Failed to handle GitLab event: %v", err)
		http.Error(w, "Failed to handle GitLab event", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
