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

(None yet — ship to validate)

### Active

- [ ] Load and validate questionnaire YAML files at startup (AUTH-style fatal-exit on bad input)
- [ ] Register one cron job per questionnaire with its own IANA timezone
- [ ] Auto-trigger a single questionnaire at cron time and skip already-answered cycles silently
- [ ] When multiple questionnaires fire simultaneously, send an inline-keyboard picker
- [ ] `/pull` command lists pending questionnaires (auto-skipping past-due cycles first)
- [ ] Ask questions one at a time; accept any text as the answer; rewrite `session.yaml` after every answer
- [ ] On completion, prepend a `completed` entry to `answers.yaml` and delete `session.yaml`
- [ ] Resume an interrupted session from `current_question_index` after restart
- [ ] `/status` command summarises every questionnaire's state
- [ ] `/list` command shows every questionnaire's cron + next trigger
- [ ] Free-text with no active session returns help text; slash commands route to handlers
- [ ] Silently drop any update whose `chat.id` ≠ `TELEGRAM_CHAT_ID`
- [ ] Multi-stage Alpine Dockerfile (golang:1.22-alpine builder → alpine:3.19 runtime + tzdata + non-root)
- [ ] docker-compose.yml binds `./data` host directory and loads `.env`
- [ ] Integration + E2E tests using real Telegram test bot (no unit tests)

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
| YAML files as sole persistence | PRD §1 explicit non-goal of DB; simpler ops, human-readable | — Pending |
| Long-polling over webhooks | PRD non-goal; avoids public HTTPS endpoint for a single-user bot | — Pending |
| Prepend (never rewrite) `answers.yaml` | FR-11 — preserves history under partial-write failures | — Pending |
| Rewrite `session.yaml` after every answer | FR-12 — guarantees resumability after crash mid-session | — Pending |
| `current_question_index == len(questions)` on restart → finalise | US-008 AC — covers crash between writing last answer and writing completed entry | — Pending |
| Auto-skip past-due cycles on `/pull` and on cron fire | PRD §8 past-due logic — keeps `answers.yaml` history complete without `/skip` UX | — Pending |
| Drop updates from wrong chat ID silently (no log) | US-012 — denies attacker any signal that the bot saw them | — Pending |
| Standard granularity, 5 horizontal phases | Project structure in PRD §8 is layered (config → loader → scheduler → session → storage → handler → bot); horizontal phases match | — Pending |
| Workflow agents disabled (research/plan-check/verifier) | Comprehensive PRD obviates research; GSD subagents not installed in this runtime | — Pending |

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

---
*Last updated: 2026-05-18 after initialization*
