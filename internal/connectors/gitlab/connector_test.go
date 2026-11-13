package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/storage"
	"webhook-telegram-bot/internal/telegram"

	"github.com/gorilla/mux"
)

func TestConnectorMountsRoutes(t *testing.T) {
	bot, err := Connector().New(context.Background(), app.Deps{
		Token: "test-token",
		Store: storage.NewStore(""),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	router := mux.NewRouter()
	bot.Mount(router)

	for _, path := range []string{"/gitlab/123/token", "/gitlab/123", "/telegram/webhook/gitlab"} {
		var match mux.RouteMatch
		if !router.Match(httptest.NewRequest(http.MethodPost, path, nil), &match) {
			t.Errorf("POST %s is not routed", path)
		}
	}
}

func TestConnectorAuthorizesWebhooks(t *testing.T) {
	ctx := context.Background()
	store := storage.NewStore("")
	chatCollection, err := storage.OpenChatCollection(ctx, store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bot, err := Connector().New(ctx, app.Deps{
		Token:       "test-token",
		Settings:    telegram.Settings{SecretKey: "secret"},
		Store:       store,
		ChatStorage: storage.NewChatStorage(chatCollection),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	router := mux.NewRouter()
	bot.Mount(router)

	// The chat has no record, so neither URL is accepted
	for _, path := range []string{"/gitlab/123", "/gitlab/123/token"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("POST %s: status = %d, want %d", path, response.Code, http.StatusUnauthorized)
		}
	}
}
