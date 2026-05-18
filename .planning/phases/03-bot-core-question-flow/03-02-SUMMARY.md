---
plan_id: "03-02"
status: complete
date: 2026-05-18
---

# Plan 03-02 Summary — Question/Answer state machine

- `internal/handler/flow.go` — `QuestionFlow` decoupled from telegram-bot-api via the `Sender` interface. `New(...)` builds slug→Questionnaire map and defaults `Now` to `time.Now`. Methods: `StartQuestionnaire`, `SendQuestion`, `HandleAnswer`, `FinalizeIfDone`, internal `finalize`. Italic example rendered as `_Example: <text>_` via `SendMarkdown`. Plain questions use `Send`.
- `internal/handler/flow_test.go` — `TestQuestionFlowFullCycle` exercises start → 3 answers → completion. Asserts:
  - Q1 via `Send`, Q2 via `SendMarkdown` containing `_Example: Ex2_`, Q3 via `Send`.
  - Completion message `✅ Daily complete! Answers saved.`.
  - `answers.yaml` contains exactly one completed entry with answers in order.
  - `session.yaml` is deleted.
- `TestFinalizeIfDoneOrphan` covers the resume-and-finalise path (US-008 AC-3): a session at index==len gets finalised and the orphan session.yaml removed.

## Requirements Closed
BOT-03, BOT-04, BOT-05, BOT-06, BOT-07.

## Acceptance Evidence
- `go test ./internal/handler/...` and `go test -race ./internal/...` clean.
