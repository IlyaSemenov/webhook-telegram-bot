package sentry

import (
	"context"

	"webhook-telegram-bot/internal/app"

	"github.com/gorilla/mux"
)

const botID = "sentry"

// Connector returns the Sentry connector.
func Connector() app.Connector {
	return app.Connector{
		Name:     "Sentry",
		TokenEnv: "SENTRY_TELEGRAM_BOT_TOKEN",
		New: func(ctx context.Context, deps app.Deps) (app.Bot, error) {
			return NewSentryTelegramService(ctx, deps)
		},
	}
}

// Mount registers the Sentry webhook and Telegram bot webhook endpoints.
func (s *SentryTelegramService) Mount(router *mux.Router) {
	handler := NewSentryHandler(NewSentryService(s))
	router.HandleFunc("/"+botID+"/{chatID}", handler.HandleWebhook).Methods("POST")
	router.HandleFunc("/telegram/webhook/"+botID, s.WebhookHandler()).Methods("POST")
}
