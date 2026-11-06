package telegram

import (
	"errors"
	"io"
	"log"
	"net/http"
)

// maxWebhookBodySize is the largest accepted webhook body, equal to the 25 MB cap of GitHub webhook payloads.
const maxWebhookBodySize = 25 << 20

// ReadWebhookBody returns the body of a webhook request, or responds with an error and returns false.
func ReadWebhookBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodySize))
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request body is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
		}
		return nil, false
	}
	return body, true
}
