---
name: claude-code-cmc-cli
description: Use when preparing Claude Code prompts or workflows that should leverage the CMC CLI skill.
version: 0.1.0
metadata:
  requires:
    bins:
      - cmc
  install: |
    go install github.com/openCMC/CoinMarketCap-CLI@latest
    mv "$(go env GOPATH)/bin/coinmarketcap-cli" "$(go env GOPATH)/bin/cmc"
---

# Claude Code + CMC CLI

Use this as a thin entrypoint for Claude Code. Do not duplicate the CMC CLI reference; route command choice, flags, and output expectations to [cmc-cli](../cmc-cli/SKILL.md).

## Prerequisites

This skill requires `cmc` installed and on PATH — see [CoinMarketCap CLI](https://github.com/openCMC/CoinMarketCap-CLI) for installation options.

An API key is optional. Without one, `cmc` uses the Keyless Public API: `price`, `markets`, `metrics`, `resolve`, `search`, `monitor`, `dex pair`, `dex pairs` and the endpoints in `cmc api --list --keyless` work. `trending`, `top-gainers-losers`, `history`, `news`, `pairs` and `price --with-chain-stats` need a free key: set `CMC_API_KEY` or run `cmc auth`. `cmc commands` shows `requires_auth` for every command.

## Guidance

- Prefer short, explicit prompts that name the user intent.
- Prefer the smallest `cmc` command that answers the request.
- Prefer compact JSON for machine use and table output for human review.
- When unsure, read the core `cmc-cli` skill first.
