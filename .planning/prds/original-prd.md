# PRD: Telegram Questionnaire Bot

## 1. Introduction / Overview

A self-hosted Telegram bot written in Go that delivers scheduled questionnaires to a single
user. Questionnaires are defined as YAML files on disk. The bot reads each file's cron
schedule and automatically messages the user when a questionnaire is due. The user answers
questions one at a time inside Telegram; all answers are written back to disk as YAML. The
system requires no database — the file system is the sole source of truth. The application
is distributed as a Docker image with a docker-compose setup for easy deployment.

**Problem solved:** Recurring self-reflection or data-collection routines (daily standups,
weekly reviews, health logs, etc.) are easy to forget and tedious to organise. This bot
automates the prompt-and-record loop so the user only needs to answer, not manage.

---

## 2. Goals

- Deliver questionnaire questions automatically at their scheduled cron time via Telegram.
- Allow the user to pull and start any currently-due questionnaire on demand via `/pull`.
- Persist every completed session and every skipped cycle to a per-questionnaire YAML answer file in descending timestamp order.
- Resume an interrupted session from the last unanswered question, even after a bot restart.
- Support multiple independent questionnaires, each with its own cron schedule and timezone.
- Expose `/status` and `/list` commands for visibility into questionnaire state.
- Validate all questionnaire YAML files at startup and crash with a clear error if any are malformed.
- Provide comprehensive integration and E2E test coverage using a real Telegram test bot token.
- Ship a multi-stage Dockerfile (Alpine-based) and a docker-compose.yml with host-mounted data directory.

---

## 3. File & Directory Layout

```
data/
  {questionnaire-name}/
    questionnaire.yaml    # Definition (read-only at runtime)
    answers.yaml          # Completed + skipped sessions (prepended, newest first)
    session.yaml          # In-progress session state (exists only during an active session)
```

The `{questionnaire-name}` directory name is the internal ID for the questionnaire
(e.g. `daily-standup`). It must be a lowercase, hyphen-separated filesystem slug.

---

## 4. YAML Schemas

### 4.1 Questionnaire definition (`questionnaire.yaml`)

```yaml
name: "Daily Standup"          # Human-readable display name (required)
schedule: "0 9 * * *"          # Standard 5-field cron expression (required)
timezone: "Asia/Kolkata"       # IANA timezone name (required)
questions:
  - question: "What did you work on yesterday?"
    example: "Finished the authentication module"   # optional
  - question: "What are you working on today?"
  - question: "Any blockers?"
    example: "Waiting on API credentials from the team"
```

### 4.2 Answer file (`answers.yaml`) — newest entry first

```yaml
- status: completed
  scheduled_for: "2025-05-18T09:00:00+05:30"
  completed_at:  "2025-05-18T09:15:32+05:30"
  answers:
    - question: "What did you work on yesterday?"
      answer:   "Finished the auth module"
    - question: "What are you working on today?"
      answer:   "Working on the cron scheduler"
    - question: "Any blockers?"
      answer:   "None"

- status: skipped
  scheduled_for: "2025-05-17T09:00:00+05:30"
  skipped_at:    "2025-05-18T09:00:00+05:30"
```

### 4.3 Session state (`session.yaml`)

```yaml
questionnaire_id: "daily-standup"          # Matches the data/ subdirectory name
scheduled_for: "2025-05-18T09:00:00+05:30"
started_at:    "2025-05-18T09:10:00+05:30"
current_question_index: 2                  # 0-based index of the next unanswered question
answers:
  - question: "What did you work on yesterday?"
    answer:   "Finished the auth module"
  - question: "What are you working on today?"
    answer:   "Working on the cron scheduler"
```

### 4.4 Environment variables (`.env`)

```
TELEGRAM_BOT_TOKEN=<token from BotFather>
TELEGRAM_CHAT_ID=<numeric chat ID of the single authorised user>
DATA_DIR=./data
```

---

## 5. User Stories

