# Running Locally

This guide explains how to run the Webhook-Telegram Bot locally for development and testing.

## Prerequisites

1. [mise](https://mise.jdx.dev) installed on your local machine
2. A Telegram bot token (from [@BotFather](https://t.me/BotFather))
3. A tunneling tool to expose your local server to the internet (Microsoft Dev Tunnels, ngrok, etc.)

## Step 1: Install Tools

All required tools (Go, Air) are managed via [mise](https://mise.jdx.dev). Install mise first:

```bash
curl https://mise.run | sh
```

Then install all project tools:

```bash
mise install
```

This will install Go and [Air](https://github.com/air-verse/air) (hot-reload tool for Go) as specified in `mise.toml`, with versions pinned in `mise.lock`.

## Step 2: Install Go Dependencies

```bash
go mod tidy
```

This command will download all Go module dependencies listed in the `go.mod` file and create/update the `go.sum` file with the correct checksums.

## Step 3: Set Up a Tunnel

Since the bot operates exclusively in webhook mode, you need to expose your local server to the internet.

### Using ngrok

1. Install ngrok if you haven't already:

```bash
brew install ngrok
```

2. Start ngrok:

```bash
ngrok http 8080
```

3. Note the HTTPS URL provided by ngrok (e.g., `https://a1b2c3d4.ngrok.io`)

## Step 4: Set Up Environment Variables

Create a `.env` file in the root directory of the project based on the `.env.example` file:

```
GITHUB_TELEGRAM_BOT_TOKEN=your_github_telegram_bot_token
GITLAB_TELEGRAM_BOT_TOKEN=your_gitlab_telegram_bot_token
SECRET_KEY=your_random_secret_key
BASE_URL=https://your-tunnel-url  # Use the URL from your tunnel
```

Set the token of at least one bot; a connector without a token is disabled.
All binaries read this file.

To generate a random secret key:

```bash
openssl rand -base64 32
```

## Step 5: Run the Bot

Start the bot in local mode with hot reload:

```bash
make dev
```

This will start every bot that has a token, with hot reload enabled, automatically restarting the server when you make code changes.

To run a single bot, pass its binary:

```bash
make dev BOT=github
```

Alternatively, you can run without hot reload, with or without `BOT`:

```bash
make run
```

The bot will start and automatically set up a webhook with Telegram using your tunnel URL.

If you encounter errors about missing dependencies or `go.sum` entries, run `go mod tidy` again to resolve them.

## Step 6: Test the Bot

1. Add your bot to a Telegram group or send a direct message to it
2. Use the `/webhook` command to get a unique webhook URL
3. Add this URL to your repository's webhook settings for testing

## Debugging

The bot logs information to the console, which can help with debugging.

If you're having issues with webhooks, check:

1. The tunnel console for incoming requests
2. The bot's console output for any errors
3. GitHub/GitLab's webhook delivery page for request/response details

## Local Development Tips

- `make run` does not reload code changes; restart it or use `make dev`
- You can use `go run ./cmd/bot` or `go run ./cmd/github` with the Go debugger for step-by-step debugging
- Keep your tunnel running in a separate terminal window while developing
