package main

import (
	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/connectors/sentry"
)

func main() {
	app.Run(sentry.Connector())
}
