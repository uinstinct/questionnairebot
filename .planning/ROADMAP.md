# Roadmap: Telegram Questionnaire Bot

## Overview

Five horizontal phases build the bot bottom-up: foundation (config + YAML loader + cron registration), then persistent storage (`answers.yaml`/`session.yaml` with concurrency), then the Telegram bot core (polling, routing, question flow, resume), then cron-driven triggers and the user-facing commands (`/pull`, `/status`, `/list`, picker), and finally Docker packaging plus integration/E2E tests. Each phase delivers a coherent layer that the next phase consumes.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation — Config, Loader, Scheduler init** - Boot the process: load `.env`, discover and validate questionnaires, register cron jobs (no triggering yet) (completed 2026-05-18)
- [ ] **Phase 2: Storage & Session State** - Persistent `answers.yaml` prepend + `session.yaml` read/write/delete with mutex-protected access
- [ ] **Phase 3: Bot Core & Question Flow** - Telegram long-polling, chat-ID auth, command routing, question/answer cycle, session resume, free-text fallback
- [ ] **Phase 4: Cron Triggers & Commands** - Wire cron fires into the bot (auto-trigger, simultaneous picker, past-due skip) and ship `/pull`, `/status`, `/list`
- [ ] **Phase 5: Docker & Integration/E2E Tests** - Multi-stage Alpine Dockerfile, docker-compose, README, and the full integration + E2E suite

## Phase Details

### Phase 1: Foundation — Config, Loader, Scheduler init
**Goal**: Bot starts cleanly with `.env`, discovers and validates every `data/*/questionnaire.yaml`, registers one cron job per questionnaire with the correct timezone, and logs the next trigger time. Cron handlers are wired but stubbed (no Telegram I/O yet).
**Depends on**: Nothing (first phase)
**Requirements**: CFG-01, LOAD-01, LOAD-02, LOAD-03, LOAD-04, SCHED-01, SCHED-02
**Success Criteria** (what must be TRUE):
  1. Running the binary with a missing env var exits before any network call with a descriptive error.
  2. Running with a malformed `questionnaire.yaml` exits code 1 and prints `FATAL: data/<name>/questionnaire.yaml: <reason>`.
  3. Running with valid files logs `Loaded N questionnaire(s): [...]` and one RFC3339 next-trigger line per questionnaire.
  4. `go build ./...` and `go vet ./...` pass cleanly.
**Plans**: 3 plans

Plans:
**Wave 1**
- [x] 01-01: `cmd/bot/main.go` skeleton + `internal/config` `.env` loader and env-var validation (CFG-01)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 01-02: `internal/loader` — directory scan + YAML parsing + schema/cron/IANA validation + fatal-exit error messages (LOAD-01..04)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 01-03: `internal/scheduler` — register one `cron/v3` job per questionnaire with `time.LoadLocation`, log next-trigger lines, wire stub callback (SCHED-01/02)

### Phase 2: Storage & Session State
**Goal**: A reusable storage layer that prepends entries to `answers.yaml` (never rewriting), and a session layer that reads/writes/deletes `session.yaml` atomically with mutex-protected concurrent access. Both layers are exercisable in isolation.
**Depends on**: Phase 1
**Requirements**: STOR-01, STOR-02, STOR-03, SESS-01, SESS-02, SESS-03
**Success Criteria** (what must be TRUE):
  1. `storage.PrependCompleted` and `storage.PrependSkipped` produce YAML matching the PRD §4.2 schema and never destroy existing entries (preserve newest-first ordering across many calls).
  2. `session.Save` is durable after each call — partial writes don't corrupt prior `session.yaml`.
  3. `session.Delete` removes `session.yaml` atomically; calling it twice is not an error.
  4. A concurrent stress harness (cron goroutine + handler goroutine writing the same session) shows no data races under `go test -race`.
**Plans**: 2 plans

Plans:
- [ ] 02-01: `internal/storage` — prepend-only `answers.yaml` writer (atomic rename via tempfile), completed/skipped entry builders (STOR-01..03)
- [ ] 02-02: `internal/session` — `Save`/`Load`/`Delete` for `session.yaml` with `sync.Mutex`-protected in-memory active-session registry (SESS-01..03)

### Phase 3: Bot Core & Question Flow
**Goal**: Telegram long-polling loop runs, drops unauthorised chats silently, routes slash commands vs free text, conducts the one-question-at-a-time flow with example italics, persists progress after every answer, finalises sessions, and resumes correctly after restart. Commands `/pull`/`/status`/`/list` are stubbed (Phase 4 implements them); cron callbacks remain stubbed.
**Depends on**: Phase 2
**Requirements**: BOT-01, BOT-02, BOT-03, BOT-04, BOT-05, BOT-06, BOT-07, BOT-08, BOT-09, BOT-10
**Success Criteria** (what must be TRUE):
  1. Sending a message from an unauthorised chat ID produces no reply and no log entry.
  2. Manually starting a session (calling the internal start API) then replying with N text messages produces the completion message and a correctly structured `answers.yaml` completed entry; `session.yaml` is gone.
  3. Killing the bot mid-session and restarting it resumes from the same `current_question_index`; if the session was fully answered before crash, restart finalises it.
  4. Free-text with no active session returns the help message; slash commands invoke their handlers (even if stubbed).
