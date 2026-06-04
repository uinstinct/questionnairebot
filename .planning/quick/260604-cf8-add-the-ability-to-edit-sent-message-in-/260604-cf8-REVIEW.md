---
task: 260604-cf8
type: code-review
depth: quick
reviewer: orchestrator (hands-on)
date: 2026-06-04
verdict: clean
counts:
  critical: 0
  high: 0
  medium: 0
  low: 0
  info: 1
---

# Code Review — 260604-cf8 (edit sent message → answers.yaml)

Review of the diff `HEAD~3..HEAD` (commits 71ff710, 5d8f46c, 4e7dd0e), 10 files.
Performed by the orchestrator by reading each changed file in full and running
`go build ./...`, `go vet -tags integration ./...`, and the storage/session/handler
test suites (all green, uncached). The independently-spawned reviewer run was
aborted mid-flight; this is the orchestrator's own review.

## Verdict: CLEAN — no blocking findings

### 1. Match + legacy-only fallback gate (correctness) — PASS
- `storage.UpdateAnswerByMessageID` rejects `messageID == 0` up front
  (storage.go:74), so a legacy id-less answer can never be matched by a zero id.
- Match scans the first `MessageID == messageID` across all entries/answers; the
  fallback `UpdateLastAnswer` rewrites the newest entry's last answer **only when
  its `MessageID == 0`** (storage.go:104; session manager.go:114). A candidate
  carrying a real id returns `(false, nil)` — an unmatched edit cannot clobber a
  genuine answer. Confirmed by `TestUpdateLastAnswerRealIDGated`,
  `TestHandleEditedAnswerSafetyGate`, `TestDispatcherEditSafetyGateReply`.
- "Last answer" index is `len(answers)-1` with explicit empty-guard — no off-by-one.

### 2. Atomic write (storage.updateEntries) — PASS
- Missing file → `(false, nil)` via `errors.Is(err, os.ErrNotExist)` (storage.go:121).
- Writes only when `mutate` reports a change (no needless rewrites).
- Whole-document re-marshal + temp file + `f.Sync()` + `Close` + `os.Rename`, with
  `os.Remove(tmp)` on **every** error path (storage.go:142-165). Reuses prepend's
  ceremony; no byte-concatenation. Partial-write safe (rename is atomic on POSIX).

### 3. Concurrency — PASS
- Session edits go through `Manager.UpdateAnswerByMessageID` / `UpdateLastAnswer`,
  both taking `m.mu` and mutating the stored `*Session` directly (manager.go:86,107)
  — never a `Get()` clone. answers.yaml writes stay on the single synchronous
  `bot.Run` dispatch loop; cron only reads answers.yaml (`LastEntry`). No new race.

### 4. YAML round-trip — PASS
- `message_id,omitempty` on an `int` elides `0`; legacy files stay free of
  `message_id: 0` (asserted by the storage round-trip test). Full re-marshal of
  `[]Entry` preserves count/order; tests assert decoded values, not raw bytes.

### 5. Edit routing — PASS
- `update.EditedMessage` is handled **before** the `update.Message == nil` guard
  (dispatcher.go:50-53). `handleEditedMessage` early-returns on `msg.Text == ""`
  (caption/media) and `msg.IsCommand()` (dispatcher.go:67). `bot.go` polling config
  untouched (empty diff vs HEAD~3).

### 6. Test quality — PASS
- 15 new/extended cases assert real behavior: in-place text change with
  ordering/siblings intact, no-match leaves file unchanged, missing-file no-op,
  legacy fallback fires, safety-gate blocks + "couldn't match" reply, caption/command
  edits ignored, message_id round-trips to both session.yaml and answers.yaml.
  None would pass with the feature removed.

## Info-level note (non-blocking)
- I-01 `flow.go:newestCompletedSlug` skips an entry whose timestamp fails RFC3339
  parsing rather than erroring. This only affects the rare pre-feature fallback path
  and the bot's own writes are always RFC3339; documented in the function comment.
  Acceptable; no change required.
