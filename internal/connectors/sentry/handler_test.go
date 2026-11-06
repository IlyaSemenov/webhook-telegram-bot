package sentry

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

const issueWebhookBody = `{"action":"resolved","data":{"issue":{"title":"Boom","project":{"slug":"backend"}}}}`

// sendWebhook posts a webhook with the given resource header to a handler, skipping the header if it is empty.
func sendWebhook(handler *SentryHandler, chatIDVar, target, resource, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	request = mux.SetURLVars(request, map[string]string{"chatID": chatIDVar})
	if resource != "" {
		request.Header.Set("Sentry-Hook-Resource", resource)
	}
	response := httptest.NewRecorder()
	handler.HandleWebhook(response, request)
	return response
}

func TestSentryWebhookSendsMessageToChatFromURL(t *testing.T) {
	service, sender := newService()

	response := sendWebhook(NewSentryHandler(service), "123", "/sentry/123", "issue", issueWebhookBody)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if len(sender.sentMessages) != 1 || sender.sentMessages[0].chatID != 123 {
		t.Fatalf("messages = %+v, want one message to chat 123", sender.sentMessages)
	}
	if strings.Contains(sender.sentMessages[0].text, "backend") {
		t.Errorf("message includes the project without ?project: %q", sender.sentMessages[0].text)
	}
}

func TestSentryWebhookIncludesProjectOnRequest(t *testing.T) {
	service, sender := newService()

	response := sendWebhook(NewSentryHandler(service), "123", "/sentry/123?project=1", "issue", issueWebhookBody)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if len(sender.sentMessages) != 1 || !strings.Contains(sender.sentMessages[0].text, "<b>backend</b>: ") {
		t.Fatalf("messages = %+v, want one message with the project", sender.sentMessages)
	}
}

func TestSentryWebhookRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name      string
		chatIDVar string
		resource  string
		body      string
	}{
		{name: "invalid chat ID", chatIDVar: "abc", resource: "issue", body: issueWebhookBody},
		{name: "missing resource header", chatIDVar: "123", body: issueWebhookBody},
		{name: "malformed payload", chatIDVar: "123", resource: "issue", body: "{"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, sender := newService()

			response := sendWebhook(NewSentryHandler(service), tt.chatIDVar, "/sentry/"+tt.chatIDVar, tt.resource, tt.body)

			if response.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if len(sender.sentMessages) != 0 {
				t.Errorf("messages = %+v, want none", sender.sentMessages)
			}
		})
	}
}

func TestSentryWebhookReportsSendFailure(t *testing.T) {
	service, sender := newService()
	sender.err = errors.New("telegram is down")

	response := sendWebhook(NewSentryHandler(service), "123", "/sentry/123", "issue", issueWebhookBody)

	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
