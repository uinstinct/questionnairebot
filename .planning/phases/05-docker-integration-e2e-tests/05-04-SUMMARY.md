---
plan_id: "05-04"
status: complete
date: 2026-05-18
---

# Plan 05-04 Summary — E2E User-Action Mirroring to Telegram

Goal: make every user-side message sent in-process during an E2E run also appear in the real Telegram chat, so an observer watching the chat sees a readable two-way conversation instead of a one-sided stream of bot replies.

- `internal/e2e/helpers_test.go`:
  - New `(*e2eSender).logUserAction(text string)` (helpers_test.go:78) sends `"👤 " + text` via the real Telegram client. The send is intentionally not recorded into the `out` channel so `probeClient.waitForMessage` does not consume it as a bot reply.
  - `(*botRig).inject` (helpers_test.go:147) now calls `r.sender.logUserAction(text)` as its first line, before constructing the synthetic `tgbotapi.Update` and invoking the dispatcher. All entry points that produce user-side traffic (`probeClient.send`, `probeClient.sendCallback`) already delegate to `inject`, so a single hook covers every test path.
  - `newProbeClient` simplified to `(t *testing.T, rig *botRig) *probeClient` (helpers_test.go:175) — the probe no longer needs its own Telegram client, since the mirror is emitted by the existing `e2eSender`. This removes a redundant `tgbotapi.NewBotAPI` call per test.
- `internal/e2e/happy_path_test.go`: `newProbeClient(t, rig)` call site updated to match the new signature.
- `internal/e2e/pull_picker_test.go`: same call-site update.

## Requirements Closed
None new — this plan is a test ergonomics refinement on top of TEST-03/TEST-04 (already closed by 05-03). All phase-5 requirements (DOCK-01..07, TEST-01..09) remain covered.

## Acceptance Evidence
- `go build ./...` clean.
- `go test ./internal/e2e/... -tags integration` exits 0 (E2E tests skip cleanly when `TEST_TELEGRAM_*` env vars are absent).
- Expected on-chat transcript per the plan (happy path):

  ```
  👤 (cron fires — no user message)
  Bot: How was today?
  👤 fine
  Bot: Anything else?
  👤 nope
  Bot: ✅ Daily complete!
  ```

## Human Verification
Run the E2E suite once with real credentials and watch the chat:

```
TEST_TELEGRAM_BOT_TOKEN=… TEST_TELEGRAM_CHAT_ID=… \
  go test ./internal/e2e/... -tags integration -v
```

Confirm `👤 …` mirror lines appear interleaved with the bot replies in the configured chat.

## Out of Scope
- Sending the mirror as a real Telegram user account (MTProto) — same chat is reused, mirror is sent by the bot identity with a `👤` prefix.
- Recording mirror messages into `probeClient.log` / `e2eSender.out`.
