# Quick Task 260604-cf8: Edit a sent Telegram message → reflect into answers.yaml — Research

**Researched:** 2026-06-04
**Domain:** Telegram long-polling update handling (`go-telegram-bot-api/v5`) + on-disk YAML mutation
**Confidence:** HIGH (library facts verified against vendored source `v5@v5.5.1`; Telegram delivery semantics cited from official Bot API docs)

---

## User Constraints (from CONTEXT.md — locked)

- **Scope:** ANY past answer is editable, matched by the user's answer message ID. Fallback to editing the **most recent answer** when no stored `message_id` matches (pre-feature data).
- **answers.yaml update is IN-PLACE** (read → mutate → atomic temp+rename rewrite), NOT a prepend of a corrected entry. Same in-place rule for an active `session.yaml`.
- **Detection:** dedicated `HandleEditedAnswer` method on `internal/handler.QuestionFlow`; dispatcher routes `update.EditedMessage` to it. `HandleAnswer` (new-answer path) unchanged in behaviour.
- **Storage:** new `message_id` field (`yaml:"message_id,omitempty"`) on `storage.AnswerPair` (aliased as `session.AnswerPair`), persisted in both `session.yaml` and `answers.yaml`. Captured from `update.Message.MessageID`, threaded `HandleAnswer → session.Manager.RecordAnswer`.
- Tests: `recordingSender` (`bot.Sender`) + temp-dir YAML table tests (`internal/handler/flow_test.go`, `internal/storage/storage_test.go`, `internal/session/manager_test.go`). No new mock frameworks. This task **reverses** PROJECT.md's "no editing past answers" non-goal for edits only (not deletes) — intentional, user-approved.

---

## Summary

The feature is fully achievable with the existing stack and **needs no transport/config change**. `tgbotapi.Update` already exposes `EditedMessage *Message`, and the bot's current `tgbotapi.NewUpdate(0)` polling config receives `edited_message` updates by default (Telegram only excludes `chat_member`/reactions from the default set). The dispatcher silently drops edits today purely because of the `if update.Message == nil { return }` guard — the fix is one new branch *before* that guard.

A user editing their own message keeps the same `MessageID`, so matching a stored answer by ID is reliable; the `message_id`/fallback design from CONTEXT.md is sound. The in-place rewrite reuses the storage package's existing atomic temp+rename pattern, but unmarshals the whole `[]Entry`, mutates, and re-marshals (cleaner than `prepend`'s byte-concatenation trick).

The one subtlety worth getting right is **write serialisation**, and the good news is it falls out of the existing architecture: all inbound updates (new answers *and* edits) are processed by the single synchronous `bot.Run` loop, so answers.yaml has exactly one writer goroutine. Session-file edits must still go through a new `session.Manager` method to stay under the existing mutex.

**Primary recommendation:** Add `MessageID int` to `AnswerPair`; thread the ID through `HandleAnswer → RecordAnswer`; add `QuestionFlow.HandleEditedAnswer(messageID int, newText string)` + a `storage.UpdateAnswerByMessageID(...)` helper + a `session.Manager.UpdateAnswerByMessageID(...)` method; route `update.EditedMessage` in the dispatcher *before* the nil-Message guard, ignoring non-text and command edits. Do **not** touch the polling config.

---

## Library Findings (`go-telegram-bot-api/v5@v5.5.1`)

All line references are to the vendored module at `$(go env GOMODCACHE)/github.com/go-telegram-bot-api/telegram-bot-api/v5@v5.5.1`.

### EditedMessage surfacing — VERIFIED (source)

`types.go:34-117` `type Update struct`:

```go
Message       *Message `json:"message,omitempty"`
EditedMessage *Message `json:"edited_message,omitempty"`   // types.go:53
```

The exact field is **`Update.EditedMessage *Message`** (pointer; nil unless the update is an edit).

`(*Update).FromChat()` (`types.go:151-166`) already handles edits — `case u.EditedMessage != nil: return u.EditedMessage.Chat` (types.go:155-156). **Consequence:** the existing `bot.IsAuthorised` (`internal/bot/auth.go`, which calls `update.FromChat()`) authorises `edited_message` updates from the configured chat with **no change required**.

