# CLAUDE.md

## Project Overview

`cmc` is the CoinMarketCap-native CLI in this repository. The current focus is command correctness, compact JSON output, distribution hygiene, and preserving the CMC-specific command surface.

Recently shipped data commands include `metrics`, `news`, `pairs <asset>`, `price --with-info`, and `price --with-chain-stats`. Keep README examples and command docs aligned with those shipped behaviors.

Public release source is `openCMC/CoinMarketCap-CLI`. Do not reintroduce legacy personal forks or `coinmarketcap/*` as the primary release target in docs.

Repo-distributed skills live under `skills/`:

- `skills/cmc-cli/` is the core skill for shipped command selection and output contracts.
- `skills/claude-code-cmc-cli/` and `skills/openclaw-cmc-cli/` are thin adapters.

## Command Taxonomy Guidance

Future command work should be bundle-oriented, not metric-oriented. Prefer commands that answer a user intent in one pass over commands that expose a single isolated number.

- Majors should group spot, derivatives, and market regime context together when that bundle is relevant.
- Token-oriented commands should consider DEX context as part of the research surface instead of treating spot price as the whole answer.
- Protocol-oriented commands should emphasize protocol economics, not just price or market cap.
- `metrics`, `news`, `pairs`, and the shipped `price` enrichments `--with-info` / `--with-chain-stats` are intent-aligned examples; use them as the baseline before inventing new splits.
- Avoid adding one-off metrics unless they clearly belong to a user-facing bundle.

Use the taxonomy guidance above as the source of truth before inventing new command names or splits.

## Build & Test

```sh
make build
make test
make lint
make clean
```

Or directly:

```sh
go build -o cmc .
go test -race ./...
```

For source installs, clone `git@github.com:openCMC/CoinMarketCap-CLI.git` and use `go build ./...` or `go install ./...` from the repo root.

## Project Structure

```
coinmarketcap-cli/
├── main.go
├── cmd/                     # Cobra commands: auth, status, resolve, price, metrics, news, pairs, markets, history, trending, top-gainers-losers, monitor, dex, api, tui, version
├── internal/
│   ├── api/                 # CMC HTTP client, resolver, history helpers, tests
│   ├── config/              # Config loading and persistence
│   ├── display/             # Tables, formatting, banners, stderr warnings
│   ├── export/              # CSV export helpers
│   ├── spec/                # Operation index generated from the published OpenAPI spec (gen/ is the generator)
│   └── tui/                 # Bubble Tea TUI models and refresh logic
├── npm/                     # npm/pnpm release wrapper
├── skills/                  # repo-distributed skills for Claude Code and OpenClaw
├── Makefile
├── install.sh
└── README.md
```

## Conventions

- Go version is pinned in `go.mod`.
- Binary name is `cmc`.
- Config lives at `~/.config/cmc-cli/config.yaml`.
- Default output for data commands is compact JSON.
- `-o table` is for human-readable output.
- `cmc monitor` is polling, not WebSocket streaming.
- `cmc resolve` is the canonical asset lookup entry point.
- `cmc price --with-info` enriches quote output with crypto profile data, and `cmc price --with-chain-stats` adds blockchain statistics where available. They share the same base flags: `--id`, `--slug`, `--symbol`, `--convert`, `--with-info`, `--with-chain-stats`, `--dry-run`, `-o json|table`, and can be combined.
- `cmc metrics` reports global market metrics by convert currency. Flags: `--convert`, `--dry-run`, `-o json|table`.
- `cmc news` reports latest CMC news content with pagination and language/news-type filters. Flags: `--start`, `--limit`, `--language`, `--news-type`, `--dry-run`, `-o json|table`.
- `cmc pairs <asset>` reports market pairs for an asset with category filtering. Positional asset argument, flags: `--limit`, `--category all|spot|derivatives`, `--convert`, `--dry-run`, `-o json|table`.
- `CMC_API_KEY` takes priority over `--key`, and `--key` should warn about shell history exposure.
- With no key, requests go to the Keyless Public API (`/public-api`, no key header). Whether an endpoint is keyless comes from `x-keyless` in the spec, via `internal/spec`; never hard-code it. Key-only endpoints fail with `api.ErrKeyRequired` before any request.
- Every endpoint in `commandMeta` must be in the spec (`cmd/spec_contract_test.go`). Operation IDs and `requires_auth` in `cmc commands` are derived from the spec, not written by hand.
- `internal/spec/operations.json` is generated: run `make spec` after the published spec changes and commit the result. `CMC_SPEC_CHECK=1 go test ./internal/spec/gen/` checks it against the live spec.
- `cmc api` passes responses through byte for byte (compacted); do not decode and re-encode them.
- Avoid reintroducing legacy branding, comments, or endpoint assumptions.
- Keep code `gofmt`-clean and prefer the existing tests over new abstractions unless they are needed.
