package main

import (
	"webhook-telegram-bot/internal/app"
	"webhook-telegram-bot/internal/github"
	"webhook-telegram-bot/internal/gitlab"
)

func main() {
	app.Run(github.Connector(), gitlab.Connector())
}
