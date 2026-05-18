# Requirements: Telegram Questionnaire Bot

**Defined:** 2026-05-18
**Core Value:** Automate the prompt-and-record loop for recurring questionnaires so the user only needs to answer, not manage. Zero answer data loss across restarts.

**Source spec:** `/Users/instinct/Desktop/working/questionnairebot/original-prd.md` — authoritative for schemas, acceptance criteria, and edge cases. Every REQ-ID below maps 1:1 to a User Story (US-XXX) or Functional Requirement (FR-XX) in that PRD.

## v1 Requirements

### Configuration & Loading

- [ ] **CFG-01**: Bot reads `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `DATA_DIR` from environment (via `.env`) and exits with a descriptive error before any network call if any are missing. *(maps PRD FR-2)*
- [ ] **LOAD-01**: Bot scans every subdirectory of `DATA_DIR` for `questionnaire.yaml` at startup. *(maps US-001 AC-1)*
- [ ] **LOAD-02**: Each `questionnaire.yaml` is validated: `name` (non-empty string), `schedule` (valid 5-field cron), `timezone` (valid IANA tz via `time.LoadLocation`), `questions` (non-empty array; each item has non-empty `question`, optional `example`). *(maps US-001 AC-2)*
- [ ] **LOAD-03**: Any validation failure → exit code 1 with `FATAL: data/<name>/questionnaire.yaml: <reason>` to stderr. *(maps US-001 AC-3)*
- [ ] **LOAD-04**: On successful load, log to stdout: `Loaded N questionnaire(s): [name-1, name-2, ...]`. *(maps US-001 AC-4)*

### Scheduling

- [ ] **SCHED-01**: One cron job registered per questionnaire using its `schedule` + `timezone`. *(maps US-002 AC-1, FR-4)*
- [ ] **SCHED-02**: On startup, log the next scheduled trigger time for each questionnaire in RFC3339. *(maps US-002 AC-2)*
- [ ] **SCHED-03**: When a single questionnaire's cron fires and that cycle is unanswered, send first question to `TELEGRAM_CHAT_ID` and create `session.yaml`. *(maps US-003 AC-1, FR-5)*
- [ ] **SCHED-04**: If that cycle already has a `completed` entry in `answers.yaml` with matching `scheduled_for`, the auto-trigger is silently skipped (no message, no skip entry). *(maps US-003 AC-2)*
- [ ] **SCHED-05**: When two or more questionnaires fire within the same calendar minute (1s tick window), send a single inline-keyboard picker: `📋 Multiple questionnaires are due. Which would you like to start?` with one button per name. *(maps US-004 AC-1, FR-6)*
- [ ] **SCHED-06**: Tapping a picker button starts that questionnaire (first question + `session.yaml`); the others remain available via `/pull`. *(maps US-004 AC-2/3)*

### Storage & Session

- [ ] **STOR-01**: `answers.yaml` entries are prepended (newest first); file is never fully rewritten from scratch. *(maps FR-11)*
- [ ] **STOR-02**: Completed entry written with `status: completed`, `scheduled_for`, `completed_at` (current time in the questionnaire's timezone), and full `answers` array. *(maps US-007 AC-2)*
- [ ] **STOR-03**: Skipped entry written with `status: skipped`, `scheduled_for: <missed-time>`, `skipped_at: <now>`. *(maps US-005 AC-3 / past-due algorithm)*
- [ ] **SESS-01**: `session.yaml` is rewritten after every recorded answer (state survives a crash mid-session). *(maps FR-12, US-006 AC-4)*
- [ ] **SESS-02**: `session.yaml` is deleted immediately after a completed entry is written. *(maps FR-13, US-007 AC-3)*
- [ ] **SESS-03**: All session-state access is mutex-protected — cron and Telegram polling goroutines must not race. *(maps PRD §8 Concurrency)*

### Bot Core & Question Flow

- [ ] **BOT-01**: Bot polls Telegram via long-polling (no webhook). *(maps FR-1)*
- [ ] **BOT-02**: Updates whose `chat.id` ≠ `TELEGRAM_CHAT_ID` are silently dropped (no reply, no error log). *(maps US-012 AC-1, FR-14)*
- [ ] **BOT-03**: Bot sends each question as a separate Telegram message. *(maps US-006 AC-1)*
- [ ] **BOT-04**: If the question has an `example` field, append on the next line in Telegram italic markdown: `_Example: <text>_`. *(maps US-006 AC-2, FR-10)*
- [ ] **BOT-05**: While a session is active, any text reply is accepted as the answer to the current question — no content validation. *(maps US-006 AC-3, FR-9)*
- [ ] **BOT-06**: After recording an answer, immediately send the next question (after incrementing `current_question_index` and rewriting `session.yaml`). *(maps US-006 AC-5)*
- [ ] **BOT-07**: After the last question, send `✅ [Questionnaire Name] complete! Answers saved.`, prepend completed entry, delete `session.yaml`. *(maps US-007 AC-1)*
- [ ] **BOT-08**: On startup, if a `session.yaml` exists, load it as active; the next user text triggers sending the question at `current_question_index`. *(maps US-008 AC-1/2)*
- [ ] **BOT-09**: If on startup `current_question_index == len(questions)`, finalise the session (write completed entry, delete `session.yaml`). *(maps US-008 AC-3)*
- [ ] **BOT-10**: Free-text received with no active session → reply with the help/commands message; slash commands are routed to their own handlers and bypass this fallback. *(maps US-011 AC-1/2)*

### Commands

- [ ] **CMD-01**: `/pull` shows an inline keyboard with one button per questionnaire that has a pending (unanswered) next cron cycle. *(maps US-005 AC-1)*
- [ ] **CMD-02**: Before computing the pending list, `/pull` auto-skips past-due unanswered cycles (per PRD §8 algorithm). *(maps US-005 AC-3, FR-7)*
- [ ] **CMD-03**: `/pull` exposes only the single next upcoming cron per questionnaire — the cron-after-next is never surfaced. *(maps US-005 AC-4, FR-8)*
- [ ] **CMD-04**: If a session is active, `/pull` replies `⚠️ You have an active session in progress. Please finish it first.` and shows no picker. *(maps US-005 AC-5)*
- [ ] **CMD-05**: If nothing is pending, `/pull` replies `✅ All questionnaires are up to date. Nothing to answer right now.` *(maps US-005 AC-6)*
- [ ] **CMD-06**: `/status` lists every questionnaire with name, last-answered timestamp (or `Never`), next scheduled time in its own timezone, and state (`✅ Done` / `🔄 In Progress` / `⏳ Pending`). *(maps US-009 AC-1/2, FR-15)*
- [ ] **CMD-07**: `/list` lists every questionnaire with display name, cron expression, timezone, and next trigger in RFC3339. *(maps US-010 AC-1)*

### Docker & Deployment

- [ ] **DOCK-01**: `Dockerfile` is two-stage: `golang:1.22-alpine` builder → `alpine:3.19` runtime. *(maps US-013 AC-1)*
- [ ] **DOCK-02**: Builder stage compiles via `CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot`. *(maps US-013 AC-2)*
- [ ] **DOCK-03**: Runtime stage installs `ca-certificates` and `tzdata` (`apk add --no-cache`). *(maps US-013 AC-3, FR-16)*
- [ ] **DOCK-04**: Runtime stage creates and uses non-root user `botuser`. *(maps US-013 AC-4)*
- [ ] **DOCK-05**: `docker build .` succeeds without errors. *(maps US-013 AC-5)*
- [ ] **DOCK-06**: `docker-compose.yml` defines a single `bot` service that builds from local `Dockerfile`, loads `.env` via `env_file`, bind-mounts `./data` → `/app/data`, uses `restart: unless-stopped`. *(maps US-014 AC-1/2/3/4, FR-17)*
- [ ] **DOCK-07**: `docker compose up -d` starts the bot successfully. *(maps US-014 AC-5)*

### Tests & Docs

- [ ] **TEST-01**: Tests source `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID` from the environment. *(maps US-015 AC-1)*
- [ ] **TEST-02**: `README.md` documents required test environment variables under a "Running Tests" section. *(maps US-015 AC-2)*
- [ ] **TEST-03**: E2E test — cron fires → bot sends question 1 → user replies → … → completion message → `answers.yaml` contains correctly structured `completed` entry. *(maps US-015 AC-3)*
- [ ] **TEST-04**: E2E test — `/pull` with two pending questionnaires → picker shown → user selects one → session starts → completes → second still in next `/pull`. *(maps US-015 AC-4)*
- [ ] **TEST-05**: Integration test — partial `session.yaml` on disk → bot restarts → user sends any message → bot sends question at `current_question_index`. *(maps US-015 AC-5)*
- [ ] **TEST-06**: Integration test — past-due cron cycle → `/pull` → skip entry prepended → next upcoming cron shown. *(maps US-015 AC-6)*
- [ ] **TEST-07**: Integration test — `/status` output has correct name, state label, next due time per questionnaire. *(maps US-015 AC-7)*
- [ ] **TEST-08**: Integration test — malformed `questionnaire.yaml` → bot exits with code 1 + `FATAL:` error. *(maps US-015 AC-8)*
- [ ] **TEST-09**: All tests run via `go test ./... -tags integration`. *(maps US-015 AC-9)*

## v2 Requirements

(None — PRD §7 explicitly excludes future features for v1.)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Multiple users / chat IDs | PRD §7 — single-user by design |
| Editing/deleting past answers via Telegram | PRD §7 — answers are append-prepend only |
| Reminders beyond cron auto-message | PRD §7 — no nag system |
| Answer content validation | PRD §7 — any text accepted |
| Web UI / HTTP API | PRD §7 — file system is the interface |
| Unit tests | PRD §7 / US-015 — integration + E2E only |
| Webhook mode | PRD §7 / FR-1 — long-polling only |
| Hot-reload of questionnaire files | PRD §7 — restart required for new files |
| User-initiated `/skip` command | PRD §7 — skipping is automatic |
| Non-text question types (photos, voice, etc.) | PRD §7 — text questions only |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CFG-01 | Phase 1 | Pending |
| LOAD-01 | Phase 1 | Pending |
| LOAD-02 | Phase 1 | Pending |
| LOAD-03 | Phase 1 | Pending |
| LOAD-04 | Phase 1 | Pending |
| SCHED-01 | Phase 1 | Pending |
| SCHED-02 | Phase 1 | Pending |
| STOR-01 | Phase 2 | Pending |
| STOR-02 | Phase 2 | Pending |
| STOR-03 | Phase 2 | Pending |
| SESS-01 | Phase 2 | Pending |
| SESS-02 | Phase 2 | Pending |
| SESS-03 | Phase 2 | Pending |
| BOT-01 | Phase 3 | Pending |
| BOT-02 | Phase 3 | Pending |
| BOT-03 | Phase 3 | Pending |
| BOT-04 | Phase 3 | Pending |
| BOT-05 | Phase 3 | Pending |
| BOT-06 | Phase 3 | Pending |
| BOT-07 | Phase 3 | Pending |
| BOT-08 | Phase 3 | Pending |
| BOT-09 | Phase 3 | Pending |
| BOT-10 | Phase 3 | Pending |
| SCHED-03 | Phase 4 | Pending |
| SCHED-04 | Phase 4 | Pending |
| SCHED-05 | Phase 4 | Pending |
| SCHED-06 | Phase 4 | Pending |
| CMD-01 | Phase 4 | Pending |
| CMD-02 | Phase 4 | Pending |
| CMD-03 | Phase 4 | Pending |
| CMD-04 | Phase 4 | Pending |
| CMD-05 | Phase 4 | Pending |
| CMD-06 | Phase 4 | Pending |
| CMD-07 | Phase 4 | Pending |
| DOCK-01 | Phase 5 | Pending |
| DOCK-02 | Phase 5 | Pending |
| DOCK-03 | Phase 5 | Pending |
| DOCK-04 | Phase 5 | Pending |
| DOCK-05 | Phase 5 | Pending |
| DOCK-06 | Phase 5 | Pending |
| DOCK-07 | Phase 5 | Pending |
| TEST-01 | Phase 5 | Pending |
| TEST-02 | Phase 5 | Pending |
| TEST-03 | Phase 5 | Pending |
| TEST-04 | Phase 5 | Pending |
| TEST-05 | Phase 5 | Pending |
| TEST-06 | Phase 5 | Pending |
| TEST-07 | Phase 5 | Pending |
| TEST-08 | Phase 5 | Pending |
| TEST-09 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 49 total
- Mapped to phases: 49
- Unmapped: 0 ✓

---
*Requirements defined: 2026-05-18*
*Last updated: 2026-05-18 after initial definition*
