# Phase 2: Storage & Session State - Context

**Gathered:** 2026-05-18
**Status:** Ready for planning
**Mode:** Smart discuss — infrastructure/data-layer phase (proposals skipped)

<domain>
## Phase Boundary

Two reusable file-I/O layers exercisable in isolation:

1. `internal/storage` — append-prepend writer for `data/<slug>/answers.yaml`. Entries are *prepended* (newest first); the file is **never** fully rewritten from scratch. Atomic via temp-file + rename. Two entry types: completed and skipped (PRD §4.2 schema).
2. `internal/session` — `Save`/`Load`/`Delete` for `data/<slug>/session.yaml`, plus an in-memory mutex-protected registry tracking active sessions. Save uses tempfile+rename for crash safety. Delete is idempotent. All access is `sync.Mutex`-protected so cron and polling goroutines can't race.

Out of scope (Phase 3+):
- Telegram I/O.
- Reading session.yaml at startup to resume (Phase 3 plumbs this).
- Past-due skip computation against `answers.yaml` (Phase 4).

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
- Atomic write strategy: write to `<final>.tmp` (same directory, so rename is atomic on the same filesystem) then `os.Rename` to final path. fsync the tempfile before rename for crash safety.
- Prepend strategy: read existing file bytes, marshal new entry to YAML, then write `[new entry yaml][existing bytes]` — the YAML stream remains a valid list because each entry begins with `- status:`. Missing-file case: just write the new entry.
- Session registry: a single struct `*session.Manager` holding `map[slug]*Session` plus a `sync.Mutex`. Manager is constructed once in main and passed to consumers.
- Time format: PRD uses RFC3339 with timezone offset; both `completed_at` and `skipped_at` use `time.Now().In(loc).Format(time.RFC3339)`.
- `internal/session` does NOT call into `internal/storage` — the orchestration (write completed entry + delete session.yaml) lives in the caller (Phase 3 handler). Keeps each layer independently testable.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/loader.Questionnaire` carries the per-questionnaire `*time.Location` — pass it (or just the slug + location) to storage helpers for time formatting.
- `gopkg.in/yaml.v3` is already a dep.

### Established Patterns
- Errors returned as values, not panicked.
- Package-level functions in `internal/loader` (no constructor). Use the same shape for storage; session uses a constructor (`session.NewManager()`) because it holds the mutex+map.

### Integration Points
- Future: Phase 3 handler calls `session.Save` after every answer (SESS-01), and on completion calls `storage.PrependCompleted(...)` then `session.Delete(...)` (SESS-02).
- Future: Phase 4 past-due algorithm reads `answers.yaml` to find the last entry — expose a `storage.LastEntry(dataDir, slug)` helper.

</code_context>

<specifics>
## Specific Ideas

- `session.Manager` exposes `Start(slug, scheduled, started time.Time, ...)`, `RecordAnswer(slug, q, a string)`, `Get(slug)`, `Delete(slug)`. All take the slug as the key so callers don't pass session pointers around.
- A separate `LoadFromDisk(slug, dataDir)` helper rehydrates active sessions on startup (Phase 3 calls this once per slug).
- `go test -race` will be exercised in Phase 5; this phase's success criterion (4) just demands the code be race-free *when* such a test exists. We will add a smoke `_test.go` that drives concurrent Save calls so `go test ./internal/session -race` passes now.

</specifics>

<deferred>
## Deferred Ideas

- Storage file locking across processes — single-process bot, not needed.
- Compaction of `answers.yaml` — out of scope (PRD non-goal).

</deferred>
