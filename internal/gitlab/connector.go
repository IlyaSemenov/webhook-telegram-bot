package gitlab

import (
	"context"

	"webhook-telegram-bot/internal/app"

	"github.com/gorilla/mux"
)

const botID = "gitlab"

// Connector returns the GitLab connector.
func Connector() app.Connector {
	return app.Connector{
		Name:     "GitLab",
		TokenEnv: "GITLAB_TELEGRAM_BOT_TOKEN",
		New: func(ctx context.Context, deps app.Deps) (app.Bot, error) {
			return NewGitLabTelegramService(ctx, deps)
		},
	}
}

// Mount registers the GitLab webhook and Telegram bot webhook endpoints.
func (s *GitLabTelegramService) Mount(router *mux.Router) {
	handler := NewGitLabHandler(s, NewGitLabService(s))
	router.HandleFunc("/"+botID+"/{chatID}", handler.HandleWebhook).Methods("POST")
	router.HandleFunc("/telegram/webhook/"+botID, s.WebhookHandler()).Methods("POST")
}
