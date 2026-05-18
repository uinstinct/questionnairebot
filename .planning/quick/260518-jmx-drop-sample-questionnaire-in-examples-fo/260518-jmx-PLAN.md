---
quick_id: 260518-jmx
description: drop sample questionnaire in examples folder
date: 2026-05-18
---

# Quick Plan 260518-jmx: drop sample questionnaire in examples folder

## Goal

Ship a reference questionnaire under `examples/` so a new user can see the on-disk
YAML shapes (definition + answer log) without spinning up the bot. `data/` stays
empty — examples are documentation, not runtime input.

## Tasks

1. `examples/daily-standup/questionnaire.yaml` — mirrors the PRD §4.1 sample
   (Daily Standup, 09:00 IST, three questions, two with `example` hints).
2. `examples/daily-standup/answers.yaml` — newest-first log with one `completed`
   and one `skipped` entry, matching PRD §4.2.
3. `examples/README.md` — explains `examples/` is reference-only, shows the
   `cp -r examples/daily-standup data/daily-standup` copy-in path, and points at
   PRD §4.3 for the in-progress `session.yaml` shape (not shipped — never
   hand-authored).

## must_haves

- `examples/daily-standup/questionnaire.yaml` is valid against the PRD §4.1
  schema (name, schedule, timezone, questions[].question, optional
  questions[].example).
- `examples/daily-standup/answers.yaml` is valid against PRD §4.2 (array,
  status, scheduled_for, completed_at/skipped_at, answers[]).
- `examples/` is not under `data/` — no risk of being auto-loaded by the bot.
