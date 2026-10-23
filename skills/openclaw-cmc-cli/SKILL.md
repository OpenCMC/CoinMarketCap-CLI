---
name: openclaw-cmc-cli
description: Use when preparing OpenClaw prompts or workflows that should leverage the CMC CLI skill.
version: 0.1.0
metadata:
  requires:
    bins:
      - cmc
  install: |
    go install github.com/openCMC/CoinMarketCap-CLI@latest
    mv "$(go env GOPATH)/bin/coinmarketcap-cli" "$(go env GOPATH)/bin/cmc"
---

# OpenClaw + CMC CLI

Use this as a thin entrypoint for OpenClaw. Keep it aligned with the core [cmc-cli](../cmc-cli/SKILL.md) skill rather than duplicating command guidance.

## Prerequisites

This skill requires `cmc` installed and on PATH — see [CoinMarketCap CLI](https://github.com/openCMC/CoinMarketCap-CLI) for installation options.

An API key is optional. Without one, `cmc` uses the Keyless Public API: `price`, `markets`, `metrics`, `resolve`, `search`, `monitor`, `dex pair`, `dex pairs` and the endpoints in `cmc api --list --keyless` work. `trending`, `top-gainers-losers`, `history`, `news`, `pairs` and `price --with-chain-stats` need a free key: set `CMC_API_KEY` or run `cmc auth`. `cmc commands` shows `requires_auth` for every command.

## Guidance

- Frame requests in terms of the user intent, not the endpoint.
- Prefer shipped `cmc` commands over ad hoc API calls.
- Use compact JSON for automation and `-o table` for human review.
- Read [cmc-cli](../cmc-cli/SKILL.md) for the authoritative command map.
