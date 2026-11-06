# Webhook-Telegram Bot

A Telegram bot that delivers GitHub and GitLab webhook events to Telegram chats.

It's an alternative to [notifine](https://github.com/mhkafadar/notifine) which is developed very slowly and lacked important features that I needed: notifications on GitHub/GitLab workflow runs, and branch filtering.

## Features

- Receive GitHub and GitLab webhook events and forward them to Telegram chats
- Support for multiple events:
  - Push events
  - GitHub workflow run events
  - GitLab pipeline events with real-time updates
  - GitLab merge request events
  - Branch filtering for push, pipeline, workflow run, and merge request events

## How It Works

1. Add [@github_watch_bot](https://t.me/github_watch_bot) or [@gitlab_watch_bot](https://t.me/gitlab_watch_bot) to your Telegram chat
2. Use `/webhook` command to get a unique webhook URL for your repository
3. Add this URL to your GitHub/GitLab repository's webhook settings
4. Events are now delivered to your Telegram chat

### URL Parameters

You can customize the webhook behavior by adding query parameters to your webhook URL. Parameters can be mixed and matched to suit your needs.

#### Project Name Prefix

Add the `?project=1` query parameter to include the project/repository name at the beginning of each notification message. This is useful when monitoring multiple repositories in the same chat.

**Without project parameter:**

> 🚀 **John Doe** pushed to `main`.

**With `?project=1`:**

> 🚀 **My Project**: **John Doe** pushed to `main`.

#### Branch Filtering

Add `?branch=<name>` to your webhook URL to only receive events for a specific branch.
Repeat the parameter to receive events for any of several branches, for example `?branch=main&branch=develop`.
Branch filtering works for both GitHub and GitLab.

**Filtered events:**

| Event | Branch matched against |
| --- | --- |
| Push | Pushed branch |
| GitHub workflow_run | Workflow's `head_branch` |
| GitLab Pipeline | Pipeline `ref` |
| GitLab Merge Request | Target branch |

Events not tied to a branch (e.g. issues) are not affected by this filter.

## Privacy Policy

This bot is designed with privacy as a core principle. Here’s how data is handled:

**Stored data:**

- **Chat identifiers**:
  - Telegram chat IDs (numeric only) and timestamp of the last handled event
  - Automatically removed if the bot is blocked by the chat
- **GitLab pipeline tracking**:
  - SHA-256 hashes of pipeline identifiers (irreversible, cannot reveal original URLs)
  - Associated Telegram message IDs (for updating status messages)
  - Automatically purged after 24 hours of pipeline inactivity

**Explicitly NOT stored:**

- Repository/pipeline URLs (only hashes)
- Names of users, organizations, or repositories
- Commit messages, code content, or file changes
- Personally identifiable information (PII)
- Data that could identify individuals or organizations

**Data flow**:

1. Webhook events are processed in real-time (never persisted)
2. Pipeline URLs are instantly hashed for status updates
3. Only necessary notification content is forwarded to Telegram
4. No message content remains in the system after delivery

**Retention rules**:

- Most data: Purged immediately after processing
- Pipeline tracking: Purged after 24 hours of inactivity
- All chat data: Removed when the bot is blocked

This is a privacy-focused relay bot that retains only the minimal data required for functionality.

## Builds

The repository builds a binary per bot and a binary that runs several bots in one process:

- `cmd/all` runs every bot that has a token.
- `cmd/github` and `cmd/gitlab` run a single bot.

All binaries read the same environment variables, and a bot has the same URLs in any binary.

## Developer Documentation

- [Running Locally](docs/run-local.md)
- [Deploying to AWS Lambda with Terraform](docs/deploy-aws-lambda.md)
- [Deploying to Dokku](docs/deploy-dokku.md)

## Requirements

- Go 1.25 or higher
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))
- AWS account (for Lambda deployment)
- Terraform 1.0+ (for infrastructure deployment, if using AWS Lambda)

## License

MIT
