---
phase: quick-260529-qsv
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/commands/registry.go
  - internal/bot/bot.go
  - cmd/bot/main.go
  - internal/e2e/register_commands_test.go
autonomous: true
requirements: [QSV-AUTOREG]
must_haves:
  truths:
    - "On bot startup, the user-facing commands (pull, status, list, help) are registered with Telegram via setMyCommands"
    - "A transient setMyCommands failure at startup logs a warning and does not kill the process"
    - "getMyCommands against the test bot returns exactly pull, status, list, help (in registry order)"
  artifacts:
    - path: "internal/commands/registry.go"
      provides: "App-layer command catalog — Commands() []tgbotapi.BotCommand"
      contains: "func Commands"
    - path: "internal/bot/bot.go"
      provides: "Generic RegisterCommands transport method"
      contains: "func (b *Bot) RegisterCommands"
    - path: "internal/e2e/register_commands_test.go"
      provides: "E2E proof that setMyCommands round-trips against the real test bot"
      contains: "GetMyCommands"
  key_links:
    - from: "cmd/bot/main.go"
      to: "internal/bot RegisterCommands + internal/commands Commands"
      via: "b.RegisterCommands(commands.Commands()) with log-and-continue"
      pattern: "RegisterCommands\\(commands\\.Commands\\(\\)\\)"
---

<objective>
Auto-register the bot's user-facing slash commands (pull, status, list, help) with
Telegram via setMyCommands on startup, so they appear in the Telegram client's "/"
autocomplete menu.

Today the dispatcher parses these commands but the bot never calls setMyCommands —
getMyCommands returns []. setMyCommands is idempotent (overwrites), so no
"register once" guard is needed.

Purpose: Commands become discoverable in the Telegram client menu without manual
BotFather configuration.
Output: A command catalog in the app layer, a generic registration method on the
transport, startup wiring in main.go (log-and-continue on failure), and an e2e test
proving the round-trip against the real test bot.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./CLAUDE.md

<interfaces>
<!-- Existing contracts the executor builds against. No codebase exploration needed. -->

From internal/bot/bot.go (transport wrapper — DO NOT put the command list here):
```go
type Bot struct {
    API        *tgbotapi.BotAPI // already exported
    ChatID     int64
    dispatcher Dispatcher
}
func New(token string, chatID int64, dispatcher Dispatcher) (*Bot, error)
// AckCallback shows the established b.API.Request(...) pattern:
func (b *Bot) AckCallback(callbackID string) error {
    _, err := b.API.Request(tgbotapi.NewCallback(callbackID, ""))
    return err
}
```

From tgbotapi v5 (github.com/go-telegram-bot-api/telegram-bot-api/v5 @ v5.5.1) — verified:
```go
// Each command: tgbotapi.BotCommand{Command: "pull", Description: "..."} // no "/" prefix
func NewSetMyCommands(commands ...BotCommand) SetMyCommandsConfig
func (b *BotAPI) Request(c Chattable) (*APIResponse, error)
func (b *BotAPI) GetMyCommands() ([]BotCommand, error) // for the e2e test
```

From cmd/bot/main.go — current startup wiring (b is constructed at line ~38; token is
valid at that point, before b.Run is launched):
```go
b, err := bot.New(cfg.BotToken, cfg.ChatID, disp) // <- register after this succeeds
...
go b.Run(ctx)
```

From internal/handler/dispatcher.go — the existing single source of command names
(handleCommand switch): "pull", "status", "list", plus "start"/"help" share HelpText.
The catalog mirrors these names; "start" is intentionally SKIPPED.
</interfaces>

From internal/e2e/helpers_test.go — the e2e harness conventions the test must follow:
- Build tag `//go:build integration` (first line, blank line, then `package e2e`).
- `requireTestEnv(t)` returns `(token string, chatID int64)`, reading
  TEST_TELEGRAM_BOT_TOKEN / TEST_TELEGRAM_CHAT_ID with .env fallbacks; it `t.Fatal`s
  if absent. `.env` is auto-loaded by TestMain.
