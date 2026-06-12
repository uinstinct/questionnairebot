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

## OpenTelemetry / Observability

Telemetry is **opt-in and disabled by default**. When no OTLP endpoint is
configured, the bot installs no-op providers — its behavior, logs, and network
activity are identical to a build without telemetry. Setting an OTLP endpoint
enables:

- **Traces** — `questionnaire.fire → flow.* → storage.*` and
  `telegram.update → flow.*`, so a single questionnaire can be followed from cron
  tick to persisted `answers.yaml`.
- **Metrics** — questionnaires-fired counter, answers-recorded counter,
  active-sessions observable gauge, and an errors counter.
- **Logs** — existing `log.Printf` output is also exported through the OTel logs
  SDK while still printing to stderr. (Per-line trace correlation is a documented
  follow-up; exported log records do not yet carry a trace id.)

All export is OTLP, configured via the standard `OTEL_*` environment variables.
Leave them unset to keep telemetry off.

| Variable | Required | Description |
|----------|----------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | OTLP collector endpoint. Setting this (or any signal-specific endpoint below) **enables** telemetry |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` / `_METRICS_ENDPOINT` / `_LOGS_ENDPOINT` | no | Per-signal endpoint overrides; any one of them also enables telemetry |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | no | `grpc` (default) or `http/protobuf` |
| `OTEL_SERVICE_NAME` | no | Service name in exported data; defaults to `questionnairebot` |
| `OTEL_RESOURCE_ATTRIBUTES` | no | Comma-separated `key=value` resource attributes (e.g. `deployment.environment=prod`) |

If the collector is unreachable, exporters retry/drop in the background — the
question flow never blocks on export, and providers flush on shutdown.

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
