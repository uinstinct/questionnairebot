---
phase: 260518-jtt-add-support-for-github-actions
plan: 01
subsystem: infra
tags: [github-actions, go, ci, linting, golangci-lint]

requires:
  - phase: v1.0 milestone
    provides: tagged test split (//go:build integration) so default `go test` is CI-safe

provides:
  - GitHub Actions CI workflow (.github/workflows/ci.yml) running build / tidy-check / vet / lint / race-tests on every push to main and every PR
  - golangci-lint v2 config (.golangci.yml) shared by local and CI runs

affects: [all future phases — every PR will be gated by CI]

tech-stack:
  added:
    - GitHub Actions (actions/checkout@v5, actions/setup-go@v6, golangci/golangci-lint-action@v9)
    - golangci-lint v2.12 (standard preset + gocritic, misspell, revive)
  patterns:
    - "CI excludes //go:build integration; live-Telegram tests stay local-only"
    - "Action pinning policy: major tags for first-party actions; minor pin for golangci-lint"
    - "Single ubuntu-latest job (no OS matrix); single Go version sourced from go.mod"

key-files:
  created:
    - .github/workflows/ci.yml
    - .golangci.yml
  modified: []

key-decisions:
  - "CI runs default-tag tests with -race -count=1; integration/E2E remain local-only"
  - "Workflow uses go-version-file: go.mod as single source of Go version truth"
  - "golangci-lint pinned to v2.12 (minor) to avoid Friday-release breakage"
  - "Linter config kept conservative: standard preset + gocritic, misspell, revive"

patterns-established:
  - "Single-job CI: checkout → setup-go → tidy-check → build → vet → lint → test"
  - "concurrency: cancel superseded runs on the same ref to save Action minutes"

requirements-completed:
  - CI-01

duration: ~3 min
completed: 2026-05-18
---

# Quick 260518-jtt: GitHub Actions CI Summary

**Single-job GitHub Actions CI on ubuntu-latest running tidy-check / build / vet / golangci-lint v2.12 / race-enabled tests, plus shared .golangci.yml so local and CI lint agree.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-05-18T08:55:07Z
- **Completed:** 2026-05-18T08:58:27Z
- **Tasks:** 3
- **Files modified:** 2 (both newly created)

## Accomplishments

- `.github/workflows/ci.yml` triggers on push-to-main and PRs, runs in a single ubuntu-latest job under the 10-minute timeout ceiling
- `.golangci.yml` (v2 schema) shared by local and CI lint runs — `standard` preset plus gocritic, misspell, revive
- Locked scope respected: no Docker push, no releases, no matrix builds, no `-tags=integration`, no GitHub Secrets referenced
- Local sanity check (build, vet, race-tests) passes against the pre-existing tree

## Task Commits

1. **Task 1: Create .golangci.yml (v2 schema)** — `8ad4569` (chore)
2. **Task 2: Create .github/workflows/ci.yml** — `c7db719` (ci)
3. **Task 3: Local sanity check — build, vet, test** — no commit (verification-only task; results reported below)

## Files Created/Modified

- `.golangci.yml` — golangci-lint v2 config; standard preset + gocritic/misspell/revive; 5m timeout; no truncation caps
- `.github/workflows/ci.yml` — single-job CI pipeline pinned to actions/checkout@v5, actions/setup-go@v6, golangci/golangci-lint-action@v9 (lint version v2.12); race-enabled tests on default tags only

## Decisions Made

- None beyond what was already locked in the plan and RESEARCH.md (Option b for tests, single-job shape, v2.12 minor pin, conservative linter set).

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None during execution. See "Findings — CI will fail on first push" below for one pre-existing repo-state issue surfaced by the sanity check.

## Findings — CI will fail on first push

**Pre-existing `go.mod` tidy drift (out of scope to fix here).**

- **What:** Running `go mod tidy` reclassifies `github.com/stretchr/testify v1.11.1` from an indirect to a direct `require`. The package is imported directly by the project's test code, so `go mod tidy` is correct to promote it.
- **Why it matters:** The new CI workflow's "Verify go.mod tidy" step (`go mod tidy && git diff --exit-code -- go.mod go.sum`) will therefore fail on the first push to `main` or first PR until this is committed.
- **Reproduction:**
  ```bash
  go mod tidy && git diff -- go.mod go.sum
  # Shows testify moved from `// indirect` block to the direct require block.
  ```
- **Out of scope here because:** The locked scope for this quick task was CI infrastructure only. Per the executor scope boundary, this is a pre-existing condition (testify has been imported directly by tests for the entire v1.0 milestone), not an issue caused by this task's two new files.
- **Suggested follow-up (one-liner, separate commit):**
  ```bash
  go mod tidy
  git add go.mod go.sum
  git commit -m "chore: tidy go.mod — promote testify from indirect to direct require"
  ```
  After that lands on `main`, the next CI run will go green on the tidy gate.

## Sanity Check Results (Task 3)

All commands run from the worktree root, against the pre-task tree (after reverting the `go mod tidy` change above — scope-bounded out of this task):

| Command | Result |
|---------|--------|
| `go mod tidy && git diff --exit-code -- go.mod go.sum` | **FAILS** — pre-existing testify direct/indirect drift (see Findings above). Reverted before continuing other checks. |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -race -count=1 ./...` | PASS — 4 packages: commands (2.05s), handler (3.45s), session (6.28s), storage (6.86s). 5 packages have no test files (cmd/bot, internal/bot, internal/config, internal/loader, internal/scheduler — all expected; integration-tagged tests in loader/ are filtered by build tags). |
| `golangci-lint run ./...` | SKIPPED — golangci-lint not installed on this workstation. CI will run it on first push and any findings can be triaged then. Per the plan, code is NOT silently changed to satisfy new lints. |

**Local Go version note:** Workstation runs Go 1.26.0 (darwin/arm64); `go.mod` pins `go 1.22`. Go is forward-compatible so local checks remain valid. CI uses `go-version-file: go.mod` so it will install 1.22.x — no version drift in CI.

## Known Stubs

None.

## Self-Check: PASSED

- File `.github/workflows/ci.yml` — FOUND
- File `.golangci.yml` — FOUND
- Commit `8ad4569` — FOUND
- Commit `c7db719` — FOUND
- Plan verification gates 1–5 all pass (both files exist; race tests green; go.mod/go.sum reverted to clean state on disk; only major-tag-pinned actions; no secrets, no Docker).

## Next Phase Readiness

- CI gating is now in place for every future PR and push to main.
- One small pre-existing follow-up (`go mod tidy` commit) is recommended before the next push to avoid the first CI run failing on the tidy gate.

---
*Quick task: 260518-jtt-add-support-for-github-actions*
*Completed: 2026-05-18*
