---
plan_id: "05-02"
status: complete
date: 2026-05-18
---

# Plan 05-02 Summary — Integration tests (resume / past-due / status / malformed YAML)

All tests carry `//go:build integration` so `go test ./...` (no tag) excludes them and `go test ./... -tags integration` includes them.

- `internal/handler/restore_integration_test.go` (`TestRestoreResumesFromMidSession`) — pre-seeds a partial `session.yaml` at `current_question_index=1`, builds the real loader + session.Manager + flow chain with a recording sender, runs `handler.Restore`, and asserts the resumed question is Q2 (not Q1) and goes via `SendMarkdown` because it carries an `example` field.
- `internal/commands/pull_integration_test.go` (`TestPullSkipsPastDueAndSurfacesNextUpcoming`) — seeds a completed entry 25 min ago for a `*/5` cron, calls `pull.Handle` at a fixed "now"; asserts 4 skipped entries are prepended (newest first) and the picker contains exactly one option whose callback data points at the next strictly-future tick.
- `internal/commands/status_integration_test.go` (`TestStatusReportsAllQuestionnaireStates`) — three questionnaires across `Asia/Kolkata` and `UTC`, exercising Done / Pending / In Progress; asserts state labels, `last=Never` for the never-answered one, and `next=…+05:30` vs `next=…Z` per-row timezone formatting.
- `internal/loader/loader_integration_test.go` (`TestLoadFatalsOnMalformedQuestionnaire`) — two subtests (missing `schedule` field; unterminated YAML string) assert `loader.Load` returns a `*LoadError` whose `Path` is `data/<slug>/questionnaire.yaml` and whose `Reason` mentions the failure mode.
- Added `github.com/stretchr/testify v1.11.1` to `go.mod` (already specified as the chosen test assertion library in PROJECT.md).

## Requirements Closed
TEST-05, TEST-06, TEST-07, TEST-08, TEST-09 (build-tag gating).

## Acceptance Evidence
- `go test ./...` exits 0 (integration files skipped via build tag).
- `go test ./... -tags integration` exits 0 with all four new tests passing.
- Each test runs in under one second.