**Plans**: 3 plans

Plans:
- [ ] 03-01: `internal/bot` polling loop + dispatcher + chat-ID auth middleware (BOT-01/02)
- [ ] 03-02: `internal/handler` — question/answer state machine (send question with optional italic example, accept text, advance index, persist after each answer, finalise) (BOT-03..07)
- [ ] 03-03: Startup-time session restore (BOT-08/09) + free-text fallback help message + slash-command router stubs for `/pull`/`/status`/`/list` (BOT-10)

### Phase 4: Cron Triggers & Commands
**Goal**: Replace stubbed cron callbacks and command handlers with real implementations. A cron fire either starts the session, sends a multi-pick picker, or silently no-ops if the cycle is already completed. `/pull` runs the past-due skip algorithm and presents only the next-upcoming pending questionnaires. `/status` and `/list` produce the formatted summaries.
**Depends on**: Phase 3
**Requirements**: SCHED-03, SCHED-04, SCHED-05, SCHED-06, CMD-01, CMD-02, CMD-03, CMD-04, CMD-05, CMD-06, CMD-07
**Success Criteria** (what must be TRUE):
  1. Cron fires for one questionnaire and the cycle is unanswered → the user receives question 1 and `session.yaml` exists for that questionnaire.
  2. Cron fires for one questionnaire and a matching `completed` entry already exists → no message is sent, no skip entry is written.
  3. Two crons fire in the same calendar minute → exactly one `📋 Multiple questionnaires are due. …` picker message is sent with one button per questionnaire; tapping a button starts that session and leaves the others pullable.
  4. `/pull` correctly prepends `skipped` entries for every past-due unanswered cycle since the last recorded entry, then shows a picker of only the next upcoming pending crons (or the "all up to date" / "active session" / picker reply, as appropriate).
  5. `/status` and `/list` outputs render every loaded questionnaire with the fields specified in CMD-06 / CMD-07.
**Plans**: 3 plans

Plans:
- [ ] 04-01: Cron callback implementation — single-fire auto-trigger + already-completed silent skip + multi-fire 1s-window picker grouping (SCHED-03..06)
- [ ] 04-02: `/pull` — past-due skip algorithm, pending-list computation, picker / active-session / nothing-pending replies, callback-query handling (CMD-01..05, FR-7/8)
- [ ] 04-03: `/status` and `/list` formatters (CMD-06/07)

### Phase 5: Docker & Integration/E2E Tests
**Goal**: Project is deployable via `docker compose up -d` on a fresh VPS and the full integration + E2E test suite passes against a real Telegram test bot. README documents env-var requirements.
**Depends on**: Phase 4
**Requirements**: DOCK-01, DOCK-02, DOCK-03, DOCK-04, DOCK-05, DOCK-06, DOCK-07, TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, TEST-08, TEST-09
**Success Criteria** (what must be TRUE):
  1. `docker build .` succeeds and produces an image whose runtime stage runs as `botuser` and contains `tzdata`/`ca-certificates`.
  2. `docker compose up -d` starts the bot in under 30s and `./data` on the host is the mount source for `/app/data`.
  3. `go test ./... -tags integration` passes end-to-end against `TEST_TELEGRAM_BOT_TOKEN`/`TEST_TELEGRAM_CHAT_ID`, covering all eight scenarios in TEST-03 through TEST-08.
  4. `README.md` has a "Running Tests" section listing required env vars.
**Plans**: 3 plans

Plans:
- [ ] 05-01: Multi-stage `Dockerfile` (golang:1.22-alpine builder → alpine:3.19 runtime + `apk add ca-certificates tzdata` + non-root `botuser`) + `docker-compose.yml` + `.env.example` (DOCK-01..07)
- [ ] 05-02: Integration tests — session-resume, past-due skip via `/pull`, `/status` output, malformed-yaml fatal exit (TEST-05/06/07/08)
- [ ] 05-03: E2E tests against real test bot — full happy-path completion and dual-pending `/pull` picker flow; `README.md` "Running Tests" section (TEST-01/02/03/04/09)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation — Config, Loader, Scheduler init | 3/3 | Complete   | 2026-05-18 |
| 2. Storage & Session State | 0/2 | Not started | - |
| 3. Bot Core & Question Flow | 0/3 | Not started | - |
| 4. Cron Triggers & Commands | 0/3 | Not started | - |
| 5. Docker & Integration/E2E Tests | 0/3 | Not started | - |
