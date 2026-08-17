# Implementation Plan: Issue #1 — GitHub PR Review Posting

## Source of truth

The acceptance contract is GitHub issue #1 in this repository
(`richhaase/bcr`). Implement exactly the `In Scope` acceptance criteria AC1–AC8.
Do not expand into the `Out of Scope` items. Preserve the listed invariants.

## Working rules for this worker

- Repository: `/Users/rdh/src/bcr` (Go CLI; `Makefile` present).
- Create and work on a branch: `git checkout -b feature/issue-001-pr-posting`.
- NO code comments anywhere (repo rule). Package doc comments are allowed.
- Run `make fmt && make check` until it is clean before committing. The build,
  vet, golangci-lint, and `go test -race ./...` must pass.
- Commit your work on the feature branch. Do not merge to `main`.

## Architecture to follow

The project layout is:

- `cmd/bcr/main.go` — entrypoint
- `internal/cli/` — Cobra commands (`review.go`, `config.go`, `root.go`, `version.go`)
- `internal/domain/` — pure types (`finding.go`, `dedup.go`, `parse.go`)
- `internal/pipeline/pipeline.go` — fan-out reviewers, dedup, synthesize; returns `*domain.ReviewRun`
- `internal/diff/git.go` — git/`gh` subprocess diff helpers (follow this pattern for other `gh`
  subprocess calls)
- `internal/config/config.go` — configuration

`domain.ReviewRun` fields (see `internal/domain/finding.go`):
`Diff string`, `Findings []Finding`, `Final []FinalFinding`, `Dismissed int`, `Models []string`.
`FinalFinding` carries `Keep bool`, `Rule`, `Severity`, `File`, `Line`, `Message`,
`Suggestion`, `Confidence`, `Count`, `Agents`.

The current `internal/cli/review.go` builds the diff (local ACR-style `git diff`, or `-p N` via
`internal/diff.PRDiff`), runs the pipeline, and calls an existing `renderReport(...)` that prints
to the terminal. Keep that pipeline/report as the `Invariants to Preserve` require.

## Required changes

### 1. New package `internal/github/pr.go`

Subprocess helpers via `gh` (mirror the style of `internal/diff/git.go`; use `os/exec` with
`CommandContext`, capture stdout/stderr, wrap errors):

- `CurrentUser(ctx context.Context) (string, error)`
  - `gh api user --jq .login`
- `RepoName(ctx context.Context) (string, error)`
  - `gh repo view --json nameWithOwner --jq .nameWithOwner` (gives `owner/repo`)
- `PRInfo(ctx context.Context, owner, repo string, number int) (PR, error)` with a `PRInfo`
  struct holding `Owner`, `Repo`, `Number`, `Author`, `URL`.
  - `gh pr view <number> --repo <owner/repo> --json author,url,number --jq '{author:.author.login,url:.url,number:.number}'`
- `CIState(ctx context.Context, owner, repo string, number int) (string, error)`
  - `gh pr checks <number> --repo <owner/repo>` — parse the `STATE` column; return `"success"`,
    `"failure"`, `"pending"`, or `"unknown"` (plus the raw line count for logging).
- `SubmitReview(ctx context.Context, owner, repo string, number int, event, body string) error`
  - Validate `event` is one of `request-changes`, `comment`, `approve` before running
    `gh pr review <number> --repo <owner/repo> --<event> --body <body>`.

### 2. New package `internal/review/posting.go` (pure, unit-testable)
- `PRBody(run *domain.ReviewRun) string` — render a GitHub PR review body (Markdown) from the
  kept (`Keep == true`) `Final` findings: for each, a section with severity, file:line, message,
  and (when present) suggestion. If no kept findings, render an LGTM-style body.
- `Disposition(run *domain.ReviewRun, ciOK bool, selfReview bool) (event string, reason string)`
  - If the user is reviewing their own PR: only `comment` (or `request-changes`) is allowed;
    never `approve`. Return that as `event` with a reason.
  - Else if any kept findings: `request-changes`.
  - Else if CI is OK: `approve`.
  - Else (clean but CI not green): `comment` with a "waiting on CI" reason.

### 3. Wire into `internal/cli/review.go`
- Add an auto-submit flag `-y, --yes` to the `bcr review` command (short `-y`).
- Add a guard so posting only occurs when a GitHub PR is identified (i.e. `-p N` was passed).
  Local diff-only runs must not try to post (preserves `Invariants`).
- After `renderReport`, if a PR is identified and a PR target is present:
  1. Resolve `owner/repo` (from `github.RepoName`) and `PRInfo` (author).
  2. Compute `selfReview = (CurrentUser == PRInfo.Author)`.
  3. Compute CI result only when an approval is under consideration (mirror ACR: don't gate a
     comment/request-changes on CI).
  4. If `--yes` is set: call `Disposition(...)`, then `SubmitReview` with `PRBodyText`, and log
     the posted event.
  5. Else (interactive): use a simple prompt on `cmd.InOrStdin()` and `cmd.OutOrStdout()`:
     - If kept findings exist: prompt `Post review? [R]equest changes / [C]omment / [S]kip` (default R).
     - Else: prompt `Post LGTM? [A]pprove / [C]omment / [S]kip` (default A, but only allow A when
       CI is green and not self-review).
     - Map the choice to an event and call `SubmitReview`; `[S]kip` does nothing.
- Keep `renderReport` as the terminal output until posting.
- Implement `Invoke` failures clearly (AC8): if `gh` is unavailable/unauthged, fail the command
  with a clear error.

### 4. Tests (table-driven, no comments)
- `internal/domain`/`internal/review`: test `PRBodyText` for (a) zero kept findings, (b) kept set
  with all fields, (c) suggestion omitted when empty.
- Test `Disposition` across: self-review, findings present, clean+CI-ok, clean+CI-not-ok tables.
- `internal/github/pr.go`: parse `gh` subprocess output for `gh pr checks` state column (pure
  parse helper) and the `SubmitReview` event whitelist.

## Completion gates
- `make fmt` clean, `make check` clean (`fmt-check`, `vet`, `lint`, `go test -race ./...`).
- AC1–AC8 motivate the added behavior.
- No code comments in any new/changed file.
- Committed to `feature/issue-001-pr-review-posting`; nothing pushed.