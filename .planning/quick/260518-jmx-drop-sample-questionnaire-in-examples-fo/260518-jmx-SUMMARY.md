---
quick_id: 260518-jmx
description: drop sample questionnaire in examples folder
date: 2026-05-18
status: complete
commit: 73f7d25
---

# Quick Summary 260518-jmx

Added an `examples/` directory with a reference Daily Standup questionnaire so
users have a copy-pasteable starting point for the YAML shapes documented in
`original-prd.md` §4.

## Files created

- `examples/daily-standup/questionnaire.yaml` — PRD §4.1 sample, three
  questions, two with `example` hints, 09:00 Asia/Kolkata daily cron.
- `examples/daily-standup/answers.yaml` — newest-first answer log with one
  `completed` entry and one `skipped` entry, matching PRD §4.2.
- `examples/README.md` — explains examples are reference-only, shows the
  `cp -r examples/daily-standup data/daily-standup` copy-in step, and points at
  PRD §4.3 for the auto-managed `session.yaml`.

## Verification

- `examples/` lives outside `DATA_DIR` (default `./data`) so the bot never
  auto-loads it.
- YAML shapes match the schemas the bot validates at startup (US-001).

## Commit

`73f7d25` — docs(examples): add reference questionnaire and answer YAML samples
