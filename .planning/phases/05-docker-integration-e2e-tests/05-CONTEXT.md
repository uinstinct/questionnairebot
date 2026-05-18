# Phase 5: Docker & Integration/E2E Tests - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase — discuss skipped)

<domain>
## Phase Boundary

Project is deployable via `docker compose up -d` on a fresh VPS and the full integration + E2E test suite passes against a real Telegram test bot. README documents env-var requirements.

This phase delivers:
- Multi-stage Alpine `Dockerfile` (golang:1.22-alpine builder → alpine:3.19 runtime + `tzdata`/`ca-certificates`, non-root `botuser`)
- `docker-compose.yml` with `./data` bind-mount and `.env` loading
- `.env.example` documenting required env vars
- Integration tests for session-resume, past-due skip, `/status`, malformed-yaml fatal exit (TEST-05/06/07/08)
- E2E tests against real test bot for happy-path completion and dual-pending `/pull` picker flow (TEST-03/04)
- `README.md` "Running Tests" section listing `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID`
- All tests run via `go test ./... -tags integration` (TEST-09)

Requirements covered: DOCK-01..07, TEST-01..09.

</domain>

<decisions>
## Implementation Decisions

### Docker Image
- Two-stage build: `golang:1.22-alpine` builder → `alpine:3.19` runtime (per DOCK-01, PRD §6)
- Builder: `CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot` (DOCK-02)
- Runtime: `apk add --no-cache ca-certificates tzdata` (DOCK-03, required for IANA tz like `Asia/Kolkata`)
- Non-root user `botuser` created and used (DOCK-04)
- Binary placed at `/app/bot`; `WORKDIR /app`; `CMD ["/app/bot"]`

### docker-compose
- Single `bot` service builds from local `Dockerfile`
- `env_file: .env` loads `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `DATA_DIR`
- Bind mount `./data:/app/data`
- `restart: unless-stopped`
- `DATA_DIR=/app/data` set in `.env.example`

### Test Strategy
- Build tag `integration` gates all integration + E2E tests (TEST-09)
- Tests source `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID` from environment (TEST-01)
- Integration tests use real filesystem (temp dirs) and exercise the bot end-to-end
- E2E tests use real Telegram test bot — open polling, send messages, verify responses
- Use `github.com/stretchr/testify` for assertions (already adopted per PROJECT.md)
- Each test is hermetic: temp `DATA_DIR`, unique questionnaires, cleanup on teardown

### README
- New "Running Tests" section documenting `TEST_TELEGRAM_BOT_TOKEN` and `TEST_TELEGRAM_CHAT_ID` (TEST-02)
- Existing sections (run via docker compose, env vars for production) updated/added as needed for VPS deployment context

### Claude's Discretion
All implementation choices — file layout for tests (e.g., `internal/<pkg>_integration_test.go` vs `tests/` directory), specific timing/sleeps for cron-fire E2E, helpers for spinning up the bot subprocess vs in-process — are at Claude's discretion. Defer to existing codebase conventions (per phase 1-4 patterns) where applicable.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/loader` — questionnaire YAML loading + validation (used in TEST-08 malformed-yaml check)
- `internal/storage` — `answers.yaml` prepend logic (used in TEST-03 completion check)
- `internal/session` — `session.yaml` read/write/delete (used in TEST-05 resume check)
- `internal/scheduler` — cron registration + tick dispatch
- `internal/commands` — `/pull`, `/status`, `/list` handlers with cron computation
- `internal/handler` — Telegram update dispatch, flow, restore
- `internal/bot` — Telegram long-polling loop + auth filter
- `internal/config` — `.env` loading via godotenv
- `cmd/bot/main.go` — entry point composing all the above

### Established Patterns
- Build: `go build -o bot ./cmd/bot` (referenced in DOCK-02)
- Existing tests use `_test.go` files alongside source (no separate `tests/` dir)
- `testify` is the test assertion library
- Phase plans are 1-3 plans each, ~3 plans for this phase

### Integration Points
- Docker `WORKDIR /app`, binary `/app/bot`, data `/app/data` — must align with `DATA_DIR` env var
- Test harness must boot the bot process (or in-process) and interact via real Telegram API

</code_context>

<specifics>
## Specific Ideas

- PRD §6 Dockerfile pattern is canonical — adopt verbatim
- PRD §8 specifies `tzdata` is mandatory for Alpine runtime (else `time.LoadLocation` fails)
- Past-due skip algorithm (PRD §8) is already implemented in `internal/commands` — TEST-06 just verifies its end-to-end behavior via `/pull`

</specifics>

<deferred>
## Deferred Ideas

None — phase scope is well-defined by ROADMAP plans and TEST-01..09 / DOCK-01..07 requirements.

</deferred>
