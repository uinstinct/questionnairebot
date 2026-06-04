---
phase: 260604-cf8-add-the-ability-to-edit-sent-message-in-
plan: 01
status: complete
subsystem: telegram
tags: [telegram, edit-message, yaml, storage, go]

# Dependency graph
requires: []
provides:
  - storage.AnswerPair.MessageID (yaml:message_id,omitempty) persisted in answers.yaml and session.yaml
  - storage.UpdateAnswerByMessageID + storage.UpdateLastAnswer over a shared atomic read->mutate->rewrite-[]Entry core
  - session.Manager.UpdateAnswerByMessageID + session.Manager.UpdateLastAnswer (legacy-gated, under m.mu+saveLocked)
  - message_id threaded through dispatcher.handleFreeText -> QuestionFlow.HandleAnswer -> session.Manager.RecordAnswer
  - QuestionFlow.HandleEditedAnswer (search active sessions -> answers.yaml -> legacy-gated fallback -> reply)
  - Dispatcher routing of update.EditedMessage before the nil-Message guard
affects: [telegram-dispatcher, answer-storage, session-persistence]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared unexported read->mutate(closure)->atomic temp+fsync+rename core for whole-document YAML rewrites"
    - "LOCKED legacy-only safety gate (message_id==0) lives inside both UpdateLastAnswer helpers"
    - "Edit-path session mutation goes through Manager methods under m.mu+saveLocked, never a Get() clone"

key-files:
  created: []
  modified:
    - internal/storage/types.go
    - internal/storage/storage.go
    - internal/storage/storage_test.go
    - internal/session/manager.go
    - internal/session/manager_test.go
    - internal/handler/flow.go
    - internal/handler/flow_test.go
    - internal/handler/dispatcher.go
    - internal/handler/dispatcher_test.go
    - internal/commands/status_integration_test.go

key-decisions:
  - "MessageID is a plain int (not *int): Telegram ids are always > 0, so 0 unambiguously means legacy/no-id and omitempty elides it."
  - "Edit reply copy uses existing emoji style (✏️ success / ❓ miss); tests assert on the stable substrings 'Updated' and 'match', not exact bytes."
  - "newestCompletedSlug compares CompletedAt (falling back to ScheduledFor) as RFC3339 and skips an unparseable timestamp rather than aborting the edit."

patterns-established:
  - "Whole-[]Entry re-marshal (not prepend's byte-concatenation) for in-place edits, reusing prepend's temp+fsync+rename ceremony with os.Remove cleanup on every error path."
  - "messageID==0 guard rejects matching before any disk read so a legacy id-less answer can never be matched by id."

requirements-completed: [EDIT-01]

# Metrics
duration: ~5min (commits 10:00:45Z -> 10:05:50Z)
completed: 2026-06-04
---

# Phase 260604-cf8 Plan 01: Edit a Sent Telegram Message Summary

**Editing a previously-sent Telegram answer is matched by the user's message_id and rewritten in place in answers.yaml / session.yaml, with a legacy-only fallback that never clobbers a real answer; bot.go polling config untouched.**

## Performance

- **Duration:** ~5 min (first task commit 2026-06-04T10:00:45Z → last 2026-06-04T10:05:50Z)
- **Started:** 2026-06-04T10:00:45Z
- **Completed:** 2026-06-04T10:05:50Z
- **Tasks:** 3
- **Files modified:** 10

## Accomplishments
- `storage.AnswerPair` now carries `MessageID int` (omitempty); every newly recorded answer persists its Telegram message id in both `session.yaml` and `answers.yaml`, and legacy files stay valid with no spurious `message_id: 0`.
- In-place edit helpers (`storage.UpdateAnswerByMessageID` / `UpdateLastAnswer`, plus the session-manager equivalents under `m.mu`+`saveLocked`) rewrite a single answer via a full-document atomic re-marshal, preserving entry ordering and sibling answers.
- `QuestionFlow.HandleEditedAnswer` + dispatcher routing of `update.EditedMessage` (before the nil-Message guard) reflect edits end-to-end, with the LOCKED legacy-only fallback and a "couldn't match" reply for unmatched edits whose most-recent candidate already has a real id.
- Caption/media edits (`Text==""`) and edited slash-commands (`IsCommand`) are ignored — no rewrite, no answer-edit reply.

## Task Commits

Each task was committed atomically (code only; planning docs left to the orchestrator):

1. **Task 1: Storage data model + in-place edit helpers** — `71ff710` (feat)
2. **Task 2: Thread message_id through the record path + session edit primitives** — `5d8f46c` (feat)
3. **Task 3: Edit detection + reflection (HandleEditedAnswer + dispatcher routing)** — `4e7dd0e` (feat)

_Note: per the assignment, each task is one atomic commit (no separate RED/GREEN split); tests and implementation land together in the task commit._

