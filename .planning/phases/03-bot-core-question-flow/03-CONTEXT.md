# Phase 3: Bot Core & Question Flow - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning
**Mode:** Smart discuss — user-facing phase, but PRD locks every visible string and flow

<domain>
## Phase Boundary

Telegram long-polling loop:
- Drops updates from any chat ID other than `TELEGRAM_CHAT_ID` silently (no log, no reply).
- Routes slash commands (`/pull`, `/status`, `/list`, `/start`) to handler stubs.
- Free text → answer for current session OR fallback help message if no session is active.
- One-question-at-a-time flow with optional `_Example: ...` italic line.
- Persists progress to `session.yaml` after every answer; finalises on the last answer (writes `completed` entry, deletes `session.yaml`, sends a completion message).
- On startup, restores any in-progress sessions from disk; if a restored session is already fully answered, finalises it immediately.

Out of scope (Phase 4):
- Implementing the cron callback (currently `cron fire (stub)`).
- Real bodies for `/pull`, `/status`, `/list` (placeholder replies "Phase 4 will implement").

</domain>

<decisions>
## Implementation Decisions

### Telegram client
- Library: `github.com/go-telegram-bot-api/telegram-bot-api/v5` (already a dep).
- Long-polling via `bot.GetUpdatesChan(cfg)` with `Timeout: 30` (seconds).
- Parse mode: `Markdown` (the v5 client supports `tgbotapi.ModeMarkdown`); the italic line is `_Example: ...` per PRD FR-10.

### Auth gate
- Single function `authorise(update *tgbotapi.Update, chatID int64) bool` checks `update.FromChat()` (handles both messages and callback queries). If mismatch → return false; caller silently `continue`s. **No log line on rejection.** This is a security decision (FR-14 / US-012).

### Session lifecycle hooks
- `handler.OnText(ctx, slug-or-empty, text)` is the entry point for free text. If there's a single active session in `session.Manager`, that's the slug. If 0 → help fallback. If >1 → defer to picker (Phase 4); for now in Phase 3 we only support a single active session at a time (the cron picker arrives in Phase 4).
- `handler.StartQuestionnaire(slug)` creates the session, sends the first question. Phase 4 cron handler will call this.
- Completion message: `✅ {Name} complete! Answers saved.` (PRD US-007 AC-1).

### Resume semantics
- On startup, walk each loaded questionnaire, call `sessionManager.LoadFromDisk(slug)`.
- If loaded AND `CurrentQuestionIndex == len(questions)` → finalise (write completed entry from session, delete session.yaml). Otherwise just keep it in memory; the next user text will trigger sending `questions[CurrentQuestionIndex]`.

### Help message
- `Send /pull to start a questionnaire now, /status for state, or /list to see schedules.` (concise; Phase 4 expands as needed.)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config.Config` carries BotToken + ChatID (already typed int64).
- `internal/loader.Questionnaire` carries `Slug`, `Name`, `Questions`, `Location`.
- `internal/session.Manager` already provides `Start`, `RecordAnswer`, `Get`, `Delete`, `LoadFromDisk`.
- `internal/storage.PrependCompleted` writes the completed entry.

### Established Patterns
- Errors returned not panicked.
- Atomic write via temp+rename.
- Mutex-guarded session state.

### Integration Points
- `cmd/bot/main.go` wires: load → start scheduler with handler that calls `bot.OnCronFire(slug)` (stubbed in Phase 3; real handler in Phase 4).
- New `internal/bot.Bot` struct holds `*tgbotapi.BotAPI`, config, questionnaire map (slug→*Questionnaire), `*session.Manager`, and `*handler.QuestionFlow`. `bot.Run(ctx)` blocks polling.

</code_context>

<specifics>
## Specific Ideas

- Use `tgbotapi.NewMessage(chatID, text)` with `msg.ParseMode = tgbotapi.ModeMarkdown` for italic example rendering.
- `/start` is also accepted and returns the help message (BotFather convention).
- Free-text fallback message is one line, plain text.
- Session startup-restore log lines for visibility: `Restored session: <slug> (q=<idx>/<n>)` and `Finalised orphan session: <slug>` when applicable.

</specifics>

<deferred>
## Deferred Ideas

- Multi-active-session picker — Phase 4.
- Past-due skip algorithm — Phase 4.
- /pull / /status / /list bodies — Phase 4.
- Markdown escaping of user-provided example/question text — PRD does not require escaping; treat content as authored by the user.

</deferred>
