---
name: cmc-cli
description: Use when working with the CoinMarketCap CLI, choosing shipped commands, or answering how to use cmc in scripts, TUI flows, or agent workflows.
version: 0.1.0
metadata:
  requires:
    bins:
      - cmc
  install: |
    go install github.com/openCMC/CoinMarketCap-CLI@latest
    mv "$(go env GOPATH)/bin/coinmarketcap-cli" "$(go env GOPATH)/bin/cmc"
---

# CMC CLI

## Overview

`cmc` is the CoinMarketCap-native CLI. Prefer commands that answer a user intent in one pass and keep stdout compact for scripts and agents.

## Prerequisites

This skill requires `cmc` installed and on PATH — see [CoinMarketCap CLI](https://github.com/openCMC/CoinMarketCap-CLI) for installation options.

An API key is optional. Without one, `cmc` uses the Keyless Public API: `price`, `markets`, `metrics`, `resolve`, `search`, `monitor`, `dex pair`, `dex pairs` and the endpoints in `cmc api --list --keyless` work. `trending`, `top-gainers-losers`, `history`, `news`, `pairs` and `price --with-chain-stats` need a free key: set `CMC_API_KEY` or run `cmc auth`. `cmc commands` shows `requires_auth` for every command.

## Quick Reference

| Need | Use | Notes |
|---|---|---|
| Exact asset lookup | `resolve` | Prefer `--id`, `--slug`, or `--symbol` for deterministic identity. |
| Quote + enrichments | `price` | Use `--with-info` and `--with-chain-stats` when you need more context. |
| Fuzzy discovery | `search` | Use for name/symbol lookup or DEX address discovery. |
| Market scan | `markets`, `trending`, `top-gainers-losers` | Use table output when a human is reading. |
| Time series | `history` | Use `--interval 5m|hourly|daily` and `--days max` only where supported. |
| Global context | `metrics`, `news`, `pairs` | Good bundle commands; avoid splitting into smaller metrics unless necessary. |
| Live polling | `monitor` | Polling only, not websocket streaming. |
| DEX pairs | `dex pair`, `dex pairs` | One pair by contract address, or a DEX's pairs on a network (`--dex-slug` required). |
| Anything else in the API | `api` | Any spec endpoint by path or operationId, response passed through unchanged. Find endpoints with `cmc api --list [filter]`. |
| Interactive views | `tui` | Human inspection only; not for scripting. |

## Output Rules

- Default output is compact JSON.
- Use `-o table` for readable terminal output.
- Use `--dry-run` to inspect request shape without calling the API.
- A key-only command run without a key fails with `api_key_required`; suggest `cmc auth` rather than retrying.
- Keep identity flags explicit when determinism matters.

## Slash Command Note

This skill is guidance for using the `cmc` runtime. It does not create a built-in `/cmc` slash command by itself.

- Hosts may surface it as a skill, reusable workflow, or contextual suggestion.
- A `/cmc` command only exists if the host explicitly adds a slash-command wrapper or alias for this skill.
- For hosts such as Claude Code or OpenClaw, do not assume skill registration alone will produce `/cmc`.

## Workflow

1. Use `resolve` when the user already knows the asset.
2. Use `search` when the user knows a name, symbol, or contract but not the exact identity.
3. Use `price` for quotes, then add `--with-info` or `--with-chain-stats` only when the bundle is needed.
4. Use `markets`, `trending`, `top-gainers-losers`, `metrics`, `news`, or `pairs` for broader market context.
5. Use `api` when no shipped command covers the data (Fear & Greed, CMC20/CMC100, RWA, derivatives, CMC AI, DEX holders and K-lines).
6. Use `tui` only when a human wants an interactive terminal view.

## Common Mistakes

- Do not use `search` as an exact-lookup replacement when `resolve` is available.
- Do not send scripting workloads to `tui`.
- Do not split a bundle into smaller commands unless the user asked for the narrow view.
- Do not assume every numeric or token-like string should map to a symbol; prefer the shipped command heuristics.
