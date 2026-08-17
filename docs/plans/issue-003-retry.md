# Implementation Plan: Issue #3 — Retry and Fallback Resilience

## Source of truth

The acceptance contract is GitHub issue #3 in this repository (`richhaase/bcr`).
Implement exactly the `In Scope` acceptance criteria AC1–AC6. Do not expand
into the `Out of Scope` items. Preserve the listed invariants.

## Working rules for this worker

- Repository: `/Users/rdh/src/bcr` (Go CLI; `Makefile` present).
- Branch: `feature/issue-003-retry`.
- NO code comments anywhere (repo rule).
- Run `make fmt && make check` until it is clean before committing. Build, vet,
  golangci-lint, and `go test -race ./...` must pass.
- Commit work on the feature branch; do not merge to `main`.

## Architecture

Reviewer fan-out, dedup, and synthesize live in `internal/pipeline/pipeline.go`.
Both reviewer and synthesizer calls funnel through a single
`provider.Client.Complete(...)` (`internal/provider/client.go`). That single
call site is the natural place to add bounded retry with exponential backoff +
jitter so AC1/AC2/AC5 are satisfied for both reviewer and synthesizer calls.

`internal/config/config.go` holds settings; `internal/cli/review.go` wires config
into the pipeline. Retry budget becomes configurable there (AC5). AC4 is already
preserved: the pipeline appends findings only from reviewers with `err == nil`,
so permanently-failed reviewers drop their contribution while successful ones are
kept. AC6 (usage summed/reported) has no code today; it is preserved by not
changing reporting paths.

## Required changes

### 1. `internal/provider/client.go` — retry in `Complete`

- Add fields to `Client`: `MaxRetries int`, `BaseDelay time.Duration`,
  `MaxDelay time.Duration`.
- `NewClient` sets defaults: `MaxRetries = 3`, `BaseDelay = 200ms`,
  `MaxDelay = 4s`.
- New types: `statusError` (carries `StatusCode` and `Body`), sentinel
  `errEmptyChoices`.
- `completeOnce(...)` performs a single HTTP round-trip:
  - `3xx/4xx/5xx` status → return `&statusError{StatusCode, Body}`.
  - empty `Choices` → return `errEmptyChoices`.
  - decode / provider-api-error → return plain (permanent) errors.
- `Complete(...)` loops up to `MaxRetries+1` attempts:
  - success → return content.
  - `ctx.Err() != nil` → return the error without retrying.
  - permanent error (`!isRetryable`) → return immediately (AC3).
  - `attempt == MaxRetries` → return last error.
  - otherwise `wait(ctx, attempt+1)` then retry (AC1/AC2).
- `isRetryable(err)`:
  - `*statusError` with status 429 or >= 500 → retryable.
  - `errEmptyChoices` → retryable (empty output).
  - `net.Error` (timeouts / connection errors) → retryable.
  - all other 4xx and protocol errors → permanent.
- `wait(ctx, retry)`: exponential delay `BaseDelay << (retry-1)` capped at
  `MaxDelay`, with full jitter via `crypto/rand`; aborts on `ctx.Done()`.
  Using `crypto/rand` keeps `gosec` (G404) happy.

### 2. `internal/pipeline/pipeline.go`

- Add `Retries int` to `pipeline.Config`.
- `NewRunner` sets `client.MaxRetries = cfg.Retries`.

### 3. `internal/config/config.go`

- Add `Retries int` (`yaml:"retries"`).
- `DefaultConfig` sets `Retries: 3` (AC5 sensible default).
- Env override `BCR_RETRIES`.
- Add `retries: 3` to `DefaultTemplate`.

### 4. `internal/cli/config.go`

- Print `retries` in `config show`.

### 5. `internal/cli/review.go`

- Add flag `--retries` (short `-t`), pass `cfg.Retries` to `pipeline.Config`,
  override from flag when > 0.

### 6. Tests (table-driven, no comments)

- `internal/provider/client_test.go`: fake `http.RoundTripper` driving status
  codes; assert transient (429, 500, empty) retried up to `MaxRetries` and
  success on later attempt; assert 4xx permanent not retried.
  Directly unit-test `isRetryable` for each error class.
- `internal/config/config_test.go`: default `Retries == 3`; `BCR_RETRIES`
  override; empty env keeps default.
- `internal/pipeline/pipeline_test.go`: add a permanent-failure reviewer case to
  prove findings from other reviewers are preserved (AC4).

## Completion gates

- `make fmt` clean, `make check` clean (`fmt-check`, `vet`, `lint`,
  `go test -race ./...`).
- AC1–AC6 motivate the added behavior; Out-of-Scope items untouched.
- No code comments in any new/changed file.
- Committed to `feature/issue-003-retry`; nothing pushed.