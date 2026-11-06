# webhook-telegram-bot Agent Guide

## Overview

A relay that turns webhooks from external services into Telegram messages.

Read [README.md](README.md) before changing user-visible behavior, URL parameters, or stored data.

Extend this guide only with stable, non-obvious conventions, architecture, workflows, and gotchas.
Do not catalog files or restate what their names and locations show.

## Architecture

Each source, such as GitHub or GitLab, is a connector and a separate Telegram bot with its own token.

A connector consists of:

- `internal/<source>/handler.go`: the `POST /<source>/{chatID}` endpoint that parses the request.
- `internal/<source>/bot.go`: bot commands and `/telegram/webhook/<source>`.
- Event parsing and message formatting also live in `internal/<source>/`.

Follow these boundaries:

- Do not import one connector from another.
  Move code that several connectors really share to `internal/telegram/`.
- Keep source-specific concepts, such as branch filters, out of shared contracts.
  Each handler parses its own query parameters.
- Treat the Privacy Policy in the README as a contract.
  Do not store payload content, names, or URLs; store hashes where a lookup needs a key.
  Update the policy whenever stored data changes.

## Adding a connector

Besides the connector itself, update:

- `internal/server/server.go` and the token check in `internal/config/config.go`.
- `.env.example`, `terraform/variables.tf`, `terraform/terraform.tfvars.sample`, and the Lambda environment in `terraform/lambda.tf`.
- The header whitelist in `terraform/cloudfront.tf`, or the source's event headers will not reach the bot.
- `README.md`.

## Runtime and deployment

The same binary runs as a long-lived process on Dokku or as an AWS Lambda when `AWS_LAMBDA_FUNCTION_NAME` is set.

- Pushing to `main` deploys to Dokku through `.github/workflows/deploy-dokku.yaml`.
- Lambda freezes after the HTTP response, so finish all work before responding.
  Telegram commands run in a background worker after the response and can be lost on Lambda.
- Keep storage code portable across the docstore drivers: MongoDB, DynamoDB, and in-memory.
- Do not change `local.function_name` in `terraform/lambda.tf` for a live stack: it replaces the Lambda and its URL and recreates the DynamoDB tables empty.
- Do not print or commit `terraform/*.tfvars` and Terraform state, which contain bot tokens and secrets.

## Tests

Connector tests are golden-file tests over `testdata/`:

- Add a `testdata/<name>.json` fixture for each new event or message variant; tests discover fixtures automatically.
- Use `expect_no_output` in a fixture for an event that must not send a message.
- Generate `.golden` files with `make test-update` instead of editing them by hand, and review their diff.

## Commands

```sh
make test
make test-update
make dev
```

`make dev` needs a tunnel and a `.env`; see [docs/run-local.md](docs/run-local.md).

## Documentation

- Use One Sentence Per Line in new Markdown prose.
- Keep code excerpts in Markdown unindented, with a single blank line before and after.
