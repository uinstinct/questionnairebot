---
plan_id: "04-02"
status: complete
date: 2026-05-18
---

# Plan 04-02 Summary — /pull + past-due algorithm

- `internal/commands/parse.go` — `parseSchedule(q)`, `NextTrigger(q, after)`, and `ApplyPastDueSkips(dataDir, q, now, clock)`. The skip walker uses `LastEntry.ScheduledFor` as baseline (or `now-1y` if no entries), iterates `cron.Schedule.Next` until reaching `now`, prepends a `skipped` entry per tick. Capped at `maxPastDueSkips = 365`.
- `internal/commands/pull.go` — `Pull.Handle(sender)` checks for active sessions (CMD-04 reply), runs past-due skip per slug, then emits picker or "all up to date" (CMD-05). `HandleCallback(sender, "start:<slug>:<RFC3339>")` starts the corresponding session; malformed payloads → "❌ Invalid selection." (CMD-05 / FR-7 / FR-8).
- 5 tests under `-race`: active-session, all-up-to-date (next cycle pre-completed), past-due-adds-skips-and-picker (baseline 3 days back → 3 skips prepended, 1-option picker), callback-starts-session, callback-malformed.

## Requirements Closed
CMD-01, CMD-02, CMD-03, CMD-04, CMD-05.

## Acceptance Evidence
`go test -race ./internal/commands/...` clean.
