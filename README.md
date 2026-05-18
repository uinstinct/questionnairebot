# Telegram Questionnaire Bot

A self-hosted Telegram bot that delivers scheduled YAML-defined questionnaires
to a single authorised user, asks the questions one at a time, and persists
answers back to YAML on disk. No database — the file system is the only source
of truth.

## Quick Start

```bash
cp .env.example .env       # then fill in TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID
docker compose up -d
```

The bot reads every `data/<slug>/questionnaire.yaml`, registers one cron job per
questionnaire in its declared IANA timezone, and starts long-polling Telegram.

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | yes | Bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | yes | Numeric chat id of the single authorised user; all other chats are silently dropped |
| `DATA_DIR` | no | Path containing questionnaire subdirectories; defaults to `./data` (set to `/app/data` inside the Docker image) |

## Commands

- `/pull` — picker of pending questionnaires (skips past-due cycles first)
- `/status` — last-answered + next-trigger + state per questionnaire
- `/list` — every questionnaire's cron expression, timezone, and next trigger

## Running Tests

The default `go test ./...` run executes the unit-style tests only. Integration
and end-to-end tests are gated behind the `integration` build tag and live in
the same packages as the production code.

```bash
go test ./...                      # unit-style tests
go test ./... -tags integration    # unit + integration + E2E
```

End-to-end tests drive the bot against a real Telegram test bot. They require
two environment variables and are **skipped** (not failed) when either is
absent — so the same `-tags integration` command is safe to run on CI without
secrets.

| Variable | Description |
|----------|-------------|
| `TEST_TELEGRAM_BOT_TOKEN` | Bot token of a dedicated test bot (use a separate bot, not your production one) |
| `TEST_TELEGRAM_CHAT_ID` | Chat id the test bot may message during the E2E run |

```bash
export TEST_TELEGRAM_BOT_TOKEN=...   # from @BotFather, for a throwaway test bot
export TEST_TELEGRAM_CHAT_ID=...     # a chat the test bot can send into
go test ./... -tags integration -v
```
