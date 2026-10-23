package cmd

import (
	"strings"

	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// oasRepo points agents at the published spec the CLI's operation index is generated from.
const oasRepo = spec.SourceURL

// commandAnnotation lists the endpoints a command calls. Operation IDs and
// requires_auth are not hard-coded: they come from the embedded spec index.
type commandAnnotation struct {
	APIEndpoint  string
	APIEndpoints map[string]string
	Transport    string // "rest" (default) or "websocket"
	PaidOnly     bool
}

// endpoints returns every endpoint the command can call.
func (a commandAnnotation) endpoints() []string {
	out := make([]string, 0, len(a.APIEndpoints)+1)
	if a.APIEndpoint != "" {
		out = append(out, a.APIEndpoint)
	}
	for _, ep := range a.APIEndpoints {
		out = append(out, ep)
	}
	return out
}

// requiresAuth reports whether the command needs an API key: true unless at
// least one of its endpoints is served by the Keyless Public API. Modes that
// call a key-only endpoint (for example price --with-chain-stats) are stopped
// by the API client before any request is sent.
func (a commandAnnotation) requiresAuth() bool {
	for _, ep := range a.endpoints() {
		if spec.IsKeyless(ep) {
			return false
		}
	}
	return true
}

// operationID returns the spec operationId for an endpoint path, or "" if the
// spec does not document it.
func operationID(endpoint string) string {
	op, ok := spec.Lookup(endpoint)
	if !ok {
		return ""
	}
	return op.OperationID
}

func operationIDs(endpoints map[string]string) map[string]string {
	if len(endpoints) == 0 {
		return nil
	}
	out := make(map[string]string, len(endpoints))
	for key, ep := range endpoints {
		out[key] = operationID(ep)
	}
	return out
}

var commandMeta = map[string]commandAnnotation{
	"price": {
		APIEndpoint: "/v3/cryptocurrency/quotes/latest",
		APIEndpoints: map[string]string{
			"explicit":              "/v3/cryptocurrency/quotes/latest",
			"with-info":             "/v2/cryptocurrency/info",
			"with-chain-stats":      "/v1/blockchain/statistics/latest",
			"positional_slug_first": "/v2/cryptocurrency/info",
			"positional_symbol":     "/v1/cryptocurrency/map",
		},
	},
	"metrics": {
		APIEndpoint: "/v1/global-metrics/quotes/latest",
	},
	"markets": {
		APIEndpoint: "/v3/cryptocurrency/listings/latest",
		APIEndpoints: map[string]string{
			"default":         "/v3/cryptocurrency/listings/latest",
			"category_lookup": "/v1/cryptocurrency/categories",
			"category":        "/v1/cryptocurrency/category",
		},
	},
	"resolve": {
		APIEndpoints: map[string]string{
			"id":     "/v2/cryptocurrency/info",
			"slug":   "/v2/cryptocurrency/info",
			"symbol": "/v1/cryptocurrency/map",
		},
	},
	"trending": {
		APIEndpoint: "/v1/cryptocurrency/trending/latest",
		Transport:   "rest",
	},
	"news": {
		APIEndpoint: "/v1/content/latest",
	},
	"history": {
		APIEndpoints: map[string]string{
			"--date":             "/v1/cryptocurrency/quotes/historical",
			"--days":             "/v1/cryptocurrency/quotes/historical",
			"--date --ohlc":      "/v2/cryptocurrency/ohlcv/historical",
			"--days --ohlc":      "/v2/cryptocurrency/ohlcv/historical",
			"--from/--to":        "/v1/cryptocurrency/quotes/historical",
			"--from/--to --ohlc": "/v2/cryptocurrency/ohlcv/historical",
		},
	},
	"top-gainers-losers": {
		APIEndpoint: "/v1/cryptocurrency/trending/gainers-losers",
		Transport:   "rest",
	},
	"pairs": {
		APIEndpoint: "/v1/cryptocurrency/market-pairs/latest",
	},
	"monitor": {
		APIEndpoint: "/v3/cryptocurrency/quotes/latest",
		Transport:   "rest",
	},
	"search": {
		APIEndpoints: map[string]string{
			"asset":         "/v1/cryptocurrency/map",
			"address_pair":  "/v4/dex/pairs/quotes/latest",
			"address_token": "/v4/dex/spot-pairs/latest",
		},
		Transport: "rest",
	},
	"dex pairs": {
		APIEndpoint: "/v4/dex/spot-pairs/latest",
	},
	"dex pair": {
		APIEndpoint: "/v4/dex/pairs/quotes/latest",
	},
}

type flagInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// Flag enums are exposed in the command catalog for validation and docs.
var flagEnums = map[string]map[string][]string{
	"history": {
		"interval": {"5m", "hourly", "daily"},
	},
	"networks": {
		"sort-dir": {"asc", "desc"},
	},
	"top-gainers-losers": {
		"time-period": {"1h", "24h", "7d", "30d"},
	},
	"markets": {
		"sort":     {"market_cap", "price", "volume_24h"},
		"sort-dir": {"asc", "desc"},
	},
	"pairs": {
		"sort":     {"volume_24h", "liquidity", "no_of_transactions_24h"},
		"sort-dir": {"asc", "desc"},
	},
	"ohlcv": {
		"window": {"1h", "24h", "30d"},
	},
}

type commandInfo struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Flags           []flagInfo        `json:"flags"`
	Examples        []string          `json:"examples,omitempty"`
	OutputFormats   []string          `json:"output_formats"`
	RequiresAuth    bool              `json:"requires_auth"`
	PaidOnly        bool              `json:"paid_only"`
	Transport       string            `json:"transport,omitempty"`
	APIEndpoint     string            `json:"api_endpoint,omitempty"`
	APIEndpoints    map[string]string `json:"api_endpoints,omitempty"`
	OASOperationID  string            `json:"oas_operation_id,omitempty"`
	OASOperationIDs map[string]string `json:"oas_operation_ids,omitempty"`
}

