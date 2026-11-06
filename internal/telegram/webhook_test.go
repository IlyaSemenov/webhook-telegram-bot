package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadWebhookBody(t *testing.T) {
	t.Run("returns a body of the maximum size", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxWebhookBodySize)))
		response := httptest.NewRecorder()

		body, ok := ReadWebhookBody(response, request)

		if !ok || len(body) != maxWebhookBodySize {
			t.Errorf("ok = %v, length = %d", ok, len(body))
		}
	})

	t.Run("rejects a larger body", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", maxWebhookBodySize+1)))
		response := httptest.NewRecorder()

		if _, ok := ReadWebhookBody(response, request); ok {
			t.Error("ok = true, want false")
		}
		if response.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
		}
	})
}
