# Webhook-Telegram Bot

Telegram bots that deliver GitHub, GitLab, and Sentry webhook events to Telegram chats, designed with [privacy](#privacy-policy) in mind.

To connect a chat:

1. Add the bot to your Telegram chat: [@github_watch_bot](https://t.me/github_watch_bot) for GitHub, [@gitlab_watch_bot](https://t.me/gitlab_watch_bot) for GitLab, or [@getsentry_bot](https://t.me/getsentry_bot) for Sentry.
2. Use the `/webhook` command to get the webhook URL of the chat.
3. Add this URL to the source as described in [GitHub](#github), [GitLab](#gitlab), or [Sentry](#sentry).
4. Events are now delivered to your Telegram chat.

The bots do not verify webhook signatures or secret tokens, so the endpoints do not authenticate requests: anyone who knows the chat ID can send notifications to that chat through the bot.

## GitHub

### Setup

1. In your repository, go to **Settings → Webhooks → Add webhook**.
2. Paste the URL from `/webhook` in the **Payload URL** field.
3. Set **Content type** to `application/json`.
4. Choose **Let me select individual events**, then select **Pushes** and **Workflow runs**.
5. Click **Add webhook**.

GitHub then sends a ping, and the chat receives a confirmation message.

### Events

- Ping: confirmation with the repository link
- Push: author, branch, and commits, or a force push or branch deletion notice
- Completed workflow run: workflow link and result

### URL Parameters

Add query parameters to the webhook URL and combine them as needed:

- `?project=1` adds the repository name to the beginning of each message, which is useful when monitoring several repositories in one chat:

  > 🚀 **owner/repo**: **John Doe** pushed to `main`.

- `?branch=<name>` limits pushes and workflow runs to this branch.
  Repeat the parameter to receive events for any of several branches, for example `?branch=main&branch=develop`.

## GitLab

### Setup

1. In your project, go to **Settings → Webhooks → Add new webhook**.
2. Paste the URL from `/webhook` in the **URL** field.
3. Select **Push events**, **Issues events**, **Merge request events**, and **Pipeline events**.
4. Click **Add webhook**.

Use the **Test** button to check the webhook.

### Events

- Push: author, branch, and commits, or a branch deletion notice
- Pipeline: pipeline link, status, branch or merge request, and jobs
- Merge request opened, merged, closed, reopened, approved, or unapproved: author, link, and source and target branches
- Issue opened, closed, or reopened: author and link

The bot sends one message per pipeline and updates it as the status changes.
Skipped pipelines and other merge request and issue actions, such as updates, send no message.

### URL Parameters

Add query parameters to the webhook URL and combine them as needed:

- `?project=1` adds the project name to the beginning of each message, which is useful when monitoring several projects in one chat:

  > 🚀 **My Project**: **John Doe** pushed to `main`.

- `?branch=<name>` limits pushes, pipelines, and merge requests to this branch; a merge request matches by its target branch.
  Repeat the parameter to receive events for any of several branches, for example `?branch=main&branch=develop`.

## Sentry

### Setup

The bot works with [internal integrations](https://docs.sentry.io/organization/integrations/integration-platform/internal-integration/), not with the legacy WebHooks plugin.
Each integration delivers to one chat, so create one integration per chat:

1. In Sentry, go to **Settings → Developer Settings** and create a new internal integration.
2. Paste the URL from `/webhook` in the **Webhook URL** field.
3. Choose what the chat receives:
   - For alerts, turn on **Alert Rule Action**, then add an action that sends a notification via this integration to an alert.
   - For issue state changes, set the **Issue & Event** permission to **Read** and subscribe to the **issue** webhook. By default, this subscription covers all projects in the organization, so use alerts to limit the chat to specific projects.

### Events

- Issue alert: event title and link, level, culprit, and environment
- Metric alert: alert title and link, status, and description
- Issue created, resolved, unresolved, or archived: issue title and link, and the new state

Sentry can deliver a metric alert as an issue alert, as self-hosted Sentry 26.10 does for a metric monitor connected to this integration; the message then has the format of an issue alert.

Other Sentry webhooks, such as issue assignments, errors, and comments, are accepted without sending a message.

### URL Parameters

`?project=1` adds the project slug to the beginning of each message.

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
- Sentry event data, such as error messages, stack traces, request data, and user data
- Personally identifiable information (PII)
- Data that could identify individuals or organizations

**Data flow**:

1. Webhook events are processed in real-time (never persisted)
2. Pipeline URLs are instantly hashed for status updates
3. Only necessary notification content is forwarded to Telegram
   - Sentry titles contain error messages, so an application that puts personal data in its errors also sends it to the chat
4. No message content remains in the system after delivery

**Retention rules**:

- Most data: Purged immediately after processing
- Pipeline tracking: Purged after 24 hours of inactivity
- All chat data: Removed when the bot is blocked

This is a privacy-focused relay bot that retains only the minimal data required for functionality.

## Builds

The repository builds a binary per bot and a binary that runs several bots in one process:

- `cmd/all` runs every bot that has a token.
- `cmd/github`, `cmd/gitlab`, and `cmd/sentry` run a single bot.

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

## Credits

This project was created as an alternative to [notifine](https://github.com/mhkafadar/notifine), to add features I needed: notifications on GitHub/GitLab workflow runs, and branch filtering.

## License

MIT
