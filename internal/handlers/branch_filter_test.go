package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	githubService "webhook-telegram-bot/internal/services/github"
	gitlabService "webhook-telegram-bot/internal/services/gitlab"

	"github.com/gorilla/mux"
)

type webhookMessageSender struct {
	messages int
}

func (m *webhookMessageSender) SendMessage(_ int64, _ string) error {
	m.messages++
	return nil
}

func (m *webhookMessageSender) SendOrUpdatePipelineMessage(_ int64, _ string, _ string) error {
	m.messages++
	return nil
}

func TestGitHubWebhookAcceptsAnyRepeatedBranch(t *testing.T) {
	sender := &webhookMessageSender{}
	handler := NewGitHubHandler(nil, githubService.NewGitHubService(sender))
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
	if sender.messages != 1 {
		t.Fatalf("messages = %d, want 1", sender.messages)
	}
}

func TestGitLabWebhookAcceptsAnyRepeatedBranch(t *testing.T) {
	sender := &webhookMessageSender{}
	handler := NewGitLabHandler(nil, gitlabService.NewGitLabService(sender))
	request := httptest.NewRequest(
		http.MethodPost,
		"/gitlab/123?branch=develop&branch=main",
		strings.NewReader(`{"ref":"refs/heads/main","after":"abc"}`),
	)
	request = mux.SetURLVars(request, map[string]string{"chatID": "123"})
	request.Header.Set("X-Gitlab-Event", "Push Hook")
	response := httptest.NewRecorder()

	handler.HandleWebhook(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if sender.messages != 1 {
		t.Fatalf("messages = %d, want 1", sender.messages)
	}
}