### US-001: Load and validate questionnaire files at startup
**Description:** As a developer, I need the bot to discover and validate all questionnaire
YAML files from the data directory at startup so it knows what to schedule.

**Acceptance Criteria:**
- [ ] Bot scans every subdirectory of `DATA_DIR` for a file named `questionnaire.yaml`.
- [ ] Each file is validated: `name` (non-empty string), `schedule` (valid 5-field cron expression), `timezone` (valid IANA timezone), `questions` (non-empty array where each item has a non-empty `question` string and an optional `example` string).
- [ ] If any file fails validation, the bot exits immediately with exit code 1 and prints: `FATAL: data/<name>/questionnaire.yaml: <reason>`.
- [ ] On success, the bot logs to stdout: `Loaded N questionnaire(s): [name-1, name-2, ...]`.
- [ ] Typecheck/lint passes.

---

### US-002: Register cron jobs for each questionnaire
**Description:** As a developer, I need each questionnaire's cron schedule to be registered
with the correct timezone so triggers fire at the right local time.

**Acceptance Criteria:**
- [ ] One cron job is registered per questionnaire using its `schedule` and `timezone` fields.
- [ ] On startup, the bot logs the next scheduled trigger time for each questionnaire in RFC3339 format.
- [ ] Typecheck/lint passes.

---

### US-003: Auto-trigger a single questionnaire at cron time
**Description:** As a user, I want the bot to message me automatically when one questionnaire
is due so I don't have to remember to check.

**Acceptance Criteria:**
- [ ] When exactly one questionnaire's cron fires and that cycle has not already been answered, the bot sends the first question to `TELEGRAM_CHAT_ID` and creates `session.yaml`.
- [ ] If that cycle is already answered (a `completed` entry exists in `answers.yaml` with a matching `scheduled_for`), the auto-trigger is silently skipped (no message sent, no skip entry written — the cycle was completed, not skipped).
- [ ] Typecheck/lint passes.

---

### US-004: Auto-trigger picker when multiple questionnaires fire simultaneously
**Description:** As a user, I want to choose which questionnaire to start when several are
due at the same time.

**Acceptance Criteria:**
- [ ] When two or more questionnaires' crons fire within the same scheduler tick (same clock minute), the bot sends a single message: `📋 Multiple questionnaires are due. Which would you like to start?` with one inline keyboard button per questionnaire name.
- [ ] Tapping a button starts that questionnaire: sends first question and creates `session.yaml`.
- [ ] The remaining due questionnaires stay available via `/pull`.
- [ ] Typecheck/lint passes.

---

### US-005: `/pull` command — list and start a questionnaire
**Description:** As a user, I want to start a questionnaire on demand before or instead of
waiting for the automatic trigger.

**Acceptance Criteria:**
- [ ] `/pull` shows an inline keyboard with one button per questionnaire that has a pending (unanswered) next cron cycle.
- [ ] "Pending" means: the questionnaire's next upcoming cron time has no matching `completed` or `skipped` entry in `answers.yaml`.
- [ ] Before computing the pending list, the bot auto-skips any past-due unanswered cycles: for each cron time between the last recorded entry and `now` that has no `completed` entry, a `skipped` entry is prepended to `answers.yaml` (`status: skipped`, `scheduled_for: <missed time>`, `skipped_at: <now>`).
- [ ] `/pull` only ever surfaces the single next upcoming cron per questionnaire — the cron-after-next cannot be pulled until the next cron time has passed.
- [ ] If a session is already in progress, `/pull` replies: `⚠️ You have an active session in progress. Please finish it first.` and shows no picker.
- [ ] If no questionnaires are pending, `/pull` replies: `✅ All questionnaires are up to date. Nothing to answer right now.`
- [ ] Typecheck/lint passes.

---

### US-006: Ask questions one by one and record answers
**Description:** As a user, I want to receive one question per message and simply type my
answer so the flow feels conversational.

