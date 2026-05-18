# Phase 1: Foundation — Config, Loader, Scheduler init - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning
**Mode:** Smart discuss — infrastructure-only phase (proposals skipped)

<domain>
## Phase Boundary

Bot starts cleanly with `.env`, discovers and validates every `data/*/questionnaire.yaml`, registers one cron job per questionnaire with the correct timezone, and logs the next trigger time. Cron handlers are wired but stubbed (no Telegram I/O yet).

In scope:
- `cmd/bot/main.go` skeleton — startup, graceful shutdown signal handler
- `internal/config` — `.env` loading via `godotenv`, env-var presence + format validation (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `DATA_DIR`)
- `internal/loader` — directory scan of `${DATA_DIR}/*/questionnaire.yaml`, YAML parse, schema validation (name, questions, cron, timezone), cron-expression validation, IANA timezone validation, fatal-exit error messages in the form `FATAL: data/<name>/questionnaire.yaml: <reason>`
- `internal/scheduler` — register one `cron/v3` job per questionnaire using `time.LoadLocation(tz)`, log one RFC3339 next-trigger line per questionnaire, wire stub callbacks (no Telegram I/O)

Out of scope (handled in later phases):
- Telegram bot client and message I/O — Phase 3
- Session state and storage layers — Phase 2
- Real cron-fire side effects (`/pull` flow, pickers) — Phase 4
- Dockerfile and integration tests — Phase 5

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — pure infrastructure phase. PRD §8 and PROJECT.md already specify the layout, libraries, and concurrency model. Use those plus standard Go idioms.

Specifically locked by PROJECT.md / PRD:
- Go 1.22+
- Libraries: `github.com/joho/godotenv`, `github.com/robfig/cron/v3` (must support seconds optional, IANA tz via `cron.WithLocation`), `gopkg.in/yaml.v3`
- Project layout: `cmd/bot/main.go`, `internal/{config,loader,scheduler,...}/`
- Concurrency: cron scheduler in own goroutine; all I/O still mutex-protected later
- Fatal-exit semantics: env-var missing → exit before any network call with descriptive error; bad YAML → `FATAL: data/<name>/questionnaire.yaml: <reason>` and exit code 1

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield Go project. No source files exist yet.

### Established Patterns
- None yet. This phase establishes the package layout (`cmd/bot`, `internal/config`, `internal/loader`, `internal/scheduler`) that all later phases extend.

### Integration Points
- `cmd/bot/main.go` is the wire-up site: loads config, calls `loader.Load(dataDir)`, hands the result to `scheduler.New(...)`, starts the scheduler, and blocks until shutdown signal.
- Scheduler exposes a stub-callback hook that Phase 3/4 will replace with real Telegram bot handlers.

</code_context>

<specifics>
## Specific Ideas

- Use `log.Printf` with the standard library logger for the `Loaded N questionnaire(s): [...]` line and the per-questionnaire `next trigger at <RFC3339>` line. No structured-logging library yet — keep deps minimal.
- Validate cron expressions via `cron.ParseStandard` (5-field) — PRD §3 sample uses 5-field cron (`0 9 * * *`).
- Validate timezone via `time.LoadLocation(tz)` and surface the underlying error verbatim in the FATAL message so the user sees `unknown time zone X` from stdlib.
- `DATA_DIR` should default to `./data` if unset, but `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` must be present (fatal if missing).

</specifics>

<deferred>
## Deferred Ideas

- Hot-reload of YAML files on change — explicitly out of scope per PROJECT.md.
- Multi-chat-ID support — single-tenant by design.

</deferred>
