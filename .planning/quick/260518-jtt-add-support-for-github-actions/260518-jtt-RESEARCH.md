# Quick Task 260518-jtt: GitHub Actions CI — Research

**Researched:** 2026-05-18
**Domain:** GitHub Actions CI for a Go 1.22 module with integration-tagged tests
**Confidence:** HIGH
**Scope (locked):** Build + test + lint only. NO Docker image build/push, NO releases.

## Summary

This repo is a self-hosted Go 1.22 Telegram bot. There is **no** `.github/` directory yet,
no `Makefile`, and no `.golangci.yml`. The codebase already uses Go's `//go:build integration`
tag to separate tests that need a real Telegram bot (or in-memory integration setup) from the
ones that don't. `go test ./...` without the tag runs cleanly in ~8s with zero external deps
(verified locally: 4 packages pass — `commands`, `handler`, `session`, `storage`).

**Primary recommendation:** Single CI workflow file at `.github/workflows/ci.yml`, one job
on `ubuntu-latest`, single Go version (1.22.x via `go.mod`), running in this order:
`go mod download` → `go build ./...` → `go vet ./...` → `golangci-lint run` → `go test ./...`
(default tag set only — so integration/E2E tests are excluded by construction).

A new `.golangci.yml` should be added in the same PR with a minimal v2 config so linter
results are deterministic across local and CI.

---

## Project Constraints (from CLAUDE.md)

- **Go 1.22+** — match `go.mod` (`go 1.22`).
- **Test strategy: integration + E2E only, real Telegram test bot — no unit tests, no mocks.**
  In practice the repo also has non-tagged in-package tests (handler/flow, session/manager,
  storage, commands/cron, commands/format, commands/pull) that don't talk to Telegram. These
  ARE safe in CI. Only `//go:build integration` files need either env vars or live Telegram.
- **No webhooks.** Irrelevant to CI but worth noting: CI doesn't need to expose any endpoint.
- **Docker / docker-compose distribution** — out of scope per locked decision.

---

## Repo State Inventory (verified by direct read)

| Item | Status | Notes |
|------|--------|-------|
| `.github/workflows/` | **MISSING** — this task creates it | — |
| `go.mod` | exists, `go 1.22` | Deps: `telegram-bot-api/v5`, `robfig/cron/v3`, `yaml.v3`, `godotenv`, `testify` |
| `Makefile` | absent | Not strictly needed; CI can call `go` directly |
| `.golangci.yml` / `.yaml` | **MISSING** — this task should add it | Linter will use built-in defaults otherwise (noisy) |
| `Dockerfile` | exists, multi-stage `golang:1.22-alpine` → `alpine:3.19` | Out of scope for this CI |
| `//go:build integration` tagged tests | 7 files | `e2e/{happy_path,pull_picker,helpers}_test.go`, `handler/restore_integration_test.go`, `commands/{pull,status}_integration_test.go`, `loader/loader_integration_test.go` |
| Non-tagged tests | 8 files | `handler/{dispatcher,flow}_test.go`, `storage/storage_test.go`, `session/manager_test.go`, `commands/{cron,format,pull,testhelpers}_test.go` — all pass with `go test ./...` locally in ~8s |
| E2E env vars | `TEST_TELEGRAM_BOT_TOKEN`, `TEST_TELEGRAM_CHAT_ID` | `requireTestEnv` in `internal/e2e/helpers_test.go` calls `t.Skip` when absent |

**Empirical sanity check:** Ran `go vet ./...` (clean) and `go test ./...` (all 4 test packages
PASS in ~8s, zero external network) before writing this research. So the proposed CI path is
verified, not assumed.

---

## Recommendations

### 1. Test execution strategy — RECOMMENDED: Option (b) variant

**Run `go test ./...` (default tags only) in CI. Do NOT run `-tags=integration` in CI.**

Rationale:
- The repo's own convention already separates "Telegram-required" tests behind `//go:build integration`.
- Even integration tests that *don't* hit Telegram (e.g. `loader_integration_test.go`,
  `handler/restore_integration_test.go`) are gated under the same tag, so CI doesn't see them
  unless explicitly opted in. That's fine: they're already validated locally per v1.0 milestone.
