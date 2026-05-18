# Telegram Questionnaire Bot

## What This Is

A self-hosted Telegram bot written in Go that delivers scheduled questionnaires to a single
authorised user. Questionnaires are defined as YAML files on disk; the bot reads each file's
cron schedule and automatically messages the user when a questionnaire is due. The user
answers questions one at a time inside Telegram, and answers are persisted back to YAML on
disk. The file system is the sole source of truth — no database. Distributed as a multi-stage
Alpine Docker image with a docker-compose setup.

## Core Value

Automate the prompt-and-record loop for recurring self-reflection or data-collection
routines so the user only needs to answer, not manage. Zero answer data loss across restarts.

## Requirements

### Validated

- ✓ Load and validate questionnaire YAML files at startup (fatal-exit on bad input) — v1.0
- ✓ Register one cron job per questionnaire with its own IANA timezone — v1.0
- ✓ Auto-trigger a single questionnaire at cron time and skip already-answered cycles silently — v1.0
- ✓ When multiple questionnaires fire simultaneously, send an inline-keyboard picker — v1.0
- ✓ `/pull` command lists pending questionnaires (auto-skipping past-due cycles first) — v1.0
- ✓ Ask questions one at a time; accept any text as the answer; rewrite `session.yaml` after every answer — v1.0
- ✓ On completion, prepend a `completed` entry to `answers.yaml` and delete `session.yaml` — v1.0
- ✓ Resume an interrupted session from `current_question_index` after restart — v1.0
- ✓ `/status` command summarises every questionnaire's state — v1.0
- ✓ `/list` command shows every questionnaire's cron + next trigger — v1.0
- ✓ Free-text with no active session returns help text; slash commands route to handlers — v1.0
- ✓ Silently drop any update whose `chat.id` ≠ `TELEGRAM_CHAT_ID` — v1.0
- ✓ Multi-stage Alpine Dockerfile (golang:1.22-alpine builder → alpine:3.19 runtime + tzdata + non-root) — v1.0
- ✓ docker-compose.yml binds `./data` host directory and loads `.env` — v1.0
- ✓ Integration + E2E tests using real Telegram test bot (no unit tests) — v1.0

### Active

(All v1.0 requirements shipped. Scope the next milestone via `/gsd:new-milestone`.)

### Out of Scope

- Multiple users / multiple chat IDs — bot is single-tenant by design
- Editing or deleting past answers via Telegram — answers are append-prepend only
- Reminders beyond the cron auto-message — no nag system
- Answer content validation — any text is a valid answer
- Web UI / HTTP API for viewing answers — file system is the interface
- Unit tests — integration + E2E only (per US-015)
- Webhook mode — long-polling only
- Hot-reload of questionnaire files — restart required for new files
- User-initiated `/skip` command — skipping is automatic
- Non-text question types (photos, voice, etc.)

## Context

**Source spec:** Full Product Requirements Document lives at `/Users/instinct/Desktop/working/questionnairebot/original-prd.md`. Every phase plan should reference it for the authoritative spec, schemas, and acceptance criteria.

**Greenfield Go project.** No existing code. Project root is `/Users/instinct/Desktop/working/questionnairebot`. Will use Go 1.22+ and the libraries listed in PRD §8.

**Recommended Go libraries** (from PRD §8 — adopted):
- Telegram Bot API: `github.com/go-telegram-bot-api/telegram-bot-api/v5`
- Cron: `github.com/robfig/cron/v3`
- YAML: `gopkg.in/yaml.v3`
- `.env`: `github.com/joho/godotenv`
- Test assertions: `github.com/stretchr/testify`

**Project layout** (from PRD §8 — adopted verbatim):
```
cmd/bot/main.go
internal/
  config/   loader/   scheduler/   session/   storage/   handler/   bot/
data/{questionnaire}/
  questionnaire.yaml   answers.yaml   session.yaml
Dockerfile   docker-compose.yml   .env.example   README.md
```

**Concurrency model:** Cron scheduler and Telegram polling loop run in separate goroutines. All session state access is mutex-protected (`sync.Mutex`).

