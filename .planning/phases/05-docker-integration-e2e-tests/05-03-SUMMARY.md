---
plan_id: "05-03"
status: complete
date: 2026-05-18
---

# Plan 05-03 Summary — E2E tests against real Telegram bot + README "Running Tests"

- `internal/e2e/helpers_test.go` (`//go:build integration`):
  - `requireTestEnv(t)` reads `TEST_TELEGRAM_BOT_TOKEN` / `TEST_TELEGRAM_CHAT_ID`; `t.Skip(...)` when either is absent.
  - `newBotUnderTest(t, dataDir, token, chatID)` boots the same component graph as `cmd/bot/main.go` (loader → session.Manager → handler.QuestionFlow → handler.Dispatcher → bot.Bot → commands.CronBus + Pull/Status/List adapter), starts `bus.Run` and `b.Run` goroutines, and returns a teardown func that cancels the context and waits up to 5s for goroutines to drain.
  - `probeClient` is a second `tgbotapi` client used as the "test user side" — `waitForMessage(t, timeout, predicate)` polls `getUpdates` until a bot message matches, `send` issues free-text replies. `sendCallback` notes a known limitation (bot accounts cannot tap inline buttons via callback_query).
- `internal/e2e/happy_path_test.go` (`TestE2EHappyPath`) — TEST-03. Writes a fresh tempdir questionnaire with 2 questions, fires the cron via `bus.Fire`, walks Q1 → answer → Q2 → answer → completion, then asserts `answers.yaml` first entry is `status: completed` with both answers in order and a `completed_at` timestamp.
- `internal/e2e/pull_picker_test.go` (`TestE2EPullPickerWithTwoPending`) — TEST-04. Two questionnaires; first `/pull` shows a picker with both labels; starts qa via the bus (bot account cannot tap an inline-keyboard button); completes qa; second `/pull` shows a picker containing only qb. Picker structure is asserted against the inline-keyboard markup labels.
- `README.md` — created (was missing). Includes Quick Start, environment variables table, command list, and the required `## Running Tests` section that names both `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID`, shows the `go test ./... -tags integration` invocation, and explains the skip-on-missing-env behaviour.

## Requirements Closed
TEST-01 (env-sourced credentials), TEST-02 (README documents env vars), TEST-03 (happy-path E2E), TEST-04 (dual-pending picker E2E), TEST-09 (`-tags integration` runs all of them).

## Acceptance Evidence
- `go test ./... -tags integration` exits 0 with E2E tests reporting SKIP (no live credentials in this sandbox).
- `grep -c "## Running Tests" README.md` returns 1.
- The harness has been wired against the same `cmd/bot/main.go` structure so when credentials are supplied later, the tests run end-to-end without code changes.

## Human Verification
- `TestE2EHappyPath` and `TestE2EPullPickerWithTwoPending` skip without credentials — operator must run them with a real `TEST_TELEGRAM_BOT_TOKEN` / `TEST_TELEGRAM_CHAT_ID` once to confirm green.
