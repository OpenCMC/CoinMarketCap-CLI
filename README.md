# cmc - CoinMarketCap CLI

Research-grade, terminal-native CoinMarketCap access for people and agents that care about stable identifiers, machine-readable payloads, and reproducible workflows.

Use `cmc` when symbol ambiguity, shifting payload shapes, and browser-driven workflows are too fragile for your analysis or automation. Resolve to canonical CoinMarketCap IDs, keep stdout predictable, and move between inspection, scripting, and TUI without changing tools.

The repository also ships dedicated skills for Claude Code and OpenClaw, so the same `cmc` command surface can be used directly in terminal workflows or through agent-native tool prompts.

## Choose The Right Runtime

`cmc` is the best fit when your workflow lives in the terminal and you want repeatable, high-data-volume access with fewer moving parts. It keeps output stable, reduces manual HTTP plumbing, and gives agents a single command surface they can inspect, export, and script against.

| Runtime | Best fit | Why choose it |
|---|---|---|
| CMC CLI | Terminal-first analysts, power users, and agents | Stable JSON/table output, CSV export, `--dry-run`, TUI, and bundled Claude Code/OpenClaw skills for agent-native workflows |
| CMC Pro API | Apps and services that want direct HTTP integration | Lowest-level control when you are already building around the API and managing requests yourself |
| CMC MCP | MCP-capable assistants and IDEs | Convenient when the model should call CoinMarketCap tools inside an MCP runtime |

If you need repeatable inspection, bulk pulls, or agent workflows that stay close to the shell, the CLI is usually the most ergonomic choice. If you are embedding CoinMarketCap data into a product or an MCP-native assistant, the API or MCP layer may be a better runtime match.

## Try This First

