# bcr — Bare Code Reviewer

`bcr` is a lightweight CLI that runs parallel AI code reviews over a git diff using
multiple LLMs through a single direct HTTP API. It has no agent subprocess layer, no
tool loops, and no skill execution — every model gets the same fixed review prompt and
returns findings in the same JSON schema. That keeps models interchangeable and reviews
cheap and predictable.

`bcr` is the lean, direct-API successor to [ACR (Agentic Code Reviewer)](https://github.com/richhaase/agentic-code-reviewer).

## Overview

- **Parallel reviewer fan-out** — spawns N concurrent model calls and combines their findings.
- **Post-processing pipeline** — exact-match dedup → regex exclusions → LLM synthesis
  (cluster + false-positive filter) → consolidated report.
- **Direct API** — talks to the provider's `/chat/completions` endpoint. No heavy CLI
  harness, no rate-limit surprises from tool loops, and per-model instructions are isolated.
- **GitHub integration** — post reviews to pull requests, or watch a PR until it goes green.
- **Cost transparency** — reports token usage and estimated cost after every run.

## Requirements

- [Go 1.26+](https://go.dev/dl/) to build from source, or a prebuilt binary.
- **One configured LLM provider** (see [Provider setup](#provider-setup)). OpenRouter and
  OpenAI-compatible endpoints are supported.
- `gh` CLI, authenticated, for GitHub PR review/watch features.

## Installation

### From source

```bash
go install github.com/richhaase/bcr/cmd/bcr@latest
```

### From releases

Download the latest binary from [Releases](https://github.com/richhaase/bcr/releases).

### Using make

```bash
git clone https://github.com/richhaase/bcr.git
cd bcr
make install
```

## Provider setup

The provider is selected by the base URL. Your API key is read from the matching standard
environment variable for that provider:

| Provider | Base URL | API key env var |
| --- | --- | --- |
| OpenRouter (default) | `https://openrouter.ai/api/v1` | `OPENROUTER_API_KEY` |
| OpenAI | `https://api.openai.com/v1` | `OPENAI_API_KEY` |
| Anthropic | `https://api.anthropic.com/v1` | `ANTHROPIC_API_KEY` |
| Local (Ollama / vLLM / LM Studio) | `http://localhost:11434/v1` | none |

The API key never appears in config files; it is resolved from the environment based on the
active base URL. Override the base URL with `BCR_BASE_URL`, in `.bcr.yaml`, or with
`bcr config init`.

## Quick start

```bash
# baseline review against the default base ref
bcr review

# review a specific diff range and model
bcr review -b develop -r deepseek/deepseek-chat,qwen/qwen-2.5-coder-32b-instruct

# review GitHub pull request #123
bcr review -p 123

# review PR #123 and submit the result without prompting
bcr review -p 123 -y

# watch PR #123 until it is clean
bcr watch -p 123
```

## Commands

| Command | Description |
| --- | --- |
| `bcr review` | Run a code review on a git diff (local or a PR). |
| `bcr watch` | Poll and re-review a PR until it goes green or a safety bound is hit. |
| `bcr config` | Create / inspect configuration. |
| `bcr desk` | Inspect and manage the local persistent review history. |
| `bcr version` | Print version information. |

### `bcr review`

| Flag | Short | Description |
| --- | --- | --- |
| `--reviewers` | `-r` | Comma-separated reviewer model ids |
| `--summarizer` | `-s` | Summarizer model |
| `--base` | `-b` | Git base ref for the diff |
| `--pr` | `-p` | GitHub PR number to review |
| `--concurrency` | `-c` | Max parallel reviewer calls (0 = unbounded) |
| `--retries` | `-t` | Retries per call on transient failures |
| `--exclude` | | Repeatable regex exclude patterns |
| `--guidance` | | Inline review guidance |
| `--guidance-file` | | Markdown file with review guidance |
| `--no-pr-feedback` | | Disable PR discussion context |
| `--yes` | `-y` | Auto-submit the PR review without prompting |

### `bcr watch`

Re-reviews a PR as new commits arrive (after a quiet period) and stops when the PR goes
green (LGTM) or a safety bound is hit.

```bash
bcr watch -p 123 --post-mode approve
```

| Flag | Default | Description |
| --- | --- | --- |
| `--post-mode` | `comment` | `comment` or `approve` |
| `--poll-interval` | `1m` | How often to poll for changes |
| `--settle-time` | `10m` | Quiet period after new commits before re-review |
| `--max-reviews` | `15` | Max reviews before giving up (safety bound) |
| `--max-duration` | `12h` | Max watch duration (safety bound) |

### `bcr config`

```bash
bcr config init          # create a .bcr.yaml in the current directory
bcr config init -g        # create the global config file (~/.config/bcr/config.yaml)
bcr config show           # show the effective resolved configuration
```

### `bcr desk`

Stores an append-only history of review runs per PR under the XDG data directory.

```bash
bcr desk history owner/repo#123   # show chronological review history + token/cost
bcr desk forget owner/repo#123    # permanently delete a PR's stored history
```

## Configuration

Create a `.bcr.yaml` in the repository root (or a global config with `bcr config init -g`):

```yaml
base_url: "https://openrouter.ai/api/v1"

models:
  - "deepseek/deepseek-chat"
  - "qwen/qwen-2.5-coder-32b-instruct"

summarizer_model: "anthropic/claude-sonnet-4-5"

base: "main"
temperature: 0.2
concurrency: 0
retries: 3
pr_feedback: true

exclude:
  - "generated/(.*)"

guidance: "Follow the project style guide."
guidance_file: "docs/review.md"
```

Precedence (highest to lowest):
1. CLI flags
2. Environment variables
3. `.bcr.yaml` file (local, then global)
4. Built-in defaults

### Environment variables

| Variable | Description |
| --- | --- |
| `OPENROUTER_API_KEY` / `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` | Provider API key (chosen by base URL) |
| `BCR_BASE_URL` | Provider base URL override |
| `BCR_MODELS` | Comma-separated reviewer models |
| `BCR_SUMMARIZER_MODEL` | Summarizer model |
| `BCR_BASE` | Git base ref |
| `BCR_CONCURRENCY` | Max concurrent reviewer calls |
| `BCR_RETRIES` | Retries on transient failures |
| `BCR_PR_FEEDBACK` | Enable PR discussion context (`true`/`false`) |
| `BCR_EXCLUDE` | Comma-separated regex exclude patterns |
| `BCR_GUIDANCE` | Inline review guidance |
| `BCR_GUIDANCE_FILE` | Path to a guidance markdown file |
| `XDG_DATA_HOME` | Controls where `bcr desk` history is stored |

## Development

```bash
# list all targets
make help

# build the binary
make build

# run tests
make test

# run all quality checks (fmt-check, vet, lint, tests) — non-mutating
make check

# scan dependencies for known vulnerabilities
make vuln

# update dependencies
make deps-update
```

## License

MIT License — see [LICENSE](LICENSE) for details.