## Files Created/Modified
- `internal/storage/types.go` — added `MessageID int` (`yaml:"message_id,omitempty"`) to `AnswerPair`; propagates to `session.AnswerPair` via the existing alias.
- `internal/storage/storage.go` — added `UpdateAnswerByMessageID`, `UpdateLastAnswer`, and the shared unexported `updateEntries` core (read → mutate closure → conditional whole-document atomic rewrite reusing prepend's ceremony).
- `internal/storage/storage_test.go` — match / no-match / missing-file / messageID==0 guard / legacy-fallback / safety-gate / omitempty round-trip cases.
- `internal/session/manager.go` — `RecordAnswer` now takes `messageID`; added `UpdateAnswerByMessageID` and legacy-gated `UpdateLastAnswer`, both mutating the stored `*Session` under `m.mu`+`saveLocked`.
- `internal/session/manager_test.go` — message_id persistence after reload + the two manager edit methods (incl. legacy gate); updated existing `RecordAnswer` callsites.
- `internal/handler/flow.go` — `HandleAnswer` now threads `messageID`; added `HandleEditedAnswer` (search active → answers.yaml → legacy-gated fallback → reply) and `newestCompletedSlug`.
- `internal/handler/flow_test.go` — full-cycle test asserts per-answer ids land in answers.yaml; four `HandleEditedAnswer` cases (active-session match, answers.yaml match, legacy fallback, safety gate).
- `internal/handler/dispatcher.go` — route `update.EditedMessage` before the nil-Message guard; `handleFreeText` threads `update.Message.MessageID`; new `handleEditedMessage` ignores caption/media and command edits.
- `internal/handler/dispatcher_test.go` — `editedTextUpdate` / `editedCmdUpdate` helpers; routing-updates, caption-ignored, command-ignored, safety-gate-reply cases.
- `internal/commands/status_integration_test.go` — updated the `//go:build integration` `RecordAnswer` callsite (`..., 0`); verified via `go vet -tags integration ./...`.

## Decisions Made
- **`MessageID` as plain `int`, not `*int`** — Telegram ids are always > 0, so `0` unambiguously means legacy/no-id and `omitempty` elides it cleanly (per CONTEXT.md D-"Message ID tracking").
- **Reply copy** — `✏️ Updated your answer.` (success) and `❓ Couldn't match that edit to a saved answer — no changes made.` (miss), consistent with the existing `✅`/`❌` style; tests assert on disjoint stable substrings `Updated` / `match`.
- **`newestCompletedSlug` parse robustness** — an unparseable `CompletedAt`/`ScheduledFor` timestamp skips that candidate rather than failing the edit. Our own writes are always RFC3339, so this only guards against externally-corrupted files; it never aborts a legitimate edit. (Plan left parse-error behavior unspecified.)

## Deviations from Plan

None — plan executed exactly as written. No deviation rules (1–4) triggered: no bugs, missing critical functionality, blocking issues, or architectural changes were encountered. All helper additions live inside the existing `_test.go` files (no new test files), following the established `recordingSender` + `t.TempDir()` + `yaml.Unmarshal` pattern; `editedCmdUpdate` mirrors the existing `cmdUpdate` helper as the plan's "EditedMessage carrying a bot_command entity like `cmdUpdate` builds" implies.

## Issues Encountered
None.

## Verification

All four end-to-end gates passed.

### 1. `go build ./...`
```
build OK (exit 0)
```

### 2. `go vet -tags integration ./...`
```
vet OK (exit 0)
```
(Confirms every test file — including the `//go:build integration` `status_integration_test.go` — type-checks against the new signatures.)

### 3. `go test ./internal/storage/ ./internal/session/ ./internal/handler/`
```
ok  	github.com/aditya-mitra/questionnairebot/internal/storage	0.111s
ok  	github.com/aditya-mitra/questionnairebot/internal/session	0.074s
ok  	github.com/aditya-mitra/questionnairebot/internal/handler	0.035s
```
The eight new edit tests (run with `-run Edit -v`):
```
--- PASS: TestDispatcherEditRoutesAndUpdates (0.01s)
--- PASS: TestDispatcherEditCaptionIgnored (0.00s)
--- PASS: TestDispatcherEditCommandIgnored (0.00s)
--- PASS: TestDispatcherEditSafetyGateReply (0.00s)
--- PASS: TestHandleEditedAnswerActiveSessionMatch (0.00s)
--- PASS: TestHandleEditedAnswerCompletedMatch (0.00s)
--- PASS: TestHandleEditedAnswerLegacyFallback (0.00s)
--- PASS: TestHandleEditedAnswerSafetyGate (0.00s)
ok  	github.com/aditya-mitra/questionnairebot/internal/handler	0.021s
```

### 4. `internal/bot/bot.go` unmodified
```
$ git diff HEAD~3 -- internal/bot/bot.go
(empty)
$ git diff --quiet HEAD~3 -- internal/bot/bot.go && echo "bot.go UNMODIFIED"
bot.go UNMODIFIED
```
Polling config stays `NewUpdate(0)`.

## Known Stubs
None — no placeholders, mocks, or TODOs introduced; every helper is fully wired to disk and the dispatcher path.

## Next Phase Readiness
- Edit-by-message-id is complete and covered by unit/table tests across storage, session, and handler.
- No external-service or config changes required; `go.mod`/`go.sum` unchanged (no new deps).
- The optional edited-message E2E (driving the live bot) noted in RESEARCH remains a possible follow-on but is out of scope for this task.

---
*Phase: 260604-cf8-add-the-ability-to-edit-sent-message-in-*
*Completed: 2026-06-04*
