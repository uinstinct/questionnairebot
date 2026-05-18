# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-05-18
**Phases:** 5 | **Plans:** 15 | **Sessions:** 1 (single-day burn-down)

### What Was Built
- A single-tenant Telegram bot in Go 1.22+ that reads YAML questionnaires from disk, schedules them per their cron expressions, and walks the authorised user through one question at a time inside Telegram.
- Persistence layer where the file system is the only source of truth: prepend-only `answers.yaml` (newest-first, atomic temp-file rename) and mutex-protected `session.yaml` rewritten after every answer for resume-on-restart.
- Cron triggers with the "simultaneous fire" picker (multi-questionnaire inline keyboard within a 1-second window), past-due auto-skip, and the `/pull`, `/status`, `/list` commands.
- Production packaging: multi-stage Alpine Docker image (golang:1.22 builder → alpine:3.19 runtime), non-root `botuser`, `tzdata`+`ca-certificates`, docker-compose with `./data` bind mount.
- Test pyramid: integration tests (resume / past-due / status / malformed-yaml) green inline, plus real-bot E2E tests (TEST-03/04) that mirror user actions to the live chat with `👤 …` for human observability.

### What Worked
- **Horizontal layering matched the PRD's structure exactly.** Phases 1→5 (config → storage → bot core → cron+commands → docker+tests) had clean dependency chains and no rework.
- **Stub-then-implement cron callbacks** (Phase 1 wired stubs, Phase 4 replaced them) kept the dependency graph linear — scheduler tests could pass before the bot existed.
- **Plan 05-04 ergonomics add-on after the main pass** (mirror user actions to the chat) was a clean post-hoc insertion that didn't disturb verified work.
- **Audit-then-complete flow** caught the partial state on phase 5 (untracked plan, missing summary, stale verification status) before tagging.

### What Was Inefficient
- The pre-close `audit-open` flagged plan 05-04 as `in-progress` because the PLAN.md frontmatter wasn't flipped to `complete` even though the SUMMARY existed — a manual one-line fix. A workflow that derives plan status from summary presence would avoid this metadata drift.
- Phase 5 verification ended at `human_needed` for the Docker + real-bot checks and stayed there across multiple sessions. A small "validation requested at $date" timestamp in VERIFICATION.md would make it obvious when the human gate is overdue vs. fresh.

### Patterns Established
- **`internal/e2e` boots the same component graph as `cmd/bot/main.go`** so the wiring is exercised end-to-end without mocks. Adopt this for every future Go service in this repo.
- **`👤 <text>` mirror prefix for in-process test injections** (single hook in `botRig.inject`) — keeps any future E2E run human-readable in the live chat with zero per-test boilerplate.
- **Verification "human_needed" is a first-class terminal state**, not a failure. Operator validates out-of-band and flips to `passed` with a one-line evidence note (e.g. "validated against real test bot 2026-05-18").

### Key Lessons
1. **A 5-phase horizontal plan with a thorough PRD up front beats incremental discovery for greenfield bottom-up work** — the whole milestone shipped in a single day with zero phase-level rework.
2. **Plan an "ergonomics" wave at the end of test-heavy phases.** Plan 05-04 (mirror user actions to Telegram) cost ~15 LOC but made every future E2E run observably correct from the chat, not just from CI output.
3. **When the sandbox can't reach a dependency (Docker daemon, real test bot), make the human gate explicit in VERIFICATION.md with a numbered list of commands to run.** Treats "what the operator must do" as part of the deliverable, not an afterthought.
4. **`git rm` over `rm`** for archived single-source files like REQUIREMENTS.md — preserves history and stages the deletion atomically with the archive commit's safety checkpoint.

### Cost Observations
- Model mix: 100% Opus 4.7 (1M context) — single-model run, no Sonnet/Haiku splits.
- Sessions: 1 main session for execution + 1 audit/lifecycle session.
- Notable: the `--auto` workflow's batched smart-discuss + inline plan/execute kept context efficient enough to finish 5 phases plus full lifecycle in one window.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 1 | 5 | Baseline — horizontal phases, integration+E2E only, no unit tests, file-system source of truth. |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | 4 integration + 2 E2E | go test ./... green; E2E human-validated against real bot | 5 deps adopted (telegram-bot-api/v5, robfig/cron/v3, yaml.v3, godotenv, testify) |

### Top Lessons (Verified Across Milestones)

1. *(needs a second milestone to verify any lesson — list seeded from v1.0 above.)*
