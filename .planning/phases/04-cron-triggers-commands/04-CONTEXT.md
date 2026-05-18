# Phase 4: Cron Triggers & Commands - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning
**Mode:** Smart discuss — user-facing phase, locked by PRD strings + past-due algorithm

<domain>
## Phase Boundary

Replace the stubbed cron handler (which currently logs `cron fire (stub): <slug>`) and the stubbed `/pull`, `/status`, `/list` command bodies with real implementations:

1. **Cron handler** — when a cron fires for a single questionnaire:
   - If the current cycle is already completed (matching `scheduled_for` in answers.yaml) → silent no-op.
   - If no session is active → start that questionnaire's session (send question 1, write session.yaml).
   - When two or more crons fire within a 1-second window → group into a single `📋 Multiple questionnaires are due. Which would you like to start?` picker message with one inline keyboard button per slug; tapping starts that session. The others remain available via `/pull`.

2. **/pull** — apply the past-due skip algorithm against each questionnaire's answers.yaml, prepend `skipped` entries for unanswered cycles between the last recorded baseline and `now`, then present a picker with one button per questionnaire that has a pending (unanswered) next-upcoming cron. Replies:
   - `⚠️ You have an active session in progress. Please finish it first.` when a session is in progress.
   - `✅ All questionnaires are up to date. Nothing to answer right now.` when nothing is pending.

3. **/status** — table of every questionnaire: name | last answered | next scheduled (in own tz) | state (`✅ Done` / `🔄 In Progress` / `⏳ Pending`).

4. **/list** — table of every questionnaire: name | cron expression | timezone | next trigger RFC3339.

</domain>

<decisions>
## Implementation Decisions

### Cron grouping window
- Scheduler currently fires each cron independently; to group fires within a 1-second window, we add a `Bus` in front of the handler. The Bus buffers slugs that arrive within 1s and flushes either when the timer expires OR (for low-latency) when an unrelated update arrives. The Bus calls the real handler with a `[]string` slug list.
- For Phase 4 we implement this as: each cron fire calls `bus.Fire(slug, when)`. Internally Bus appends and waits 1s before flushing. Concurrency: a single goroutine handles flush; Fire just signals.

### Past-due skip algorithm (PRD §8)
- baseline := `LastEntry(answers.yaml).ScheduledFor` (parsed as time.Time); if absent, baseline := time.Now() - 1 year (window cap to avoid prepending hundreds of skips on a new questionnaire). Reasonable cap; matches PRD intent.
- For each cron tick strictly between `baseline` and `now`, prepend a `skipped` entry. Use `cron.NewParser` → `parser.Parse(schedule).Next(t)` to iterate.
- First cron strictly after `now` is the "next upcoming" surfaced by /pull.

### "Pending" classification for /pull
- A questionnaire is pending iff:
  - its past-due-resolved next-upcoming cron is in the future
  - AND there is no completed entry with that exact `scheduled_for`
  - AND no session is active for it.

### Picker inline keyboard
- Callback data format: `start:<slug>`. Bot dispatcher handles `update.CallbackQuery` with `Data` starting with `start:` → start that session.

### /status state field
- `🔄 In Progress` if session.Manager.Get(slug) != nil.
- `✅ Done` if last entry status==completed and matches the cron's most recent past-or-current scheduled cycle.
- `⏳ Pending` otherwise (default).

### Now provider
- All time queries flow through a `Clock func() time.Time` so tests can pin time. Default `time.Now`.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/storage.LastEntry(dataDir, slug)` already retrieves the newest entry (for baseline).
- `internal/scheduler.Handler` is the cron callback signature.
- `internal/session.Manager.Get(slug)` for active-session checks.
- `internal/handler.QuestionFlow.StartQuestionnaire(slug, scheduled)` for cron-fire boot.

### Established Patterns
- File I/O: temp+rename atomic (storage.prepend covers it for `skipped` entries via PrependSkipped).
- Mutex-protected concurrent access (session.Manager).
- Sender-decoupled handler logic for testability.

### Integration Points
- `cmd/bot/main.go` swaps the stub scheduler callback for `commands.NewCronBus(flow, scheduler-time-now).Fire`.
- Dispatcher updates: route `/pull`,`/status`,`/list` to commands package handlers; route `update.CallbackQuery` (Data prefixed `start:`) similarly.

</code_context>

<specifics>
## Specific Ideas

- A reusable helper `commands.NextTrigger(q, after time.Time) time.Time` that wraps `cron.NewParser` → `Next` so both `/list` and the past-due algorithm share one implementation.
- `commands.PastDueSkip(dataDir, slug, q, now, clock)` returns the count of skips prepended; `/pull` calls it on every questionnaire before computing the pending list.
- For the bus picker callback to start the questionnaire with the correct `scheduled_for`, the picker stores the "fired at" time in callback data → `start:<slug>:<unix>`.

</specifics>

<deferred>
## Deferred Ideas

- Resuming after restart while a picker was pending — out of scope; missed picker just becomes past-due skips on next /pull.
- Cancelling a picker — out of scope.

</deferred>
