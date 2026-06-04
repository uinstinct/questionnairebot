# Quick Task 260604-cf8: add the ability to edit sent message in telegram — Context

**Gathered:** 2026-06-04
**Status:** Ready for planning

<domain>
## Task Boundary

Add the ability to edit a previously-sent Telegram message (the user's answer)
and have that edit reflected back into the corresponding `answers.yaml` entry on
disk.

In Telegram, a user can edit a message they previously sent. When that happens,
Telegram delivers an `edited_message` update (not a normal `message`). Bots
cannot edit a user's message — the feature is: detect the user's edit, find the
answer it corresponds to, and update the persisted answer text.

**Note — supersedes a prior non-goal:** PROJECT.md lists "Editing or deleting
past answers via Telegram — answers are append-prepend only" as Out of Scope for
v1.0. This quick task explicitly reverses that decision for answer *edits*
(not deletes). The planner should treat this reversal as intentional and
user-approved.
</domain>

<decisions>
## Implementation Decisions

### Scope of editability
- **Any past answer is editable** — not just the current session, not just the
  most recent. An edit is matched to the specific answer by the Telegram
  message ID of the user's original answer message.
- **Backwards-compatibility fallback (precise, resolves RESEARCH Open Question #1):**
  when the edited message's ID matches no stored answer, fall back to editing the
  **most recent answer ONLY IF that candidate answer has no stored `message_id`**
  (i.e. `message_id == 0` — legacy data recorded before this feature). This is
  exactly the "if there is no message id, edit the most recent message" case.
  - "Most recent answer" = the last recorded answer of the single active session
    if one exists; otherwise the last `AnswerPair` of the newest entry
    (`entries[0]`) of the questionnaire whose newest entry is most recent (by
    `completed_at`/`scheduled_for`; trivially the only one when a single
    questionnaire exists).
  - **Safety gate:** if no ID matches AND the most-recent candidate answer
    already HAS a `message_id` (so this edit simply isn't one of our recorded
    answers — e.g. the user edited an unrelated old message), DO NOT rewrite.
    Reply that the edit couldn't be matched. This prevents an unrelated edit
    from clobbering a real answer (RESEARCH Pitfall #3) while still honoring the
    user's explicit legacy-data fallback instruction.

### answers.yaml update strategy
- **In-place modification.** Read the target `answers.yaml`, locate the matching
  entry/answer, modify the answer text, and rewrite the file atomically
  (temp-file + rename, consistent with the existing `prepend` helper). Do NOT
  prepend a new "corrected" entry — the existing entry is updated in place.
- The same in-place principle applies to an active `session.yaml` if the edited
  answer belongs to an in-progress session (the corrected value then flows to
  `answers.yaml` at finalize time).

### Edit detection
- **Dedicated handler in QuestionFlow.** Add a new method (e.g.
  `HandleEditedAnswer`) to `internal/handler.QuestionFlow`, separate from
  `HandleAnswer`. The dispatcher routes `update.EditedMessage` to this new
  method. `HandleAnswer` (new-answer path) stays unchanged in behavior.

### Message ID tracking (Claude's Discretion — forced by "any past answer" scope)
- To match an edit to a specific past answer, the **user's answer message ID**
  must be stored alongside each recorded answer. Add an optional
  `message_id` field (`yaml:"message_id,omitempty"`) to `storage.AnswerPair`
  (which is re-exported as `session.AnswerPair`), so it persists in both
  `session.yaml` and `answers.yaml`.
- Capture the message ID from `update.Message.MessageID` in the dispatcher's
  free-text path and thread it through `HandleAnswer` →
  `session.Manager.RecordAnswer` so it is persisted with the answer.
- Editing flow: on `edited_message`, take `EditedMessage.MessageID` + new text,
  search active sessions and all questionnaires' `answers.yaml` for an answer
  with that `message_id`; update in place. If none match (old data / no stored
  id), update the most recent answer (the fallback above).
- `omitempty` keeps old YAML files valid and round-trips cleanly; entries
  written before this feature simply have no `message_id`.
</decisions>

<specifics>
## Specific Ideas

- Existing `Sender` interface (`internal/bot.Sender`) returns only `error` from
  `Send`/`SendMarkdown`/`SendPicker` — the bot's own outgoing message IDs are
  discarded. This is fine: the message we track is the **user's answer message**,
  whose ID arrives on the inbound `update.Message`/`update.EditedMessage`, not
  the bot's outbound question. No change to the outbound Send signatures is
  required for the core feature.
- Dispatcher currently early-returns when `update.Message == nil`
  (`internal/handler/dispatcher.go`), so `edited_message` updates are silently
  dropped today. The fix routes `update.EditedMessage` before that guard.
- Data model touch point: `internal/storage/types.go` `AnswerPair`. Adding the
  field there propagates to `session` via the existing type alias.
- Storage helper additions live in `internal/storage` (alongside
  `PrependCompleted`/`PrependSkipped`/`LastEntry`), reusing the atomic write
  pattern from `prepend`.
</specifics>

<canonical_refs>
## Canonical References

- PRD: `.planning/prds/original-prd.md` (authoritative schemas for
  `answers.yaml` / `session.yaml`).
- Test convention: the repo uses table/unit tests with a `recordingSender`
  implementing `bot.Sender` (see `internal/handler/flow_test.go`,
  `internal/storage/storage_test.go`, `internal/session/manager_test.go`),
  plus integration tests. Despite PROJECT.md's "no unit tests" line, the
  established and expected pattern for verifying this change is a table test
  with the recording sender + a temp-dir `answers.yaml`/`session.yaml`. New
  tests MUST follow these existing patterns (no new mock frameworks).
</canonical_refs>
