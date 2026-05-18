<!-- GSD:project-start source:PROJECT.md -->
## Project

**Telegram Questionnaire Bot**

A self-hosted Telegram bot written in Go that delivers scheduled questionnaires to a single
authorised user. Questionnaires are defined as YAML files on disk; the bot reads each file's
cron schedule and automatically messages the user when a questionnaire is due. The user
answers questions one at a time inside Telegram, and answers are persisted back to YAML on
disk. The file system is the sole source of truth — no database. Distributed as a multi-stage
Alpine Docker image with a docker-compose setup.

**Core Value:** Automate the prompt-and-record loop for recurring self-reflection or data-collection
routines so the user only needs to answer, not manage. Zero answer data loss across restarts.

### Constraints

- **Tech stack**: Go 1.22+ — chosen by user; matches PRD Dockerfile builder stage
- **Persistence**: YAML files on disk only — no database, per PRD non-goals
- **Concurrency**: Single Telegram chat ID; multi-questionnaire scheduler; mutex-protected session state
- **Deployment**: Docker / docker-compose only — must run on a standard VPS
- **Telegram transport**: Long-polling — webhooks explicitly out of scope
- **Test strategy**: Integration + E2E only, real Telegram test bot — no unit tests, no mocks
- **Performance**: First question within 5s of cron tick; `/pull` picker within 2s; `docker compose up -d` under 30s on a standard VPS
<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->
## Technology Stack

Technology stack not yet documented. Will populate after codebase mapping or first phase.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
