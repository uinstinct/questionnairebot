# Milestones

## v1.0 MVP (Shipped: 2026-05-18)

**Phases completed:** 5 phases, 15 plans
**Timeline:** 2026-05-18 (single-day burn-down)
**Code:** 3,054 LOC Go; 87 files changed; 31 commits since project init
**Tag:** `v1.0`
**Audit:** ✓ passed (50/50 requirements, 5/5 phases, all E2E flows covered)

**Known deferred items at close:** 1 (see STATE.md `## Deferred Items` — metadata-only mismatch on a quick-task whose work shipped in commit `25af71d` and is rolled into plan 05-04)

**Key accomplishments:**

- **Foundation (Phase 1):** `.env`-driven config, recursive YAML loader with schema + cron + IANA-timezone validation, cron/v3 scheduler that registers one job per questionnaire and logs next-trigger times.
- **Persistence (Phase 2):** Prepend-only `answers.yaml` writer (atomic temp-file rename, newest entry first) and mutex-protected `session.yaml` save/load/delete — no DB, file system is the source of truth.
- **Bot core + Q&A flow (Phase 3):** Telegram long-polling loop with chat-ID auth, command dispatcher, one-question-at-a-time state machine with italic examples, persistence after every answer, startup session restore.
- **Cron triggers + user commands (Phase 4):** Single-fire auto-trigger and multi-fire 1s-window picker, past-due skip algorithm, `/pull` picker + already-active-session reply, `/status` and `/list` formatters.
- **Docker + tests (Phase 5):** Multi-stage Alpine `Dockerfile` (golang:1.22 builder → alpine:3.19 runtime, non-root `botuser`, `ca-certificates`+`tzdata`), `docker-compose.yml` + `.env.example`, integration tests (resume / past-due / status / malformed-yaml), real-bot E2E tests (TEST-03/04), `README.md` with "Running Tests", and E2E user-action mirroring (`👤 …` lines in the live chat).

**Archived:**

- [milestones/v1.0-ROADMAP.md](./milestones/v1.0-ROADMAP.md)
- [milestones/v1.0-REQUIREMENTS.md](./milestones/v1.0-REQUIREMENTS.md)
- [milestones/v1.0-MILESTONE-AUDIT.md](./milestones/v1.0-MILESTONE-AUDIT.md)

---