No API key needed for these; they run on the Keyless Public API (see [Keyless mode](#keyless-mode)):

```sh
cmc price --id 1,1027
cmc api /v3/fear-and-greed/latest
```

These three show the core value quickly. All but `--with-chain-stats` also work without a key:

```sh
# Stable identity resolution
cmc resolve --id 1

# Quote plus chain-level enrichment in machine-readable form
cmc price --id 1 --with-info --with-chain-stats -o json | jq '.[] | .name, .symbol, .chain_stats'

# Chain-scoped discovery that is hard to do safely by hand
cmc search --chain ethereum --address 0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2
```

## Install

Homebrew and the shell installer are the primary paths.

```sh
# Homebrew
brew install openCMC/CoinMarketCap-CLI/cmc

# Shell installer (defaults to ~/.local/bin)
curl -sSfL https://raw.githubusercontent.com/openCMC/CoinMarketCap-CLI/main/install.sh | sh
```

If `cmc` is not on your `PATH` after the shell installer:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Build from source if you want the binary directly from the repository:

```sh
git clone git@github.com:openCMC/CoinMarketCap-CLI.git
cd CoinMarketCap-CLI
go build -o ./cmc .
./cmc version
```

If you prefer to install the binary into your Go bin directory instead:

```sh
go install .
mv "$(go env GOPATH)/bin/coinmarketcap-cli" "$(go env GOPATH)/bin/cmc"
```

`go install` names the binary after the module (`coinmarketcap-cli`), so the rename keeps it consistent with every other install method.

## Keyless mode

With no key configured (no config file and no `CMC_API_KEY`), `cmc` calls the [Keyless Public API](https://coinmarketcap.com/api/documentation/pro-api-reference/keyless-public-api) at `https://pro-api.coinmarketcap.com/public-api` and sends no key header. Output has the same fields and shape with or without a key, so scripts keep working when you add one later.

Works keyless: `price` (without `--with-chain-stats`), `markets`, `metrics`, `resolve`, `search`, `monitor`, `dex pair`, `dex pairs`, and every endpoint `cmc api --list --keyless` shows.

Needs a free key: `trending`, `top-gainers-losers`, `history`, `news`, `pairs`, and `price --with-chain-stats`. Without a key they stop before calling the API and tell you to run `cmc auth`; with `-o json` the error is `{"error":"api_key_required",...}` on stderr.

Keyless requests share an IP-based rate limit. `cmc` retries a `429` with exponential backoff (1s, 2s, 4s, 8s) and then suggests a key. `monitor` polls at most every 30s keyless. `cmc status` shows `"mode": "keyless"`. A key that is set but invalid fails with an auth error; it never falls back to keyless.

## Authenticate

A free key raises the rate limits and unlocks every endpoint. Get one at https://coinmarketcap.com/api/pricing/.

```sh
# Interactive prompt
cmc auth

# Non-interactive: provide key via env var (tier is advisory and read at runtime, not saved)
export CMC_API_KEY=your-key
cmc auth

# Env-only runtime auth: no config file needed
CMC_API_KEY=your-key cmc price --id 1 -o json

# Flag form works, but --key may leak via shell history or process listings
cmc auth --key your-key
```

Config is stored in your OS user config directory. Run `cmc status` to see the mode (`keyless` or `api_key`), active tier, masked key, base URL, and exact config path.

## Using cmc with AI agents

For small AI agents and scripted runs, a stable loop is:

1. Resolve once to an explicit ID, then reuse that ID downstream.
2. Use `-o json` for machine parsing and `--dry-run` to preview the upstream request before hitting the API.
3. Treat `cmc resolve` and `cmc history` as beta / higher-caution surfaces when they sit inside high-trust workflows.
4. Avoid shorthand for high-trust workflows. Prefer `cmc price --id 1` over `cmc price btc`, and prefer `cmc history --id 1` over `cmc history 1` when you need deterministic selection.
5. Reuse the bundled skills in [`skills/cmc-cli/SKILL.md`](skills/cmc-cli/SKILL.md), [`skills/claude-code-cmc-cli/SKILL.md`](skills/claude-code-cmc-cli/SKILL.md), and [`skills/openclaw-cmc-cli/SKILL.md`](skills/openclaw-cmc-cli/SKILL.md) when you want the same workflow exposed through Claude Code or OpenClaw.

```sh
cmc resolve --id 1
cmc price --id 1 --dry-run -o json
cmc history --id 1 --days 30 --dry-run -o json
```

## Core Workflows

### Resolve and quote with stable identity

```sh
cmc resolve --id 1
cmc resolve --id 1027
cmc price --id 1
cmc price --id 1,1027 --convert EUR
cmc price --id 1 --with-info -o json
cmc price --id 1 --with-chain-stats -o json
cmc price --id 1 --with-info --with-chain-stats -o json
```

### Search by name or chain-scoped address

```sh
cmc search bitcoin
cmc search usdc --limit 5
cmc search --chain ethereum --address 0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48
```

### Scan markets, metrics, and pairs

```sh
cmc metrics --convert EUR -o table
cmc markets --limit 20 -o table
cmc markets --category layer-2
cmc markets --sort volume_24h --sort-dir desc
cmc markets --total 250 --export top250.csv
cmc pairs 1
cmc pairs 1027 --category derivatives --limit 50 -o table
```

### Call any endpoint in the spec

`cmc api` calls any endpoint in the published [OpenAPI spec](https://coinmarketcap.com/api/documentation/openapi.json) by path or operationId and prints the response unchanged, so new fields show up without a CLI release. Required parameters are checked against the spec; unknown parameters and enum values only warn, so parameters added after a release still work.

```sh
cmc api --list -o table                 # every endpoint, with keyless and tag columns
cmc api --list --keyless -o table       # the ones that work without a key
cmc api --list real-world-assets        # filter by path, operationId or summary
cmc api /v3/fear-and-greed/latest
cmc api getV1AltcoinSeasonIndexHistorical -p timeframe=30d
cmc api /v3/index/cmc20-latest -o table
cmc api /v1/dex/holders/list --body '{"platform":"ethereum","tokenAddress":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","tag":"tag_all"}'
cmc api /v5/real-world-assets/map -p limit=5 --dry-run
```

### Look up DEX pairs

```sh
cmc dex pair --network ethereum --pair 0x11b815efb8f581194ae79006d24e0d814b7697f6
cmc dex pairs --network ethereum --dex-slug uniswap-v3 --limit 10 -o table
```

`--network` takes a slug (`ethereum`) or a numeric network id. `dex pairs` needs `--dex-slug` or `--dex-id`.

### Pull history, news, and trend data

```sh
cmc history --id 1 --days 30
cmc history --id 1 --date 2024-01-01
cmc news --limit 5 -o table
cmc news --language en --news-type news -o table
cmc top-gainers-losers --time-period 1h -o table
cmc trending
```

```sh
cmc history --id 1 --days 1 --interval 5m            # paid plan required
cmc history --id 1 --days 7 --ohlc --interval hourly # paid plan required
```

Hourly and 5-minute history intervals may require a paid CoinMarketCap plan. `--days max` is daily-only, and `--ohlc` is not supported with `--interval 5m`.

## Output Contract

Most data commands default to compact JSON. Use `-o table` when you want a human-readable table and `-o json` when you want to pipe or diff the payload.

```sh
cmc history --id 1 --days 30 | jq '.[].price'
cmc markets --limit 100 --export top100.csv
cmc price --id 1 --dry-run
```

## Automation And Auditability

- Resolve once, then use canonical IDs downstream to avoid symbol ambiguity in scripts and agents.
- Positional shorthand works in `price` and `pairs` (e.g. `cmc price btc`), but explicit IDs or slugs are safer when you need deterministic selection.
- Data command stdout stays machine-readable; diagnostics and warnings go to stderr.
- For automation, split them explicitly when you need clean artifacts:

```sh
cmc price --id 1 --dry-run -o json > price.json 2> price.log
```

- `--dry-run` is available on data commands to preview the exact upstream request without sending it.
- `markets --export` and `history --export` write CSV directly when a file artifact is more useful than JSON.

## Terminal UI

`cmc monitor` polls latest quotes on a fixed interval. It is polling, not streaming.

```sh
cmc monitor --id 1,1027 --interval 60s
cmc monitor --symbol BTC,ETH -o table
```

`cmc tui` is for interactive inspection, not scripting.

```sh
cmc tui
cmc tui markets
cmc tui trending
```

## Notes

- The CLI embeds an operation index generated from the published OpenAPI spec (`internal/spec/operations.json`). `cmc commands` reads operation IDs and `requires_auth` from it. Regenerate it with `make spec`; never edit it by hand.
- Supported tiers: `basic`, `hobbyist`, `startup`, `standard`, `professional`, `enterprise`.
- Some endpoints and intervals are tier-gated by CoinMarketCap plan, especially historical 5-minute and hourly data.
- `cmc version` shows build version and commit metadata.
