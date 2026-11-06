package main

import (
	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/connectors/gitlab"
)

func main() {
	app.Run(gitlab.Connector())
}
