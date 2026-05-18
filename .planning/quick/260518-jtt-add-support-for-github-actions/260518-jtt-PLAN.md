---
phase: 260518-jtt-add-support-for-github-actions
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .github/workflows/ci.yml
  - .golangci.yml
autonomous: true
requirements:
  - CI-01
tags:
  - github-actions
  - go
  - ci
  - linting

must_haves:
  truths:
    - "Every push to main and every pull request triggers the CI workflow on ubuntu-latest"
    - "CI fails when go.mod / go.sum are not tidy"
    - "CI fails when go build, go vet, golangci-lint, or go test report errors"
    - "Integration / E2E tests (//go:build integration) are NOT executed in CI"
    - "Race detector runs against the default-tag test set in CI"
    - "Local golangci-lint run and CI golangci-lint run use the same config (.golangci.yml)"
  artifacts:
    - path: .github/workflows/ci.yml
      provides: "Single-job CI pipeline: checkout, setup-go, mod tidy check, build, vet, lint, test"
      contains: "go test ./..."
    - path: .golangci.yml
      provides: "golangci-lint v2 config — standard preset + gocritic, misspell, revive"
      contains: "version: \"2\""
  key_links:
    - from: .github/workflows/ci.yml
      to: go.mod
      via: "actions/setup-go@v6 with go-version-file: go.mod"
      pattern: "go-version-file:\\s*go\\.mod"
    - from: .github/workflows/ci.yml
      to: .golangci.yml
      via: "golangci/golangci-lint-action@v9 reads .golangci.yml from repo root"
      pattern: "golangci/golangci-lint-action"
---

<objective>
Add a GitHub Actions CI workflow plus a matching golangci-lint config so every push to `main`
and every pull request automatically verifies that the Go module is tidy, builds cleanly,
passes `go vet`, passes `golangci-lint`, and passes the default (non-integration) test set
with the race detector enabled.

Purpose: lock in the verification baseline that was previously only run manually, without
introducing flake from live-Telegram integration / E2E tests in CI.

Output:
- `.github/workflows/ci.yml` — single job on `ubuntu-latest`, runs in under 3 minutes today.
- `.golangci.yml` — v2-schema config so local and CI linter runs agree.
</objective>

<execution_context>
@/Users/instinct/Desktop/working/questionnairebot/.claude/get-shit-done/workflows/execute-plan.md
@/Users/instinct/Desktop/working/questionnairebot/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@CLAUDE.md
@.planning/STATE.md
@.planning/quick/260518-jtt-add-support-for-github-actions/260518-jtt-RESEARCH.md
@go.mod

<interfaces>
<!-- Repo facts the executor will rely on (verified by research). -->

Go version (from go.mod): `go 1.22`

Test layout (verified by research):
- Non-tagged test packages (run by `go test ./...` by default, ~8s total locally):
  internal/handler, internal/session, internal/storage, internal/commands
- Integration-tagged test files (`//go:build integration` — EXCLUDED in CI):
  internal/e2e/{happy_path,pull_picker,helpers}_test.go,
  internal/handler/restore_integration_test.go,
  internal/commands/{pull,status}_integration_test.go,
  internal/loader/loader_integration_test.go

Action versions to pin (per research §4):
- actions/checkout@v5
- actions/setup-go@v6
- golangci/golangci-lint-action@v9 with `version: v2.12`

Constraint: workflow must NOT use any GitHub Secrets — there are none for this project and
forked PRs would fail otherwise.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create .golangci.yml (v2 schema)</name>
  <files>.golangci.yml</files>
  <action>
    Create `.golangci.yml` at repo root using the golangci-lint v2 schema, matching the
    research recommendation in §3:

    - Top-level `version: "2"`.
    - `run.timeout: 5m`.
    - `linters.default: standard` (this enables errcheck, govet, ineffassign, staticcheck, unused).
    - `linters.enable:` add `gocritic`, `misspell`, `revive`.
    - `issues.max-issues-per-linter: 0` and `issues.max-same-issues: 0` so nothing is silently
      truncated.

    Do NOT add YAML comments — keep the file minimal per the project's no-comment convention.
    Do NOT enable additional linters beyond what's listed; the research explicitly recommends
    starting conservative.

    Verify locally if golangci-lint is available; otherwise rely on the CI step in Task 2 to
    validate the config.
  </action>
  <verify>
    <automated>test -f .golangci.yml &amp;&amp; grep -q '^version: "2"' .golangci.yml &amp;&amp; grep -q 'default: standard' .golangci.yml &amp;&amp; grep -Eq '^[[:space:]]+- gocritic' .golangci.yml &amp;&amp; grep -Eq '^[[:space:]]+- misspell' .golangci.yml &amp;&amp; grep -Eq '^[[:space:]]+- revive' .golangci.yml</automated>
  </verify>
  <done>
    `.golangci.yml` exists at repo root with v2 schema, `standard` preset, three extra
    linters enabled (gocritic / misspell / revive), and no truncation caps on issue reporting.
  </done>
</task>

