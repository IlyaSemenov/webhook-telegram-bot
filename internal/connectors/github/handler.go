package github

import (
	"encoding/json"
	"log"
	"net/http"

	"webhook-telegram-bot/internal/telegram"

	"github.com/gorilla/mux"
)

type GitHubHandler struct {
	telegramSvc *GitHubTelegramService
	githubSvc   *GitHubService
}

func NewGitHubHandler(telegramSvc *GitHubTelegramService, githubSvc *GitHubService) *GitHubHandler {
	return &GitHubHandler{
		telegramSvc: telegramSvc,
		githubSvc:   githubSvc,
	}
}

func (h *GitHubHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
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

	// Get event type from GitHub headers
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		log.Printf("Missing X-GitHub-Event header")
		http.Error(w, "Missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// Parse GitHub event
	if err := h.githubSvc.HandleEvent(chatID, eventType, body, branchFilters, includeProject); err != nil {
		log.Printf("Failed to parse GitHub event: %v", err)
		http.Error(w, "Failed to parse GitHub event", http.StatusBadRequest)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
