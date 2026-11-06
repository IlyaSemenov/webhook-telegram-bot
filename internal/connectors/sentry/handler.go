package sentry

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"webhook-telegram-bot/internal/telegram"

	"github.com/gorilla/mux"
)

// SentryHandler serves the webhook endpoint of the Sentry connector.
type SentryHandler struct {
	sentrySvc *SentryService
}

// NewSentryHandler creates the webhook endpoint handler.
func NewSentryHandler(sentrySvc *SentryService) *SentryHandler {
	return &SentryHandler{
		sentrySvc: sentrySvc,
	}
}

// HandleWebhook parses a webhook of a Sentry integration and sends its message to the chat from the URL.
func (h *SentryHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatID, err := telegram.ParseChatID(vars["chatID"])
	if err != nil {
		http.Error(w, "Failed to parse chatID from URL", http.StatusBadRequest)
		return
	}

	includeProject := r.URL.Query().Get("project") != ""

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	resource := r.Header.Get("Sentry-Hook-Resource")
	if resource == "" {
		log.Printf("Missing Sentry-Hook-Resource header")
		http.Error(w, "Missing Sentry-Hook-Resource header", http.StatusBadRequest)
		return
	}

	if err := h.sentrySvc.HandleEvent(chatID, resource, body, includeProject); err != nil {
		log.Printf("Failed to handle Sentry event: %v", err)
		http.Error(w, "Failed to handle Sentry event", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
