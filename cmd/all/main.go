package main

import (
	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/connectors/github"
	"webhook-telegram-bot/internal/connectors/gitlab"
	"webhook-telegram-bot/internal/connectors/sentry"
)

func main() {
	app.Run(github.Connector(), gitlab.Connector(), sentry.Connector())
}