type commandCatalog struct {
	Version  string        `json:"version"`
	OASRepo  string        `json:"oas_repo"`
	Commands []commandInfo `json:"commands"`
}

var commandsCmd = &cobra.Command{
	Use:    "commands",
	Short:  "Output machine-readable command catalog (JSON)",
	Hidden: true,
	RunE:   runCommands,
}

func init() {
	addOutputFlag(commandsCmd)
	rootCmd.AddCommand(commandsCmd)
}

func runCommands(cmd *cobra.Command, args []string) error {
	catalog := commandCatalog{
		Version: version,
		OASRepo: oasRepo,
	}

	for _, c := range rootCmd.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}

		// Skip non-data commands.
		if c.Name() == "auth" || c.Name() == "status" || c.Name() == "version" {
			info := commandInfo{
				Name:          c.Name(),
				Description:   c.Short,
				Flags:         extractFlags(c),
				Examples:      splitExamples(c.Example),
				OutputFormats: []string{},
			}
			if c.Name() == "status" {
				info.OutputFormats = []string{"json", "table"}
			}
			if c.Name() == "version" {
				info.OutputFormats = []string{"table", "json"}
			}
			catalog.Commands = append(catalog.Commands, info)
			continue
		}

		// Handle subcommands (tui markets, tui trending, dex subcommands).
		if c.HasSubCommands() {
			for _, sub := range c.Commands() {
				if sub.Hidden {
					continue
				}
				outputFormats := []string{"json", "table"}
				if c.Name() == "tui" {
					outputFormats = []string{"tui"}
				}
				if c.Name() == "dex" && sub.Name() == "pair" {
					outputFormats = []string{"json"}
				}
				info := commandInfo{
					Name:          c.Name() + " " + sub.Name(),
					Description:   sub.Short,
					Flags:         extractFlags(sub),
					Examples:      splitExamples(sub.Example),
					OutputFormats: outputFormats,
					RequiresAuth:  true,
				}
				if meta, ok := commandMeta[info.Name]; ok {
					applyCommandMeta(&info, meta)
				}
				catalog.Commands = append(catalog.Commands, info)
			}
			continue
		}

		info := commandInfo{
			Name:          c.Name(),
			Description:   c.Short,
			Flags:         extractFlags(c),
			Examples:      splitExamples(c.Example),
			OutputFormats: []string{"json", "table"},
			RequiresAuth:  true,
		}
		if c == apiCmd {
			// Depends on the endpoint; `cmc api --list --keyless` lists the keyless ones.
			info.RequiresAuth = false
		}

		if meta, ok := commandMeta[c.Name()]; ok {
			applyCommandMeta(&info, meta)
		}

		catalog.Commands = append(catalog.Commands, info)
	}

	return printJSONRaw(catalog)
}

func applyCommandMeta(info *commandInfo, meta commandAnnotation) {
	info.PaidOnly = meta.PaidOnly
	info.RequiresAuth = meta.requiresAuth()
	info.Transport = meta.Transport
	info.APIEndpoint = meta.APIEndpoint
	info.APIEndpoints = meta.APIEndpoints
	info.OASOperationID = operationID(meta.APIEndpoint)
	info.OASOperationIDs = operationIDs(meta.APIEndpoints)
}

func extractFlags(cmd *cobra.Command) []flagInfo {
	var flags []flagInfo
	cmd.NonInheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		fi := flagInfo{
			Name:        f.Name,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		}
		if enums, ok := flagEnums[cmd.Name()]; ok {
			if e, ok := enums[f.Name]; ok {
				fi.Enum = e
			}
		}
		flags = append(flags, fi)
	})
	return flags
}

func splitExamples(s string) []string {
	if s == "" {
		return nil
	}
	var examples []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimLeft(line, " ")
		if line != "" {
			examples = append(examples, line)
		}
	}
	return examples
}