**Acceptance Criteria:**
- [ ] Bot sends each question as a separate Telegram message.
- [ ] If the question has an `example` field, it is appended on the next line in Telegram italic formatting: `_Example: <text>_`.
- [ ] Any text reply from the user while a session is active is accepted as the answer to the current question — no content validation is performed.
- [ ] `current_question_index` in `session.yaml` is incremented and the file is rewritten after each answer is recorded, before the next question is sent.
- [ ] After recording an answer, the bot immediately sends the next question.
- [ ] Typecheck/lint passes.

---

### US-007: Complete a session
**Description:** As a user, I want a confirmation message after answering all questions so I
know my answers were saved.

**Acceptance Criteria:**
- [ ] After the last question is answered, the bot sends: `✅ [Questionnaire Name] complete! Answers saved.`
- [ ] A completed entry is prepended to `answers.yaml` with `status: completed`, `scheduled_for`, `completed_at` (current time in the questionnaire's timezone), and the full `answers` array.
- [ ] `session.yaml` is deleted from disk immediately after writing the completed entry.
- [ ] Typecheck/lint passes.

---

### US-008: Resume an interrupted session after restart
**Description:** As a user, I want to continue answering from where I stopped if I close
Telegram or the bot restarts.

**Acceptance Criteria:**
- [ ] On startup, if `session.yaml` exists for any questionnaire, that session is treated as active and its state is loaded into memory.
- [ ] The next text message from the user triggers the bot to send the question at `current_question_index` from the restored session.
- [ ] If `current_question_index` equals the total number of questions on startup (session was fully answered but not finalised before restart), the bot finalises the session: writes the completed entry and deletes `session.yaml`.
- [ ] Typecheck/lint passes.

---

### US-009: `/status` command
**Description:** As a user, I want a summary of all questionnaires so I can see what's done
and what's coming up.

**Acceptance Criteria:**
- [ ] `/status` replies with a formatted message listing every loaded questionnaire.
- [ ] Each row includes: questionnaire name, last answered timestamp (or `Never`), next scheduled time in the questionnaire's own timezone, and current state: `✅ Done`, `🔄 In Progress`, or `⏳ Pending`.
- [ ] Typecheck/lint passes.

---

### US-010: `/list` command
**Description:** As a user, I want to see all loaded questionnaires and their cron schedules.

**Acceptance Criteria:**
- [ ] `/list` replies with a message listing every loaded questionnaire: display name, cron expression, timezone, and next trigger time in RFC3339 format.
- [ ] Typecheck/lint passes.

---

### US-011: Handle unexpected free-text with no active session
**Description:** As a user, I want a helpful nudge if I type something when no questionnaire
is active, rather than silence.

**Acceptance Criteria:**
- [ ] When the bot receives a free-text message and no session is active, it replies with:
  ```
  I'm not sure what to do with that. Available commands:
  /pull   — Start a pending questionnaire
  /status — See all questionnaire statuses
  /list   — See all loaded questionnaires
  ```
- [ ] Telegram slash commands (prefixed with `/`) are routed to their own handlers and do not trigger this fallback.
- [ ] Typecheck/lint passes.

---

### US-012: Ignore messages from unauthorised chat IDs
**Description:** As a developer, I want the bot to ignore all messages from any chat ID
other than the configured one.

**Acceptance Criteria:**
- [ ] Any Telegram update whose `chat.id` does not match `TELEGRAM_CHAT_ID` is silently dropped — no reply is sent and no error is logged.
- [ ] Typecheck/lint passes.

---

### US-013: Dockerfile — multi-stage Alpine build
**Description:** As a developer, I want a production-ready Docker image so the bot can be
deployed anywhere Docker is available.

**Acceptance Criteria:**
- [ ] `Dockerfile` uses a two-stage build: stage 1 uses `golang:1.22-alpine` (builder), stage 2 uses `alpine:3.19` (runtime).
- [ ] Builder stage compiles with `CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot`.
- [ ] Runtime stage runs `apk add --no-cache ca-certificates tzdata` (HTTPS polling + IANA timezone resolution).
- [ ] Runtime stage creates and uses a non-root user (`botuser`).
- [ ] `docker build .` succeeds without errors.
- [ ] Typecheck/lint passes.

---

### US-014: docker-compose.yml for deployment
**Description:** As a developer, I want a docker-compose file so the bot is deployable with
a single command and answer data persists on the host machine.

**Acceptance Criteria:**
- [ ] `docker-compose.yml` defines a single service (`bot`) that builds from the local `Dockerfile`.
- [ ] The service loads environment variables from `.env` via `env_file`.
- [ ] The host `./data` directory is mounted into the container at `/app/data` (read-write volume).
- [ ] The service has `restart: unless-stopped`.
- [ ] Running `docker compose up -d` starts the bot successfully.
- [ ] Typecheck/lint passes.

---

### US-015: Integration and E2E tests
**Description:** As a developer, I need comprehensive integration and E2E tests so I can
trust the bot behaves correctly across all critical flows.

**Acceptance Criteria:**
- [ ] Tests use a real Telegram bot token and test chat ID sourced from `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID` environment variables.
- [ ] Required test environment variables are documented in `README.md` under a "Running Tests" section.
- [ ] E2E test: cron fires → bot sends question 1 → user replies → question 2 → ... → completion message → `answers.yaml` contains correctly structured `completed` entry.
- [ ] E2E test: `/pull` with two pending questionnaires → picker shown → user selects one → session starts → completes → second questionnaire still shows in next `/pull`.
- [ ] Integration test: partial `session.yaml` written to disk → bot restarts → user sends any message → bot sends question at `current_question_index`.
- [ ] Integration test: past-due cron cycle → `/pull` called → skip entry prepended to `answers.yaml` → next upcoming cron shown in picker.
- [ ] Integration test: `/status` output contains correct name, state label, and next due time for each questionnaire.
- [ ] Integration test: malformed `questionnaire.yaml` → bot exits with code 1 and prints `FATAL:` error message.
- [ ] All tests run with `go test ./... -tags integration`.
- [ ] Typecheck/lint passes.

---

## 6. Functional Requirements

- **FR-1:** The bot must poll Telegram for updates using long-polling (no webhook).
- **FR-2:** On startup, `DATA_DIR`, `TELEGRAM_BOT_TOKEN`, and `TELEGRAM_CHAT_ID` must all be present in the environment; if any are missing the bot must exit with a descriptive error before attempting any network call.
- **FR-3:** All questionnaire YAML files must be validated at startup; any malformed file causes an immediate exit (see US-001).
- **FR-4:** Each questionnaire's cron job must be scheduled using the timezone from its `timezone` field via `time.LoadLocation`.
- **FR-5:** When a cron fires, the bot must check whether the current cycle is already answered before sending any message (see US-003).
- **FR-6:** When a cron fires for multiple questionnaires simultaneously (same clock minute), the bot must send a single inline-keyboard picker message (see US-004).
- **FR-7:** `/pull` must auto-skip and log past-due unanswered cycles before computing the pending list (see US-005).
- **FR-8:** `/pull` must never surface the cron-after-next; only the single next upcoming cron per questionnaire is exposed.
- **FR-9:** While a session is active, all user text is treated as an answer. Slash commands are still routed to their handlers.
- **FR-10:** Questions with an `example` field must render the example in Telegram italic markdown on the line below the question text.
- **FR-11:** `answers.yaml` entries are prepended (newest first). The file must never be fully rewritten from scratch — new entries are prepended to preserve existing content.
- **FR-12:** `session.yaml` must be rewritten after every recorded answer so state survives a crash between questions.
- **FR-13:** `session.yaml` must be deleted immediately after a session completes or is finalised on restart.
- **FR-14:** Messages from any chat ID other than `TELEGRAM_CHAT_ID` must be silently dropped.
- **FR-15:** `/status` must display each questionnaire's name, last answered timestamp, next due time (in the questionnaire's own timezone), and current state label.
- **FR-16:** The Docker runtime image must install `tzdata` so IANA timezone names resolve correctly inside the Alpine container.
- **FR-17:** The `docker-compose.yml` must bind-mount `./data` from the host so answer and session files survive container recreation.

