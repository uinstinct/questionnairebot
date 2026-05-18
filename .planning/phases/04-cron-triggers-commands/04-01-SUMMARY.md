---
plan_id: "04-01"
status: complete
date: 2026-05-18
---

# Plan 04-01 Summary — CronBus (single/multi/already-completed)

- `internal/commands/cron.go` — `CronBus` with buffered fire channel and `Run(ctx)` flushing on a 1s window. `Fire(slug, when)` is non-blocking (select/default drops on full buffer, with log).
- Filter rules in `flush`: drop fires for unknown slugs, drop fires for slugs whose session is already active, drop fires whose cycle is already completed (LastEntry.Status=="completed" AND scheduled_for equals the fire time formatted in q.Location).
- After filtering: 0 → silent; 1 → `Flow.StartQuestionnaire(slug, when)`; ≥2 → `PickerSender.SendPicker("📋 Multiple questionnaires are due. Which would you like to start?", options)` with one `start:<slug>:<RFC3339>` callback per slug.
- 4 tests under `-race`: single-fire-starts-session, already-completed-silent, multi-fire-picker, active-session-skips.

## Requirements Closed
SCHED-03, SCHED-04, SCHED-05, SCHED-06.

## Acceptance Evidence
`go test -race ./internal/commands/...` clean.