<task type="auto">
  <name>Task 2: Create .github/workflows/ci.yml</name>
  <files>.github/workflows/ci.yml</files>
  <action>
    Create the directory and file `.github/workflows/ci.yml` implementing the single-job CI
    pipeline specified in RESEARCH.md §2. Required shape:

    - `name: CI`.
    - `on:` trigger on `push.branches: [main]` and `pull_request`.
    - `permissions: contents: read` (least privilege; workflow uses no secrets).
    - `concurrency:` group `ci-${{ github.ref }}` with `cancel-in-progress: true`.
    - One job named `ci` on `runs-on: ubuntu-latest` with `timeout-minutes: 10`.

    Steps, in order:
      1. `actions/checkout@v5`.
      2. `actions/setup-go@v6` with `go-version-file: go.mod` and `cache: true`.
      3. `go mod download`.
      4. Tidy check: run `go mod tidy` then `git diff --exit-code -- go.mod go.sum` in the
         same step so a dirty diff fails the build.
      5. `go build ./...`.
      6. `go vet ./...`.
      7. `golangci/golangci-lint-action@v9` with `version: v2.12` (no extra args — it reads
         `.golangci.yml`).
      8. `go test -race -count=1 ./...` — default tags only, so `//go:build integration`
         tests stay excluded per the user's locked decision and the researcher's
         recommendation.

    Constraints:
    - Do NOT add `-tags=integration` anywhere. Live-Telegram tests stay local-only.
    - Do NOT add Docker build / push, release steps, or matrix builds (OS or Go version).
    - Do NOT reference any GitHub Secrets.
    - Do NOT add YAML comments — CLAUDE.md no-comment convention applies.
    - Pin actions to major tags as listed in the interfaces block; no floating refs.
  </action>
  <verify>
    <automated>test -f .github/workflows/ci.yml &amp;&amp; grep -q '^name: CI' .github/workflows/ci.yml &amp;&amp; grep -q 'go-version-file: go.mod' .github/workflows/ci.yml &amp;&amp; grep -q 'golangci/golangci-lint-action@v9' .github/workflows/ci.yml &amp;&amp; grep -q 'version: v2.12' .github/workflows/ci.yml &amp;&amp; grep -q 'go test -race -count=1 ./\.\.\.' .github/workflows/ci.yml &amp;&amp; grep -q 'git diff --exit-code' .github/workflows/ci.yml &amp;&amp; ! grep -q 'tags=integration' .github/workflows/ci.yml &amp;&amp; ! grep -q 'docker' .github/workflows/ci.yml</automated>
  </verify>
  <done>
    Workflow file exists, triggers on push-to-main and pull_request, uses the pinned action
    versions, runs build / tidy-check / vet / lint / race-tests in a single ubuntu-latest job
    under 10 min, contains no integration tag, no Docker steps, and no secret references.
  </done>
</task>

<task type="auto">
  <name>Task 3: Local sanity check — build, vet, test</name>
  <files></files>
  <action>
    Without pushing, locally reproduce what CI will run to confirm the workflow won't
    immediately fail on `main`:

    1. `go mod tidy` and confirm `git status --short -- go.mod go.sum` is empty.
    2. `go build ./...` — must succeed.
    3. `go vet ./...` — must succeed.
    4. `go test -race -count=1 ./...` — must succeed across the four default-tag test
       packages (commands, handler, session, storage).

    If `golangci-lint` is installed locally, also run `golangci-lint run ./...` and confirm
    it exits 0 with the new `.golangci.yml`. If it reports failures, do NOT silently fix code
    in this quick task — instead, stop and surface the failures to the user so they can
    decide whether to address them here or as a follow-up. The CI-only locked scope means
    code changes to satisfy new lints are out of scope unless explicitly approved.

    If `golangci-lint` is not installed locally, skip the local lint check; CI will run it
    on the next push and the first failing run can be triaged then.
  </action>
  <verify>
    <automated>go mod tidy &amp;&amp; git diff --exit-code -- go.mod go.sum &amp;&amp; go build ./... &amp;&amp; go vet ./... &amp;&amp; go test -race -count=1 ./...</automated>
  </verify>
  <done>
    All four commands pass locally; go.mod / go.sum are tidy. The exact pipeline CI will
    execute (minus golangci-lint, which CI installs itself) has been validated on the local
    workstation. Any lint failures from an optional local `golangci-lint run` have been
    surfaced rather than silently patched.
  </done>
</task>

</tasks>

<verification>
End-to-end phase checks:

1. `ls .github/workflows/ci.yml .golangci.yml` — both files exist.
2. `go test -race -count=1 ./...` passes (same command CI will run).
3. `git diff --exit-code -- go.mod go.sum` is clean (CI's tidy gate will pass).
4. Workflow file uses only major-tag-pinned actions (`@v5`, `@v6`, `@v9`); no `@main` /
   `@master` / SHA refs.
5. Workflow file references no secrets and no Docker / release steps (locked scope).

Post-merge confirmation (manual, after this plan ships):
- First push to `main` (or first PR) triggers a green run under 5 minutes.
- A deliberately untidy `go.mod` change is rejected by the tidy step (one-off smoke test;
  not required as part of this plan).
</verification>

<success_criteria>
- `.github/workflows/ci.yml` and `.golangci.yml` exist and pass all gates in `<verification>`.
- Local `go build ./...`, `go vet ./...`, and `go test -race -count=1 ./...` succeed on the
  current `main`.
- Workflow honors locked scope: no Docker push, no releases, no matrix, no `-tags=integration`,
  no GitHub Secrets used.
- Action versions pinned per RESEARCH.md §4.
</success_criteria>

<output>
Create `.planning/quick/260518-jtt-add-support-for-github-actions/260518-jtt-SUMMARY.md` when done,
following the standard execute-plan summary template.
</output>
