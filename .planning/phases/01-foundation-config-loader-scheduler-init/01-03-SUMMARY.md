---
plan_id: "01-03"
status: complete
date: 2026-05-18
---

# Plan 01-03 Summary — internal/scheduler

## What Was Built

- `internal/scheduler/scheduler.go` — `Handler` callback type, `Scheduler` struct, `New(qs, h) (*Scheduler, error)` constructs one `*cron.Cron` per questionnaire using `cron.WithLocation(q.Location)` and registers the handler via `AddFunc(q.Schedule, ...)`. Errors wrapped as `scheduler: <slug>: <err>`.
- `Start(ctx)` starts each cron and logs `Next trigger for <slug>: <RFC3339>` (timezone offset preserved). A background goroutine watches `<-ctx.Done()` and calls `Stop()` to tear down all crons.
- `Stop()` is idempotent enough to be called twice (main.go calls it explicitly after `<-ctx.Done()`).
- `cmd/bot/main.go` wires the scheduler with a stub handler that logs `cron fire (stub): <slug>`.

## Requirements Closed

- SCHED-01 — one cron job per questionnaire using its schedule + timezone (via `cron.WithLocation`).
- SCHED-02 — next-trigger RFC3339 log line per questionnaire on startup.

## Acceptance Evidence

Run with two questionnaires:
```
Loaded configuration: chat_id=1 data_dir=/tmp/qb-test
Loaded 2 questionnaire(s): [daily-standup, weekly]
Next trigger for daily-standup: 2026-05-19T09:00:00+05:30
Next trigger for weekly: 2026-05-18T10:00:00Z
shutdown complete
```

- Asia/Kolkata questionnaire renders `+05:30`.
- UTC questionnaire renders `Z`.
- SIGTERM produces clean shutdown (exit 0).
- `go build ./...` and `go vet ./...` clean.