---

## 7. Non-Goals (Out of Scope)

- No support for multiple users or multiple chat IDs.
- No editing or deleting past answers via Telegram.
- No notification/reminder system beyond the cron-triggered auto-message.
- No answer content validation (any text is accepted).
- No web UI or API for viewing answers.
- No unit tests (integration and E2E tests only, per US-015).
- No webhook mode (long-polling only).
- No hot-reload of questionnaire files at runtime (restart required to pick up new files).
- No user-initiated `/skip` command (skipping is automatic and logged, not user-driven).
- No support for non-text question types (photos, voice, etc.).

---

## 8. Technical Considerations

### Recommended Go Libraries

| Purpose | Library |
|---|---|
| Telegram Bot API | `github.com/go-telegram-bot-api/telegram-bot-api/v5` |
| Cron scheduling | `github.com/robfig/cron/v3` |
| YAML parsing | `gopkg.in/yaml.v3` |
| `.env` loading | `github.com/joho/godotenv` |
| Test assertions | `github.com/stretchr/testify` |

### Recommended Project Structure

```
cmd/
  bot/
    main.go               # Entry point: load config, init bot, start scheduler + poller
internal/
  config/                 # .env loading and env var validation
  loader/                 # Questionnaire YAML discovery and schema validation
  scheduler/              # Cron job registration and trigger logic
  session/                # session.yaml read / write / delete
  storage/                # answers.yaml prepend logic
  handler/                # Telegram update router (commands + free-text + callbacks)
  bot/                    # Bot lifecycle: polling loop and dispatcher
data/                     # Host-mounted in Docker; gitignored except example
  example-questionnaire/
    questionnaire.yaml
Dockerfile
docker-compose.yml
.env.example
README.md
```

