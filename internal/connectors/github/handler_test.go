package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestGitHubWebhookAcceptsAnyRepeatedBranch(t *testing.T) {
	service, sender := newService()
	handler := NewGitHubHandler(nil, service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/github/123?branch=develop&branch=main",
		strings.NewReader(`{"ref":"refs/heads/main","after":"abc"}`),
	)
	request = mux.SetURLVars(request, map[string]string{"chatID": "123"})
	request.Header.Set("X-GitHub-Event", "push")
	response := httptest.NewRecorder()

	handler.HandleWebhook(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if len(sender.sentMessages) != 1 {
		t.Fatalf("messages = %d, want 1", len(sender.sentMessages))
	}
}
