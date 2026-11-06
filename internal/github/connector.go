package github

import (
	"context"

	"webhook-telegram-bot/internal/app"

	"github.com/gorilla/mux"
)

const botID = "github"

// Connector returns the GitHub connector.
func Connector() app.Connector {
	return app.Connector{
		Name:     "GitHub",
		TokenEnv: "GITHUB_TELEGRAM_BOT_TOKEN",
		New: func(ctx context.Context, deps app.Deps) (app.Bot, error) {
			return NewGitHubTelegramService(ctx, deps)
		},
	}
}

// Mount registers the GitHub webhook and Telegram bot webhook endpoints.
func (s *GitHubTelegramService) Mount(router *mux.Router) {
	handler := NewGitHubHandler(s, NewGitHubService(s))
	router.HandleFunc("/"+botID+"/{chatID}", handler.HandleWebhook).Methods("POST")
	router.HandleFunc("/telegram/webhook/"+botID, s.WebhookHandler()).Methods("POST")
}