### Concurrency

The cron scheduler and the Telegram polling loop run in separate goroutines. Both may read
and write session state concurrently. All session state access must be protected by a
`sync.Mutex` or equivalent to prevent data races.

### Past-Due Skip Logic (detailed)

When `/pull` is called or a cron fires, the bot computes skips as follows:

1. Find the last recorded entry in `answers.yaml` (completed or skipped). Its `scheduled_for` is the baseline.
2. Iterate cron times from baseline+1 tick up to `now`.
3. For each cron time with no matching `completed` entry, prepend a `skipped` entry.
4. The next cron time after `now` is the "next upcoming cron" shown in `/pull`.

### "Simultaneously Firing" Definition

Two questionnaires are considered to fire simultaneously if their computed next cron time
falls in the same calendar minute. The scheduler groups triggers within a 1-second window
after the tick.

### Docker Timezone Resolution

Alpine Linux does not include IANA timezone data by default. The Dockerfile runtime stage
must run `apk add --no-cache tzdata`. Without this, `time.LoadLocation("Asia/Kolkata")`
returns an error and the bot will fail to start.

---

## 9. Docker Artifacts

### Dockerfile

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S botgroup && adduser -S botuser -G botgroup
WORKDIR /app
COPY --from=builder /app/bot .
USER botuser
CMD ["./bot"]
```

### docker-compose.yml

```yaml
services:
  bot:
    build: .
    env_file: .env
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

### .env.example

```
TELEGRAM_BOT_TOKEN=your-token-here
TELEGRAM_CHAT_ID=123456789
DATA_DIR=/app/data
```

---

## 10. Success Metrics

- Bot delivers the first question within 5 seconds of the cron tick.
- `/pull` displays the picker in under 2 seconds.
- Zero answer data loss across bot restarts (all session resume scenarios pass in tests).
- All integration and E2E tests pass on a clean run: `go test ./... -tags integration`.
- `docker compose up -d` starts the bot in under 30 seconds on a standard VPS.
- A completed `answers.yaml` session entry for a 5-question questionnaire is correctly structured and human-readable without additional tooling.