- Adding `TEST_TELEGRAM_BOT_TOKEN` as a GitHub Secret and running live E2E in CI is technically
  possible but introduces:
  - Flake risk on every PR (Telegram API rate-limits, network blips).
  - Cross-PR interference if a forked-PR ever runs (forks don't get secrets by default — good —
    but partial runs cause confusing red builds).
  - A single shared test chat that any concurrent CI run would race against.
- Option (c) — vet+build only, no tests — leaves the 8 non-tagged test packages unrun in CI.
  That's wasteful when they're free and fast.

**Result:** CI runs `go build ./... && go vet ./... && go test ./...`. Integration/E2E remain
local-only, matching how v1.0 was actually validated.

### 2. Workflow file shape

Single file: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  ci:
    name: build / vet / lint / test
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v5

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true   # built-in; keys on go.sum

      - name: Download modules
        run: go mod download

      - name: Verify go.mod tidy
        run: |
          go mod tidy
          git diff --exit-code -- go.mod go.sum

      - name: Build
        run: go build ./...

      - name: Vet
        run: go vet ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.12

      - name: Test
        run: go test -race -count=1 ./...
```

Key choices:
- **`ubuntu-latest`** only. The artifact ships as a Linux Alpine Docker image; no need to
  matrix across macOS/Windows. (Optionally add `windows-latest`/`macos-latest` later if there's
  a real reason; today there isn't.)
- **`go-version-file: go.mod`** — single source of truth, no version drift between repo and CI.
- **Built-in `cache: true`** in `setup-go@v6` — caches `$GOMODCACHE` and `$GOCACHE` keyed on
  `go.sum`. No separate `actions/cache` step needed.
- **`permissions: contents: read`** — least-privilege; CI never writes to the repo.
- **`concurrency` group** cancels superseded runs on the same branch (saves CI minutes on rapid
  PR pushes).
- **`timeout-minutes: 10`** — the whole job is well under 3 min today; 10 is a generous ceiling.
- **`-race -count=1`** — race detector catches the mutex-protected session manager bugs early;
  `-count=1` disables Go's test cache so CI always actually runs.
- **`go mod tidy` + `git diff --exit-code`** — fails CI if anyone forgets to tidy. Cheap insurance.

### 3. Lint — RECOMMENDED: `golangci/golangci-lint-action@v9` with `version: v2.12`

- **Use the official action**, not a hand-rolled `curl | sh` install. v9 is the current major
  (released Dec 2025) and requires an explicit `actions/setup-go` step *before* it — which the
  workflow already has.
- **Pin to a minor** (`v2.12`), not floating `stable`, so a linter release on a Friday afternoon
  doesn't break CI on a Saturday PR.
- **Add a minimal `.golangci.yml` (v2 schema)** so local `golangci-lint run` and CI agree:

  ```yaml
  version: "2"

  run:
    timeout: 5m

  linters:
    default: standard
    # 'standard' enables: errcheck, govet, ineffassign, staticcheck, unused
    enable:
      - gocritic
      - misspell
      - revive

  issues:
    max-issues-per-linter: 0
    max-same-issues: 0
  ```

  Start conservative; add linters as the team has appetite. The "standard" preset alone is a
  reasonable floor.

### 4. Action version pinning policy

Pin to **major tag** (`@v5`, `@v6`, `@v9`) for first-party actions (`actions/*`,
`golangci/golangci-lint-action`). These are well-maintained and SHA-pinning adds churn without
much marginal safety for non-security-sensitive workflows. If the project later adopts a
SHA-pinning policy (e.g. via Dependabot or StepSecurity), upgrade all at once.

Do **not** use floating refs like `@main` or `@master`.

---

## Files this task touches / creates

| Path | Action | Purpose |
|------|--------|---------|
| `.github/workflows/ci.yml` | **CREATE** | Single-job CI: build, vet, lint, test |
| `.golangci.yml` | **CREATE** | v2-schema config so local and CI lint runs agree |

No source code changes. No changes to `go.mod`/`go.sum` (the `go mod tidy` step will verify
they're already tidy — if not, that's a separate fix).

---

## Common pitfalls (and how we avoid them)

| Pitfall | How CI handles it |
|---------|-------------------|
| Forgot to `go mod tidy` before pushing | `git diff --exit-code` after `go mod tidy` step fails the build |
| Cache poisoning across Go versions | `setup-go@v6` `cache: true` keys on Go version + `go.sum` automatically |
| Linter version drift local vs CI | `.golangci.yml` + `version: v2.12` pin in workflow |
| Tests passing locally, failing in CI | `-count=1` disables the test cache; `-race` surfaces concurrency bugs |
| Integration tests need Telegram creds that CI doesn't have | `//go:build integration` tag means they're excluded by default; CI never tries to run them |
| Long PR queue burning Action minutes | `concurrency.cancel-in-progress: true` on `ci-${{ github.ref }}` |
| Forked PR can't access secrets | N/A — workflow uses no secrets |
| Action supply-chain risk | Pin first-party actions to majors; upgrade via Dependabot if desired |
| `golangci-lint-action` v4+ silent breakage | Already covered: workflow has explicit `setup-go` step before the linter action |

---

## Alternatives considered (and rejected)

| Alternative | Why rejected |
|-------------|--------------|
| Matrix across Go versions (1.22 / 1.23 / tip) | Project pins to 1.22 (Dockerfile + go.mod); no benefit until that changes. Adds CI cost. |
| Matrix across OS | Artifact is Linux-only Alpine. No code paths differ by OS. |
| Split into separate `build`, `lint`, `test` jobs | Sequential steps in one job are simpler, run faster on a single runner, and avoid duplicating checkout + setup-go costs. The repo is small. |
| Run integration tests in CI with secrets | Flake risk; single shared test chat = concurrency hazard; v1.0 already validated locally per milestone notes. |
| `make ci` target | No `Makefile` exists; introducing one just for CI is over-engineering. Calling `go` directly is plenty readable. |
| SHA-pin every action | Not warranted for a single-user side project; major-tag pinning is industry baseline. Revisit if/when team grows. |
| Run `gofmt -l` as a separate step | `golangci-lint` v2 standard preset already covers formatting via `gofmt`/`gofumpt` if enabled; not strictly needed but easy add later. |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `golangci-lint` v2.12 has no breaking config schema changes vs. what's documented as the example | §1 Lint | Low — workflow + config can be bumped together if v2.13 changes anything |
| A2 | `actions/checkout@v5` is current stable (v6 is beta per search) | §2 Workflow | Low — v5 is widely deployed; can bump to v6 once GA |

Both assumptions are bounded — if either pin moves, it's a one-line workflow edit.

---

## Sources

### Primary (HIGH confidence — verified)
- Local repo inspection: `go.mod`, `Dockerfile`, `.gitignore`, all `*_test.go` files, `.planning/PROJECT.md`, `.planning/STATE.md`
- Empirical: `go vet ./...` clean + `go test ./...` passes locally in ~8s (no integration tag)
- [golangci-lint-action README + releases](https://github.com/golangci/golangci-lint-action) — v9.2.0 latest, requires explicit `setup-go`, pairs with golangci-lint v2.x

### Secondary (MEDIUM confidence)
- [actions/setup-go releases](https://github.com/actions/setup-go/releases) — v6.4.0 (March 2026) latest major
- [actions/checkout releases](https://github.com/actions/checkout/releases) — v5.0.1 stable; v6 beta
- [golangci-lint changelog](https://golangci-lint.run/docs/product/changelog-v1/) — v2 is current major

---

**Confidence breakdown:**
- Repo state: HIGH — direct file reads + empirical test runs
- Action versions: HIGH — fetched from official repos this session
- Test strategy recommendation: HIGH — driven by what's actually in the repo, not preferences
- Lint config: MEDIUM — minimal config is defensible but team can tune later

**Research valid until:** 2026-06-18 (30 days; stable tooling, no fast-moving deps)
