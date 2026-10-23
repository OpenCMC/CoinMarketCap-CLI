package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/spf13/cobra"
)

var pairsCmd = &cobra.Command{
	Use:   "pairs <asset>",
	Short: "List market pairs for an asset",
	Long:  "Fetch market pairs for a cryptocurrency asset from CMC market-pairs/latest. Positional shorthand follows the same heuristic as price: numeric args are treated as IDs; lowercase tokens with length >= 5 or a hyphen try slug first, then symbol; shorter tokens try symbol first, then slug. On symbol ambiguity, shorthand auto-picks the highest-ranked candidate and emits a warning on stderr.",
	Example: `  cmc pairs btc
  cmc pairs btc --limit 50
  cmc pairs btc --category spot
  cmc pairs btc --category derivatives`,
	Args: cobra.ExactArgs(1),
	RunE: runPairs,
}

func init() {
	pairsCmd.Flags().Int("limit", 20, "Number of pairs to return (1-100)")
	pairsCmd.Flags().String("category", "all", "Pair category: all, spot, or derivatives")
	pairsCmd.Flags().String("convert", "USD", "Target currency")
	addOutputFlag(pairsCmd)
	addDryRunFlag(pairsCmd)
	rootCmd.AddCommand(pairsCmd)
}

type pairView struct {
	Pair      string  `json:"pair"`
	Exchange  string  `json:"exchange"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	Volume24h float64 `json:"volume_24h"`
	FeeType   string  `json:"fee_type"`
}

type pairsPositionalDryRunPlan struct {
	Command  string                      `json:"command"`
	Mode     string                      `json:"mode"`
	Inputs   []string                    `json:"inputs"`
	Category string                      `json:"category"`
	Convert  string                      `json:"convert"`
	Limit    int                         `json:"limit"`
	Steps    []pairsPositionalDryRunStep `json:"steps"`
}

type pairsPositionalDryRunStep struct {
	Stage     string       `json:"stage"`
	Condition string       `json:"condition,omitempty"`
	Token     string       `json:"token,omitempty"`
	Request   dryRunOutput `json:"request"`
}

func runPairs(cmd *cobra.Command, args []string) error {
	jsonOut := outputJSON(cmd)
	if !jsonOut {
		display.PrintBanner()
	}

	category, _ := cmd.Flags().GetString("category")
	limit, _ := cmd.Flags().GetInt("limit")
	convert, _ := cmd.Flags().GetString("convert")
	assetToken := strings.TrimSpace(args[0])

	category = strings.ToLower(strings.TrimSpace(category))
	convert = strings.ToUpper(strings.TrimSpace(convert))
	if convert == "" {
		return fmt.Errorf("--convert must not be empty")
	}
	if limit < 1 || limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100")
	}
	switch category {
	case "all", "spot", "derivatives":
	default:
		return fmt.Errorf("--category must be one of all, spot, or derivatives")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if isDryRun(cmd) {
		if isAllDigits(assetToken) {
			params := map[string]string{
				"id":       assetToken,
				"category": category,
				"limit":    fmt.Sprintf("%d", limit),
				"convert":  convert,
			}
			return printDryRun(cfg, "pairs", "/v1/cryptocurrency/market-pairs/latest", params, nil)
		}
		return printPairsPositionalDryRunPlan(cfg, assetToken, category, convert, limit)
	}

	client := newAPIClient(cfg)
	ctx := cmd.Context()
	asset, err := resolvePairsAssetToken(ctx, client, assetToken)
	if err != nil {
		return err
	}

	pairs, err := client.MarketPairsLatest(ctx, api.MarketPairsLatestRequest{
		ID:       fmt.Sprintf("%d", asset.ID),
		Category: category,
		Limit:    limit,
		Convert:  convert,
	})
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("no pairs found")
	}

	views := make([]pairView, 0, len(pairs))
	for _, pair := range pairs {
		quote, ok := pair.QuoteFor(convert)
		if !ok {
			return fmt.Errorf("requested convert %q is not available for pair %s", convert, pair.PairLabel())
		}
		views = append(views, pairView{
			Pair:      pair.PairLabel(),
			Exchange:  pair.ExchangeLabel(),
			Category:  pair.Category,
			Price:     quote.Price,
			Volume24h: quote.Volume24h,
			FeeType:   pair.FeeType,
		})
	}

	if jsonOut {
		return printJSONRaw(views)
	}

	headers := []string{"Pair", "Exchange", "Category", "Price", "Volume 24h", "Fee Type"}
	rows := make([][]string, 0, len(views))
	for _, view := range views {
		rows = append(rows, []string{
			display.SanitizeCell(view.Pair),
			display.SanitizeCell(view.Exchange),
			display.SanitizeCell(view.Category),
			display.FormatPrice(view.Price, convert),
			display.FormatLargeNumber(view.Volume24h, convert),
			display.SanitizeCell(view.FeeType),
		})
	}
	display.PrintTable(headers, rows)
	return nil
}

func resolvePairsAssetToken(ctx context.Context, client *api.Client, token string) (api.ResolvedAsset, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return api.ResolvedAsset{}, api.ErrInvalidInput
	}
	if isAllDigits(token) {
		return client.ResolveByID(ctx, token)
	}
	if pairsPreferSlugFirst(token) {
		if asset, err := client.ResolveBySlug(ctx, strings.ToLower(token)); err == nil {
			return asset, nil
		} else if !errors.Is(err, api.ErrAssetNotFound) {
			return api.ResolvedAsset{}, err
		}
		return pairsLookupSymbol(ctx, client, token)
	}
	if asset, err := pairsLookupSymbol(ctx, client, token); err == nil {
		return asset, nil
	} else if !errors.Is(err, api.ErrAssetNotFound) {
		return api.ResolvedAsset{}, err
	}
	return client.ResolveBySlug(ctx, strings.ToLower(token))
}

func pairsLookupSymbol(ctx context.Context, client *api.Client, token string) (api.ResolvedAsset, error) {
	asset, err := client.ResolveBySymbol(ctx, strings.ToUpper(token))
	if err == nil {
		return asset, nil
	}
	var ambigErr *api.ResolverAmbiguityError
	if errors.As(err, &ambigErr) {
		if len(ambigErr.Candidates) > 0 {
			asset := ambigErr.Candidates[0]
			warnAutoPickedSymbol(token, asset)
			return asset, nil
		}
		return api.ResolvedAsset{}, fmt.Errorf("symbol %q is ambiguous; use --id or --slug instead (%w)", token, api.ErrResolverAmbiguous)
	}
	return api.ResolvedAsset{}, err
}

func pairsPreferSlugFirst(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if token != strings.ToLower(token) {
		return false
	}
	return len(token) >= 5 || strings.Contains(token, "-")
}

func buildPairsDryRunHeaders(cfg *config.Config) map[string]string {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{
		"Accept":     "application/json",
		"User-Agent": userAgent,
	}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}
	return headers
}

func newPairsDryRunRequest(cfg *config.Config, endpoint string, params map[string]string, note string, withMeta bool) dryRunOutput {
	out := dryRunOutput{
		Method:     "GET",
		URL:        cfg.BaseURL() + endpoint,
		Params:     params,
		Headers:    buildPairsDryRunHeaders(cfg),
		Note:       note,
		Pagination: nil,
	}
	if withMeta {
		out.OASSpec = spec.SourceURL
		out.OASOperationID = operationID(endpoint)
	}
	return out
}

func printPairsPositionalDryRunPlan(cfg *config.Config, token, category, convert string, limit int) error {
	steps := make([]pairsPositionalDryRunStep, 0, 3)
	if pairsPreferSlugFirst(token) {
		steps = append(steps, pairsPositionalDryRunStep{
			Stage:     "resolve_primary",
			Condition: "lowercase token with length >= 5 or hyphen prefers slug first",
			Token:     token,
			Request: newPairsDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
				"slug": strings.ToLower(token),
			}, "Primary shorthand resolution attempt.", false),
		})
		steps = append(steps, pairsPositionalDryRunStep{
			Stage:     "resolve_fallback",
			Condition: "only if slug lookup misses",
			Token:     token,
			Request: newPairsDryRunRequest(cfg, "/v1/cryptocurrency/map", map[string]string{
				"symbol": strings.ToUpper(token),
			}, "Fallback shorthand resolution attempt.", false),
		})
	} else {
		steps = append(steps, pairsPositionalDryRunStep{
			Stage:     "resolve_primary",
			Condition: "short token prefers symbol first",
			Token:     token,
			Request: newPairsDryRunRequest(cfg, "/v1/cryptocurrency/map", map[string]string{
				"symbol": strings.ToUpper(token),
			}, "Primary shorthand resolution attempt.", false),
		})
		steps = append(steps, pairsPositionalDryRunStep{
			Stage:     "resolve_fallback",
			Condition: "only if symbol lookup misses",
			Token:     token,
			Request: newPairsDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
				"slug": strings.ToLower(token),
			}, "Fallback shorthand resolution attempt.", false),
		})
	}
	steps = append(steps, pairsPositionalDryRunStep{
		Stage:     "fetch_pairs",
		Condition: "resolved asset ID is filled at runtime from the shorthand resolution steps",
		Request: newPairsDryRunRequest(cfg, "/v1/cryptocurrency/market-pairs/latest", map[string]string{
			"id":       "<resolved asset id>",
			"category": category,
			"limit":    fmt.Sprintf("%d", limit),
			"convert":  convert,
		}, "Final pair fetch after runtime shorthand resolution.", true),
	})

	return printJSONRaw(pairsPositionalDryRunPlan{
		Command:  "pairs",
		Mode:     "positional_shorthand",
		Inputs:   []string{token},
		Category: category,
		Convert:  convert,
		Limit:    limit,
		Steps:    steps,
	})
}
