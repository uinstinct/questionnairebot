---
plan_id: "01-02"
status: complete
date: 2026-05-18
---

# Plan 01-02 Summary — internal/loader

## What Was Built

- `internal/loader/types.go` — `Question`, `Questionnaire` (with `Slug` and `Location` populated post-parse), and `LoadError{Path, Reason}` with `Error()` returning `"path: reason"`.
- `internal/loader/loader.go` — `Load(dataDir)` scans one directory level deep, parses `questionnaire.yaml` via `gopkg.in/yaml.v3`, validates:
  - `name` non-empty,
  - `schedule` parses via `cron.NewParser(... 5-field flags ...)` (rejects 6-field),
  - `timezone` resolves via `time.LoadLocation` (stored on the struct),
  - `questions` non-empty array with non-empty `question` text.
  Errors wrapped as `data/<slug>/questionnaire.yaml: <reason>`. Returns sorted-by-slug slice on success and logs `Loaded N questionnaire(s): [slug-list]`.
- `cmd/bot/main.go` — invokes `loader.Load(cfg.DataDir)`; FATAL exit on error.

## Requirements Closed

- LOAD-01 — directory scan of `${DATA_DIR}/*/questionnaire.yaml`.
- LOAD-02 — schema validation (name / schedule / timezone / questions).
- LOAD-03 — fatal exit with `FATAL: data/<name>/questionnaire.yaml: <reason>`.
- LOAD-04 — `Loaded N questionnaire(s): [...]` stdout log on success.

## Acceptance Evidence

- Two-questionnaire valid load produced: `Loaded 2 questionnaire(s): [daily-standup, weekly]` (sorted).
- `schedule: "not-cron"` → `FATAL: data/x/questionnaire.yaml: invalid schedule: expected exactly 5 fields, found 1: [not-cron]`.
- `timezone: "Mars/Olympus"` → `FATAL: data/x/questionnaire.yaml: invalid timezone: unknown time zone Mars/Olympus`.
- `questions: []` → `FATAL: data/x/questionnaire.yaml: questions is required and must be non-empty`.
- Empty directory → `FATAL: /tmp/qb-empty: no questionnaires found`.
