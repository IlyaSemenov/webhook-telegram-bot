package sentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/storage"

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

	for _, path := range []string{"/sentry/123", "/telegram/webhook/sentry"} {
		var match mux.RouteMatch
		if !router.Match(httptest.NewRequest(http.MethodPost, path, nil), &match) {
			t.Errorf("POST %s is not routed", path)
		}
	}
}