- Uses `github.com/stretchr/testify/require`.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add command catalog, RegisterCommands transport method, and startup wiring</name>
  <files>internal/commands/registry.go, internal/bot/bot.go, cmd/bot/main.go</files>
  <action>
    Per locked decision D-1 (single source of truth in the app layer) and D-3 (commands:
    pull, status, list, help — SKIP start, no "/" prefix):

    1. Create internal/commands/registry.go in `package commands`. Add a top-level
       function `Commands() []tgbotapi.BotCommand` (NOT a method on Adapter — the catalog
       is a separate concern from handler bundling). Import the tgbotapi v5 package.
       Return exactly these four commands in this order, with these descriptions pinned
       (do not invent alternative copy):
         - {Command: "pull",   Description: "Start a questionnaire now"}
         - {Command: "status", Description: "Show current questionnaire state"}
         - {Command: "list",   Description: "List questionnaire schedules"}
         - {Command: "help",   Description: "Show available commands"}

    2. In internal/bot/bot.go add a GENERIC transport method (per D-2 — bot.go must not
       know app-level command semantics, so it takes the slice as a param and does NOT
       define the list):
         func (b *Bot) RegisterCommands(cmds []tgbotapi.BotCommand) error {
             _, err := b.API.Request(tgbotapi.NewSetMyCommands(cmds...))
             return err
         }
       Return the error — do NOT log inside this method. Mirror the existing AckCallback
       Request(...) pattern.

    3. In cmd/bot/main.go, after `bot.New(...)` succeeds (where err is already checked,
       around line 41) and before `go b.Run(ctx)`, call the registration with
       log-and-continue per D-4 (a transient Telegram error must NOT be fatal — it would
       break `docker compose up -d`; the bot works fine without the menu):
         if err := b.RegisterCommands(commands.Commands()); err != nil {
             log.Printf("WARN: failed to register bot commands: %v", err)
         }
       `commands` is already imported in main.go. Do NOT use fatal() here.
  </action>
  <verify>
    <automated>go build ./... && go vet ./...</automated>
  </verify>
  <done>
    internal/commands/registry.go exports Commands() returning the four BotCommands
    (pull/status/list/help, no "/" prefix, start excluded). bot.go has a generic
    RegisterCommands([]tgbotapi.BotCommand) error wrapping NewSetMyCommands via b.API.Request,
    with no internal logging. main.go calls b.RegisterCommands(commands.Commands()) after
    bot.New and before b.Run, logging a warning (not fatal) on error. Build and vet pass.
  </done>
</task>

<task type="auto">
  <name>Task 2: E2E test proving setMyCommands round-trips against the real test bot</name>
  <files>internal/e2e/register_commands_test.go</files>
  <action>
    Per CLAUDE.md test strategy (integration/E2E only, real test bot, no mocks) and the
    constraint that the getMyCommands loop is already proven in internal/e2e/.

    Create internal/e2e/register_commands_test.go. First line `//go:build integration`,
    blank line, then `package e2e`. Add `func TestE2ERegisterCommands(t *testing.T)`.

    The test is LIGHTWEIGHT — do NOT use newBotUnderTest/botRig (that harness exists for
    transport-bypass message injection, which this test does not need). Steps:
      1. token, chatID := requireTestEnv(t)
      2. b, err := bot.New(token, chatID, nil); require.NoError(t, err)
         (nil dispatcher is fine — Run is never started.)
      3. require.NoError(t, b.RegisterCommands(commands.Commands()))
      4. got, err := b.API.GetMyCommands(); require.NoError(t, err)
      5. Assert got matches commands.Commands() exactly (same length, and each
         Command + Description equal in order). Compare with require.Equal on the slices,
         or extract the Command names into a []string and require.Equal against
         []string{"pull","status","list","help"} — at minimum assert the names and count.

    Add a one-line comment noting the test mutates global state on the test bot
    (setMyCommands overwrites whatever was registered) — acceptable for a dedicated test bot.

    Import: tgbotapi v5, testify/require, internal/bot, internal/commands.
  </action>
  <verify>
    <automated>go test ./internal/e2e/... -tags integration -run TestE2ERegisterCommands -v</automated>
  </verify>
  <done>
    TestE2ERegisterCommands registers the catalog against the real test bot and asserts
    GetMyCommands returns exactly pull, status, list, help in registry order. Test passes
    when TEST_TELEGRAM_BOT_TOKEN / TEST_TELEGRAM_CHAT_ID (or .env fallbacks) are present.
  </done>
</task>

</tasks>

<verification>
- `go build ./...` and `go vet ./...` pass (Task 1).
- `go test ./internal/e2e/... -tags integration -run TestE2ERegisterCommands` passes against
  the test bot, proving getMyCommands returns the registered list (was [] before).
- Command list lives in exactly ONE app-layer home (internal/commands/registry.go); bot.go
  remains transport-only; main.go is wiring only.
</verification>

<success_criteria>
- Startup calls setMyCommands with pull/status/list/help (start excluded, no "/" prefix).
- Registration failure logs a warning and does not exit the process.
- E2E test confirms the round-trip against the real test bot.
- No duplication of the command-name list; single source of truth honored.
</success_criteria>

<output>
Create `.planning/quick/260529-qsv-i-want-the-slash-commands-to-be-auto-reg/260529-qsv-SUMMARY.md` when done.
</output>
