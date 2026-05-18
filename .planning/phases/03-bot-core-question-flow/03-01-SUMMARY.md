---
plan_id: "03-01"
status: complete
date: 2026-05-18
---

# Plan 03-01 Summary — internal/bot polling loop + auth

- `internal/bot/auth.go` — `IsAuthorised(update, chatID)` uses `update.FromChat()`; silent on mismatch (no log, no reply).
- `internal/bot/bot.go` — `Bot` wraps `*tgbotapi.BotAPI`. `New(token, chatID, dispatcher)` constructs; `Run(ctx)` drives `GetUpdatesChan(Timeout=30)` and dispatches authorised updates. Cancellation via `<-ctx.Done()` plus `StopReceivingUpdates()`. `Send` and `SendMarkdown` provide the Sender contract.
- Dispatcher interface declared in this package (`Handle(ctx, sender Sender, update)`) so the handler package satisfies it structurally without an import cycle.

## Requirements Closed
- BOT-01 (long-polling), BOT-02 (silent unauthorised drop).

## Acceptance Evidence
- `go vet ./...` clean; `grep -E "log\.|fmt\.Print" internal/bot/auth.go` returns nothing.
- Auth gate is tested indirectly via the dispatcher tests (which exercise the Sender + Update path).
