package main

import (
	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/connectors/github"
)

func main() {
	app.Run(github.Connector())
}
