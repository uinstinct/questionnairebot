---
phase: quick-260529-qsv
plan: "01"
subsystem: commands
tags: [telegram, slash-commands, startup, e2e]
dependency_graph:
  requires: []
  provides: [auto-registered slash commands via setMyCommands on startup]
  affects: [internal/commands, internal/bot, cmd/bot/main.go, internal/e2e]
tech_stack:
  added: []
  patterns: [app-layer command catalog, generic transport registration method, log-and-continue error handling]
key_files:
  created:
    - internal/commands/registry.go
    - internal/e2e/register_commands_test.go
  modified:
    - internal/bot/bot.go
    - cmd/bot/main.go
decisions:
  - "Command catalog lives in the app layer (internal/commands/registry.go) as a plain function, not a method on Adapter"
  - "bot.RegisterCommands takes the slice as a param — transport layer remains generic, no command semantics inside bot.go"
  - "startup registration failure is log-and-continue (WARN) — never fatal, per docker compose up-d reliability requirement"
  - "start excluded from registered commands; pull/status/list/help registered in that order without / prefix"
metrics:
  duration: "~5 minutes"
  completed_date: "2026-05-29"
---

# Quick Task 260529-qsv: Auto-register Slash Commands on Startup Summary

**One-liner:** Registered pull/status/list/help with Telegram setMyCommands on bot startup via a new app-layer Commands() catalog and a generic bot.RegisterCommands transport method.

## What Was Built

**internal/commands/registry.go** — App-layer command catalog. `Commands() []tgbotapi.BotCommand` returns exactly four commands (pull, status, list, help; start excluded; no "/" prefix) in a fixed order. Single source of truth.

**internal/bot/bot.go** — Added `func (b *Bot) RegisterCommands(cmds []tgbotapi.BotCommand) error`. Generic transport wrapper around `b.API.Request(tgbotapi.NewSetMyCommands(cmds...))`. No internal logging; mirrors the existing `AckCallback` pattern.

**cmd/bot/main.go** — Wiring: `b.RegisterCommands(commands.Commands())` called just before `go b.Run(ctx)`. On failure, logs `WARN: failed to register bot commands: <err>` and continues — a transient Telegram error does not kill the process.

**internal/e2e/register_commands_test.go** — `TestE2ERegisterCommands` registers the catalog against the real test bot and calls `GetMyCommands` to assert the response exactly matches the catalog (names, descriptions, count, order). Uses `requireTestEnv` / testify/require per e2e harness conventions.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add command catalog, RegisterCommands, startup wiring | 60871da | internal/commands/registry.go, internal/bot/bot.go, cmd/bot/main.go |
| 2 | E2E test proving setMyCommands round-trips against real test bot | e22688c | internal/e2e/register_commands_test.go |

## Verification

- `go build ./...` — passed
- `go vet ./...` — passed
- `go test ./internal/e2e/... -tags integration -run TestE2ERegisterCommands -v` — passed (1 test, real Telegram test bot)

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary changes introduced. setMyCommands is a bot-metadata write with no user data or elevated permissions.

## Self-Check: PASSED

- internal/commands/registry.go — FOUND
- internal/bot/bot.go — FOUND (modified)
- cmd/bot/main.go — FOUND (modified)
- internal/e2e/register_commands_test.go — FOUND
- Commit 60871da — FOUND
- Commit e22688c — FOUND