**Past-due skip algorithm** (from PRD §8):
1. Last recorded `answers.yaml` entry → baseline.
2. Iterate cron times from baseline+1 to `now`.
3. Each unmatched cron time → prepend a `skipped` entry.
4. First cron after `now` = "next upcoming cron" surfaced by `/pull`.

**"Simultaneous fire" definition:** Two questionnaires fire simultaneously if their next cron times fall in the same calendar minute. Scheduler groups triggers within a 1-second window after the tick.

**Docker timezone:** Alpine has no IANA timezone data by default. Runtime stage must `apk add --no-cache tzdata`, otherwise `time.LoadLocation("Asia/Kolkata")` fails and the bot won't start.

## Constraints

- **Tech stack**: Go 1.22+ — chosen by user; matches PRD Dockerfile builder stage
- **Persistence**: YAML files on disk only — no database, per PRD non-goals
- **Concurrency**: Single Telegram chat ID; multi-questionnaire scheduler; mutex-protected session state
- **Deployment**: Docker / docker-compose only — must run on a standard VPS
- **Telegram transport**: Long-polling — webhooks explicitly out of scope
- **Test strategy**: Integration + E2E only, real Telegram test bot — no unit tests, no mocks
- **Performance**: First question within 5s of cron tick; `/pull` picker within 2s; `docker compose up -d` under 30s on a standard VPS

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| YAML files as sole persistence | PRD §1 explicit non-goal of DB; simpler ops, human-readable | ✓ Good (v1.0) |
| Long-polling over webhooks | PRD non-goal; avoids public HTTPS endpoint for a single-user bot | ✓ Good (v1.0) |
| Prepend (never rewrite) `answers.yaml` | FR-11 — preserves history under partial-write failures | ✓ Good (v1.0) |
| Rewrite `session.yaml` after every answer | FR-12 — guarantees resumability after crash mid-session | ✓ Good (v1.0) |
| `current_question_index == len(questions)` on restart → finalise | US-008 AC — covers crash between writing last answer and writing completed entry | ✓ Good (v1.0) |
| Auto-skip past-due cycles on `/pull` and on cron fire | PRD §8 past-due logic — keeps `answers.yaml` history complete without `/skip` UX | ✓ Good (v1.0) |
| Drop updates from wrong chat ID silently (no log) | US-012 — denies attacker any signal that the bot saw them | ✓ Good (v1.0) |
| Standard granularity, 5 horizontal phases | Project structure in PRD §8 is layered (config → loader → scheduler → session → storage → handler → bot); horizontal phases match | ✓ Good (v1.0) |
| Workflow agents disabled (research/plan-check/verifier) | Comprehensive PRD obviates research; GSD subagents not installed in this runtime | ✓ Good (v1.0) |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

## Current State

**Shipped:** v1.0 MVP (2026-05-18)

- 3,054 LOC Go across 8 internal packages (`config`, `loader`, `scheduler`, `session`, `storage`, `handler`, `bot`, `commands`) + `internal/e2e` test harness + `cmd/bot/main.go`.
- Wiring graph: `config.Load → loader.Load → session.Manager → handler.QuestionFlow → handler.Dispatcher → bot.Bot → commands.{CronBus, Pull, Status, List}` + `scheduler.Start(callback=bus.Fire)`.
- Tests: integration suite (resume / past-due / status / malformed-yaml) green inline; E2E suite against a real Telegram test bot validated by operator on 2026-05-18.
- Distribution: multi-stage Alpine Dockerfile (non-root `botuser`, `tzdata`+`ca-certificates`) + docker-compose with `./data` bind mount; human-validated `docker build` + `docker compose up -d` on a real host.

**Known tech debt:** none recorded.
**Open deferred items:** 1 metadata-only quick-task mismatch (see STATE.md `## Deferred Items`).

## Next Milestone Goals

To be defined via `/gsd:new-milestone`.

---
*Last updated: 2026-05-18 after v1.0 milestone*