### Does long-polling deliver edited_message by default? — YES, no config change needed

- Current setup (`internal/bot/bot.go:53`): `u := tgbotapi.NewUpdate(0)` then `b.API.GetUpdatesChan(u)`.
- `NewUpdate(offset int) UpdateConfig` (`helpers.go:292-298`) sets only `Offset/Limit/Timeout`; **`AllowedUpdates` is left nil**.
- `UpdateConfig.params()` (`configs.go:1146-1155`) calls `params.AddInterface("allowed_updates", config.AllowedUpdates)`.
- `AddInterface` (`params.go:48-61`): a typed-nil `[]string` is **not** caught by the `value == nil`/`Ptr&&IsNil` short-circuit (its `reflect.Kind` is `Slice`, not `Ptr`), so it `json.Marshal`s the nil slice to the literal `null` and sends `allowed_updates=null`.
- Telegram treats `null`/unspecified `allowed_updates` as "use previous setting", and with no prior setting (long-polling, no webhook) the **default set applies, which includes `edited_message`** (default excludes only `chat_member`, `message_reaction`, `message_reaction_count`). [CITED: core.telegram.org/bots/api#getupdates]

The library even defines the constant `UpdateTypeEditedMessage = "edited_message"` (`configs.go:55`) for callers who want an explicit allow-list.

**Verdict:** `bot.New`/`Run` as written **already receives `edited_message` updates**. Leave `NewUpdate(0)` as-is.
**Footgun:** do NOT "fix" this by setting `u.AllowedUpdates = []string{tgbotapi.UpdateTypeEditedMessage}` — an explicit list is exhaustive, so you would silently stop receiving `message` and `callback_query`. If made explicit at all, it MUST list all three: `UpdateTypeMessage`, `UpdateTypeEditedMessage`, `UpdateTypeCallbackQuery`. Recommendation: don't make it explicit.

### Message ID semantics — VERIFIED (source) / CITED (equality)

`types.go:357-421` `type Message struct`:

```go
MessageID int    `json:"message_id"`        // types.go:359  — int, always > 0
Date      int    `json:"date"`              // types.go:372  — Unix time sent
EditDate  int    `json:"edit_date,omitempty"` // types.go:421 — Unix time of last edit
Text      string `json:"text,omitempty"`    // types.go:437  — text-message body
Caption   string `json:"caption,omitempty"` // types.go:479  — media caption (NOT Text)
```

- `MessageID` is `int` → the new `AnswerPair.MessageID` field should also be `int`. Telegram message IDs start positive and increase, so `0` is never a valid ID — `omitempty` is safe and unambiguous.
- Editing a message does **not** change its `message_id`; Telegram redelivers the *same* `Message` (same `MessageID`) inside `Update.EditedMessage`, with `EditDate` set. Matching a stored answer by `MessageID == EditedMessage.MessageID` is therefore reliable. [CITED: core.telegram.org/bots/api — edited_message / Message.edit_date semantics]
- `EditDate` is available if ordering is ever wanted, but it is **not needed** for this feature (match is by ID, fallback is "most recent").

### Update processing is single-threaded — VERIFIED (source)

`internal/bot/bot.go:58-71`: the `Run` loop calls `b.dispatcher.Handle(ctx, b, update)` **synchronously** (not in a goroutine), one update at a time. This is the linchpin of the concurrency analysis below.

---

## Storage / YAML Findings (`gopkg.in/yaml.v3`)

Schema (verified against `examples/daily-standup/answers.yaml` and `internal/storage/types.go`): `answers.yaml` is a top-level YAML **sequence** of `Entry`; each `Entry` has `answers: []AnswerPair`. Newest entry is index `0` (prepend semantics). `session.yaml` is a single `Session` mapping with `answers: []AnswerPair`.

### New field round-trips cleanly — confident

Add to `internal/storage/types.go`:

```go
type AnswerPair struct {
	Question  string `yaml:"question"`
	Answer    string `yaml:"answer"`
	MessageID int    `yaml:"message_id,omitempty"` // NEW
}
```

- `session.AnswerPair` is a type alias (`internal/session/types.go:7`) → the field appears in both files automatically.
- **Old files without `message_id`** unmarshal to `MessageID == 0` (Go zero value); yaml.v3 ignores absent keys. No migration of existing files required.
- `omitempty` on `int` omits `0`, so pre-feature answers re-marshal byte-identically in shape (no spurious `message_id: 0`). New answers emit `message_id: <N>`.

### In-place rewrite pattern — reuse `prepend`'s atomic write, NOT its concatenation

The existing `prepend` (`internal/storage/storage.go:72-119`) does: marshal new `[]Entry{e}` → write to `path+".tmp"` → append raw existing bytes → `f.Sync()` → `f.Close()` → `os.Rename(tmp, path)`. The byte-concatenation trick works only because two top-level sequences concatenate.

For the **edit** helper, do a full-document rewrite instead (simpler, no concatenation edge cases):

1. `os.ReadFile(answersPath(dataDir, slug))` → if `os.ErrNotExist`, return `(false, nil)`.
2. `yaml.Unmarshal(raw, &entries)` into `[]Entry`.
3. Scan `entries[i].Answers[j]` for `MessageID == target`; on match set `.Answer = newText`.
4. `yaml.Marshal(entries)` (single document, no `---` separators) → write to `path+".tmp"` → `Sync` → `Close` → `os.Rename`. Reuse the exact temp+rename ceremony from `prepend` (mkdir, error cleanup with `os.Remove(tmp)`).

**Gotchas:**
- yaml.v3 `Marshal` does **not** preserve comments or original key spacing. The repo's YAML is machine-generated and comment-free, so a full re-marshal is lossless in practice — but it will normalise whitespace (e.g. the aligned `answer:   ` columns in the hand-written example become single-space). This is cosmetic and acceptable; tests should assert on **unmarshalled values**, not byte equality.
- Default yaml.v3 indent is 4 spaces — matches existing machine-written files, since they too came from `yaml.Marshal`.
- Marshalling the whole `[]Entry` preserves field order from the struct definition (`status`, `scheduled_for`, …), and `omitempty` continues to drop `completed_at`/`skipped_at`/empty `answers`/`message_id: 0`.

---

## Integration Points (concrete signatures)

Affected files and the exact callsites that change (verified by repo grep):

### 1. `internal/storage/types.go` — data model
Add `MessageID int \`yaml:"message_id,omitempty"\`` to `AnswerPair` (propagates to `session.AnswerPair` alias).

### 2. `internal/storage/storage.go` — new edit helper(s)
Lives alongside `PrependCompleted`/`PrependSkipped`/`LastEntry`, reusing the atomic write.
```go
// UpdateAnswerByMessageID rewrites data/<slug>/answers.yaml in place, setting the
// answer text of the AnswerPair whose MessageID == messageID. Returns matched=false
// (no error) when the file is absent or no answer has that id.
func UpdateAnswerByMessageID(dataDir, slug string, messageID int, newText string) (matched bool, err error)
```
Fallback helper for the "most recent answer" path (pre-feature data):
```go
// UpdateLastAnswer rewrites the last AnswerPair of the newest entry (entries[0]).
func UpdateLastAnswer(dataDir, slug string, newText string) (matched bool, err error)
```
(Factor the read→rewrite-`[]Entry` core into one unexported helper to avoid duplicating the temp+rename block.)

### 3. `internal/session/manager.go` — thread ID in; edit active session under mutex
- **Change** `RecordAnswer` signature (append `messageID int`):
  - `func (m *Manager) RecordAnswer(slug, question, answer string) error`
  - → `func (m *Manager) RecordAnswer(slug, question, answer string, messageID int) error`
  - body: `s.Answers = append(s.Answers, AnswerPair{Question: question, Answer: answer, MessageID: messageID})`
- **Add** edit-in-place method (must run under `m.mu` + `saveLocked`, because cron's `Start` mutates sessions on another goroutine):
  ```go
  func (m *Manager) UpdateAnswerByMessageID(slug string, messageID int, newText string) (bool, error)
  // and/or, for the fallback:
  func (m *Manager) UpdateLastAnswer(slug, newText string) (bool, error)
  ```
  Do NOT mutate the clone returned by `Get()` — it is a copy (`cloneSession`), so writes there are lost.

### 4. `internal/handler/flow.go` — thread ID in; new edit entry point
- **Change** `HandleAnswer` signature (append `messageID int`):
  - `func (f *QuestionFlow) HandleAnswer(slug, text string) error`
  - → `func (f *QuestionFlow) HandleAnswer(slug, text string, messageID int) error`
  - line 89 call becomes `f.Sessions.RecordAnswer(slug, current.Question, text, messageID)`.
- **Add** `func (f *QuestionFlow) HandleEditedAnswer(messageID int, newText string) error`:
  1. For each `slug` in `f.Questionnaires` with an active session (`f.Sessions.Get(slug) != nil`): try `f.Sessions.UpdateAnswerByMessageID(slug, messageID, newText)`; on match → reply success, return.
  2. Else for each `slug`: try `storage.UpdateAnswerByMessageID(f.DataDir, slug, messageID, newText)`; on match → reply success, return.
  3. Else (fallback) → update the most recent answer (see Open Question) → reply success.
  4. Reply via `f.Sender.Send(...)` on success/failure.

### 5. `internal/handler/dispatcher.go` — route the edit before the nil-Message guard
- In `Handle` (currently lines 45-58), add a branch **before** `if update.Message == nil { return }`:
  ```go
  if update.EditedMessage != nil {
      d.handleEditedMessage(sender, update.EditedMessage)
      return
  }
  ```
- **Change** the free-text path to carry the ID:
  - line 57: `d.handleFreeText(sender, update.Message.Text)` → `d.handleFreeText(sender, update.Message.Text, update.Message.MessageID)`
  - `func (d *Dispatcher) handleFreeText(sender bot.Sender, text string)` → `(..., text string, messageID int)`; line 112 call becomes `d.Flow.HandleAnswer(active[0], text, messageID)`.
- **Add** `handleEditedMessage(sender bot.Sender, msg *tgbotapi.Message)`:
  - Ignore non-text edits: `if msg.Text == "" { return }` (caption/media edits carry `Caption`, not `Text`).
  - Ignore edited commands: `if msg.IsCommand() { return }`.
  - Else `d.Flow.HandleEditedAnswer(msg.MessageID, msg.Text)`.

### Callsites to update for the signature changes (grep-verified)
- `RecordAnswer` (now `+messageID int`): `internal/handler/flow.go:89` (prod); tests `internal/handler/flow_test.go:144`, `internal/session/manager_test.go:22,72`, `internal/commands/status_integration_test.go:57`.
- `HandleAnswer` (now `+messageID int`): `internal/handler/dispatcher.go:112` (prod); tests `internal/handler/flow_test.go:84,91,98`.
- `handleFreeText`: single caller `internal/handler/dispatcher.go:57`.
- E2E tests (`internal/e2e/*`) drive the real bot via Telegram and do **not** call these methods directly — unaffected by signatures, but a new edited-message E2E is optional follow-on.

---

## Pitfalls & Gotchas

1. **Nil-Message guard drops edits today.** `dispatcher.Handle` returns early on `update.Message == nil`, and an `edited_message` update has `Message == nil`. The new branch MUST come *before* that guard (and before `IsCommand` routing). [VERIFIED: dispatcher.go:50-57]

2. **Caption edits vs text edits.** Editing a photo/file caption populates `EditedMessage.Caption`, leaving `EditedMessage.Text == ""`. Answers are always text, so guard with `if msg.Text == "" { return }` to ignore media/caption edits cleanly. [VERIFIED: types.go:437,479]

3. **Edits to non-answer messages.** A user can edit a previously sent *command* (e.g. `/pull`) or any unrelated message. `msg.IsCommand()` filters edited commands; the ID-search-then-fallback naturally no-ops for messages that were never recorded as answers — **except** the fallback, which would otherwise wrongly rewrite the most-recent answer for *any* unmatched edit. Mitigation: only invoke the fallback when there is genuinely no `message_id` match anywhere AND the edited text is a plausible answer; consider replying "couldn't match that edit to an answer" instead of blindly rewriting. (See Open Question.)

4. **Concurrency — and why it's already safe for answers.yaml.** All inbound updates run on the **single synchronous `bot.Run` loop** (`bot.go:69`, no goroutine per update), so new-answer finalize writes (`HandleAnswer → finalize → storage.PrependCompleted`) and edit writes (`HandleEditedAnswer → storage.UpdateAnswerByMessageID`) **never overlap** — same goroutine, sequential. The only other goroutine is `CronBus.Run` (`go bus.Run`), which **never writes answers.yaml** (its `flush` only *reads* via `storage.LastEntry` and starts sessions). So `answers.yaml` has exactly one writer → **no new mutex needed**, provided the edit write stays in the dispatcher/handler path. Reads from cron are safe against the writer because `os.Rename` is atomic on POSIX (reader sees full-old or full-new, never torn) — the edit helper reuses that same temp+rename, preserving the guarantee.

5. **session.yaml IS cross-goroutine.** `session.yaml` is written by the polling goroutine (`RecordAnswer`) *and* the cron goroutine (`Start`), serialised today by `session.Manager.mu`. Therefore an edit to an **active** session MUST go through a new `Manager` method that takes `m.mu` and calls `saveLocked` — never by mutating a `Get()` clone (it's a copy; `cloneSession` at manager.go:154).

6. **`omitempty` correctness for `int`.** Telegram `MessageID` is always > 0, so `0` (zero value of unmatched/old answers) is safely elided and never collides with a real ID. Don't use a pointer `*int` — unnecessary, and it complicates round-trip.

7. **Full re-marshal normalises formatting.** yaml.v3 drops comments and the example file's column alignment on rewrite. Harmless (files are machine-generated); just assert on decoded values in tests, not raw bytes.

8. **User feedback.** Bots cannot edit a user's message, only react. Reply with a short confirmation on success (e.g. "✏️ Updated your answer.") and a clear miss message on no-match, so the user knows the edit took effect. Keep it consistent with existing `f.Sender.Send` plain-text style.

---

## Recommended Approach

1. `storage.AnswerPair`: add `MessageID int \`yaml:"message_id,omitempty"\``.
2. Thread the ID inbound: `dispatcher.handleFreeText(..., messageID)` → `QuestionFlow.HandleAnswer(slug, text, messageID)` → `session.Manager.RecordAnswer(slug, q, a, messageID)`. Update the 4 test callsites + 2 prod callsites listed above.
3. Add `storage.UpdateAnswerByMessageID` (+ `UpdateLastAnswer` for fallback), sharing one unexported read→mutate→atomic-rewrite core.
4. Add `session.Manager.UpdateAnswerByMessageID` (and/or `UpdateLastAnswer`) under `m.mu`+`saveLocked`.
5. Add `QuestionFlow.HandleEditedAnswer(messageID, newText)`: search active sessions → all slugs' answers.yaml → fallback; reply on outcome.
6. Dispatcher: route `update.EditedMessage` before the nil-Message guard via `handleEditedMessage`, ignoring empty-`Text` (caption/media) and `IsCommand()` edits.
7. **Do not change** `bot.go`'s `NewUpdate(0)` polling config — `edited_message` already arrives, and `IsAuthorised`/`FromChat()` already handle it.

### Tests to add (existing patterns, no new frameworks)
- `storage_test.go`: write a multi-entry `answers.yaml` (mix of with/without `message_id`), call `UpdateAnswerByMessageID` for a match (verify in-place text change, ordering preserved, sibling answers untouched), a no-match (`matched==false`, file unchanged), and a missing-file case. Verify old-file (no `message_id`) unmarshals to `MessageID==0` and round-trips.
- `manager_test.go`: `RecordAnswer(...,messageID)` persists the id to `session.yaml`; `UpdateAnswerByMessageID` mutates the active session under lock.
- `flow_test.go` / `dispatcher_test.go`: drive `HandleEditedAnswer` (and a `tgbotapi.Update{EditedMessage: ...}` through `Dispatcher.Handle`) via `recordingSender` — assert the answer text changed, fallback fires for an unmatched id, and caption/command edits are ignored. Reuse `freeTextUpdate`-style helpers; add an `editedTextUpdate(text string, id int)` helper.

---

## Validation Architecture

`workflow.nyquist_validation: true` (`.planning/config.json`) → tests required.

| Property | Value |
|---|---|
| Framework | Go stdlib `testing` (+ `gopkg.in/yaml.v3` for assertions); E2E uses `testify/require` |
| Config file | none — `go test ./...` |
| Quick run | `go test ./internal/storage/ ./internal/session/ ./internal/handler/` |
| Full suite | `go test ./...` |

| Behaviour | Test type | Command |
|---|---|---|
| `UpdateAnswerByMessageID` match / no-match / missing-file / old-file round-trip | unit | `go test ./internal/storage/ -run UpdateAnswer` |
| `RecordAnswer` persists `message_id`; `Manager.UpdateAnswerByMessageID` under lock | unit | `go test ./internal/session/ -run Answer` |
| `HandleEditedAnswer` match + fallback; dispatcher routes `EditedMessage`; ignores caption/command edits | table | `go test ./internal/handler/ -run Edit` |

**Wave 0 gaps:** none — `recordingSender` + temp-dir YAML harness already exists in all three packages; add cases to existing `_test.go` files.

---

## Open Questions (RESOLVED)

> **RESOLVED in CONTEXT.md** (Decisions → Backwards-compatibility fallback): the fallback edits the
> most-recent answer ONLY IF that candidate has no stored `message_id` (`message_id == 0`, legacy
> data). If the most-recent candidate already has a real `message_id`, nothing is rewritten and the
> user is told the edit couldn't be matched — adopting the safer "no-change" reply from Pitfall 3
> while still honoring the user's explicit legacy-data fallback instruction.

1. **Definition of "most recent answer" for the fallback path.** `AnswerPair` has no timestamp, and an unmatched edit gives no slug. Recommended default (rare, pre-feature-data only): if exactly one active session exists → edit its last recorded answer (`Manager.UpdateLastAnswer`); otherwise edit `entries[0].Answers[len-1]` of whichever slug's `answers.yaml` has the newest `entries[0]` (compare `CompletedAt`/`ScheduledFor`). Keep it simple — this only triggers for answers recorded before this feature shipped. The planner should confirm whether a "couldn't match — no change made" reply is preferable to a blind most-recent rewrite (safer; see Pitfall 3). [ASSUMED — needs planner/user confirmation]

---

## Assumptions Log

| # | Claim | Section | Risk if wrong |
|---|---|---|---|
| A1 | Fallback "most recent answer" = newest entry's last `AnswerPair` (or single active session's last answer) | Open Questions / Recommended Approach | Wrong answer edited for legacy data; mitigated by it being a rare pre-feature path |
| A2 | A blind fallback rewrite on any unmatched edit is undesirable; prefer a no-match reply | Pitfall 3 | Editing an unrelated old message could overwrite a real answer |

All other claims are VERIFIED against `v5@v5.5.1` source or CITED from the official Telegram Bot API docs.

---

## Sources

**Primary (HIGH):**
- Vendored library source `github.com/go-telegram-bot-api/telegram-bot-api/v5@v5.5.1`: `types.go` (Update L34-117, FromChat L151-166, Message L357-479), `bot.go` (GetUpdates L404, GetUpdatesChan L431, Run-loop usage), `configs.go` (UpdateConfig L1135, UpdateTypeEditedMessage L55, params L1146-1155), `helpers.go` (NewUpdate L292), `params.go` (AddInterface L48).
- Repo source: `internal/bot/bot.go`, `internal/bot/auth.go`, `internal/handler/dispatcher.go`, `internal/handler/flow.go`, `internal/session/manager.go`, `internal/session/types.go`, `internal/storage/storage.go`, `internal/storage/types.go`, `cmd/bot/main.go`, `internal/commands/cron.go`, test files, `examples/daily-standup/answers.yaml`.

**Secondary (CITED):**
- Telegram Bot API — `getUpdates` `allowed_updates` default set; `Update.edited_message` / `Message.edit_date` / `message_id` semantics: core.telegram.org/bots/api#getupdates, core.telegram.org/bots/api#update, core.telegram.org/bots/api#message

## RESEARCH COMPLETE

**File:** `.planning/quick/260604-cf8-add-the-ability-to-edit-sent-message-in-/260604-cf8-RESEARCH.md`
