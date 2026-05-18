---
plan_id: "04-03"
status: complete
date: 2026-05-18
---

# Plan 04-03 Summary — /status, /list, dispatcher wiring, main wire-up

- `internal/commands/status.go` — sorted-by-slug status table with `📊 Status:` header. Each row: `Name | last=<RFC3339-in-tz or Never> | next=<RFC3339-in-tz> | <state>` where state ∈ `🔄 In Progress`, `✅ Done`, `⏳ Pending`.
- `internal/commands/list.go` — sorted-by-slug `📋 Questionnaires:` table. Each row: `Name | cron=<schedule> | tz=<timezone> | next=<RFC3339-in-tz>`.
- `internal/commands/adapter.go` — `commands.Adapter` implements `handler.CommandHandler` (HandlePull, RenderStatus, RenderList, HandleStartCallback) without creating a cycle.
- `internal/handler/dispatcher.go` — extended with `CommandHandler` interface and `Attach(commands)`. Routes `/pull` → `HandlePull`, `/status` → `RenderStatus`, `/list` → `RenderList`. `update.CallbackQuery` with Data prefix `start:` → ack + `HandleStartCallback`.
- `internal/bot/bot.go` — Sender widened with `SendPicker(text, []PickerOption) error` and `AckCallback(id) error`. `*Bot` implementations build `tgbotapi.InlineKeyboardMarkup` and call `tgbotapi.NewCallback`.
- `cmd/bot/main.go` — replaces stub cron handler with `bus.Fire(slug, time.Now())`; spawns `CronBus.Run` goroutine; wires Pull/Status/List into the dispatcher via the adapter.

Tests:
- `format_test.go` — TestListIncludesAllFields, TestStatusStates (active / done / pending labels + slug ordering).
- `dispatcher_test.go` extended — TestDispatcherPullRoutes, TestDispatcherStatusList, TestDispatcherCallbackStart (verifies AckCallback fired with the right ID).

## Requirements Closed
CMD-06, CMD-07.

## Acceptance Evidence
`go build ./...`, `go vet ./...`, `go test -race ./internal/...` all clean.
