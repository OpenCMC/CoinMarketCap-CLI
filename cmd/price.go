package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/spf13/cobra"
)

const maxExplicitSymbolMatches = 10

var priceCmd = &cobra.Command{
	Use:   "price",
	Short: "Get current price for coins",
	Long:  "Fetch current prices by CMC ID, slug, or symbol. Use --id for CMC IDs (e.g. 1,1027), --slug for slugs (e.g. bitcoin,ethereum), or --symbol for ticker symbols (e.g. BTC,ETH). Explicit --symbol returns up to 10 best-ranked matches per symbol. Positional shorthand: all-digit args are treated as CMC IDs; tokens with length >= 5 or a hyphen try slug first, then symbol, regardless of casing; shorter tokens try symbol first, then slug. On symbol ambiguity, shorthand auto-picks the highest-ranked candidate and emits a warning on stderr. Add --with-info to enrich quotes with CMC crypto profile details and --with-chain-stats to enrich quotes with blockchain statistics.",
	Example: `  cmc price --id 1,1027
  cmc price --slug bitcoin,ethereum
  cmc price --symbol BTC,ETH --convert EUR
  cmc price btc
  cmc price btc eth
  cmc price 1 1027
  cmc price --id 1 -o json`,
	RunE: runPrice,
}

func init() {
	priceCmd.Flags().String("id", "", "Comma-separated CMC IDs (e.g. 1,1027)")
	priceCmd.Flags().String("slug", "", "Comma-separated slugs (e.g. bitcoin,ethereum)")
	priceCmd.Flags().String("symbol", "", "Comma-separated symbols (e.g. BTC,ETH)")
	priceCmd.Flags().String("convert", "USD", "Target currency")
	priceCmd.Flags().Bool("with-info", false, "Enrich quote output with crypto profile info")
	priceCmd.Flags().Bool("with-chain-stats", false, "Enrich quote output with blockchain statistics")
	addOutputFlag(priceCmd)
	addDryRunFlag(priceCmd)
	priceCmd.Args = cobra.ArbitraryArgs
	rootCmd.AddCommand(priceCmd)
}

func runPrice(cmd *cobra.Command, args []string) error {
	idStr, _ := cmd.Flags().GetString("id")
	slugStr, _ := cmd.Flags().GetString("slug")
	symbolStr, _ := cmd.Flags().GetString("symbol")
	convert, _ := cmd.Flags().GetString("convert")
	withInfo, _ := cmd.Flags().GetBool("with-info")
	withChainStats, _ := cmd.Flags().GetBool("with-chain-stats")
	jsonOut := outputJSON(cmd)
	usePositionalShorthand := false

	if !jsonOut {
		display.PrintBanner()
	}

	if parsedKind, parsedValue, shorthand, err := parsePriceIdentityInput(args, idStr, slugStr, symbolStr); err != nil {
		return err
	} else {
		usePositionalShorthand = shorthand
		switch parsedKind {
		case "id":
			idStr = parsedValue
		case "slug":
			slugStr = parsedValue
		case "symbol":
			symbolStr = parsedValue
		default:
			return fmt.Errorf("provide --id, --slug, or --symbol")
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if withChainStats && cfg.IsKeyless() {
		return api.KeyRequired("`cmc price --with-chain-stats`")
	}

	// Short-circuit before any API calls in dry-run mode.
	if isDryRun(cmd) {
		if symbolStr != "" && !usePositionalShorthand {
			return printPriceExplicitSymbolDryRunPlan(cfg, splitTrim(symbolStr), convert, withInfo, withChainStats)
		}
		if usePositionalShorthand && symbolStr != "" {
			tokens := splitTrim(symbolStr)
			return printPricePositionalDryRunPlan(cfg, tokens, convert, withInfo, withChainStats)
		}
		params := map[string]string{
			"convert": convert,
		}
		if idStr != "" {
			params["id"] = idStr
		} else if slugStr != "" {
			params["slug"] = slugStr
		} else if symbolStr != "" {
			params["symbol"] = symbolStr
		}
		if !withInfo {
			if !withChainStats {
				return printDryRun(cfg, "price", "/v3/cryptocurrency/quotes/latest", params, nil)
			}
		}
		requests := []struct {
			opKey    string
			endpoint string
			params   map[string]string
		}{
			{opKey: "explicit", endpoint: "/v3/cryptocurrency/quotes/latest", params: params},
		}
		if withInfo {
			infoParams := map[string]string{}
			if idStr != "" {
				infoParams["id"] = idStr
			} else {
				infoParams["id"] = "<resolved asset ids>"
			}
			requests = append(requests, struct {
				opKey    string
				endpoint string
				params   map[string]string
			}{opKey: "with-info", endpoint: "/v2/cryptocurrency/info", params: infoParams})
		}
		if withChainStats {
			statsParams := map[string]string{}
			if idStr != "" {
				statsParams["id"] = idStr
			} else {
				statsParams["id"] = "<resolved asset ids>"
			}
			requests = append(requests, struct {
				opKey    string
				endpoint string
				params   map[string]string
			}{opKey: "with-chain-stats", endpoint: "/v1/blockchain/statistics/latest", params: statsParams})
		}
		return printDryRunMulti(cfg, "price", requests)
	}

	client := newAPIClient(cfg)
	ctx := cmd.Context()

	if symbolStr != "" && !usePositionalShorthand {
		return runPriceExplicitSymbolMode(ctx, cmd, client, splitTrim(symbolStr), convert, withInfo, withChainStats, jsonOut)
	}

	var quotes map[string]api.QuoteCoin
	var err2 error

	if idStr != "" {
		ids := splitTrim(idStr)
		quotes, err2 = client.QuotesLatestByID(ctx, ids, convert)
	} else if slugStr != "" {
		slugs := splitTrim(slugStr)
		quotes, err2 = client.QuotesLatestBySlug(ctx, slugs, convert)
	} else {
		// Positional shorthand resolves each token with a small heuristic:
		// short tokens stay symbol-first, while longer tokens and hyphenated tokens prefer slug-first.
		symbols := splitTrim(symbolStr)
		resolvedIDs := make([]string, 0, len(symbols))
		for _, token := range symbols {
			asset, err := resolvePriceShorthandToken(ctx, client, token, usePositionalShorthand)
			if err != nil {
				return err
			}
			resolvedIDs = append(resolvedIDs, fmt.Sprintf("%d", asset.ID))
		}
		quotes, err2 = client.QuotesLatestByID(ctx, resolvedIDs, convert)
	}

	if err2 != nil {
		return err2
	}

	if len(quotes) == 0 {
		return fmt.Errorf("no valid coins found")
	}

	var infoByID map[string]api.CoinInfo
	var statsByID map[string]api.BlockchainStatistics
	if withInfo {
		infoIDs := collectSortedQuoteIDs(quotes)
		infoByID, err2 = client.InfoByIDs(ctx, infoIDs)
		if err2 != nil {
			return err2
		}
	}
	if withChainStats {
		statsIDs := collectSortedQuoteIDs(quotes)
		statsByID, err2 = client.BlockchainStatisticsLatestByIDs(ctx, statsIDs)
		if err2 != nil {
			return err2
		}
	}

	if jsonOut {
		if !withInfo && !withChainStats {
			return printJSONRaw(quotes)
		}
		return printJSONRaw(mergePriceQuotesWithEnrichments(quotes, infoByID, statsByID))
	}

	// Sort by coin ID for deterministic table output.
	keys := make([]string, 0, len(quotes))
	for k := range quotes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headers := []string{"ID", "Name", "Symbol", "Price", "24h Change"}
	var rows [][]string
	for _, id := range keys {
		coin := quotes[id]
		quote, ok := coin.Quote[convert]
		if !ok {
			continue
		}
		rows = append(rows, []string{
			id,
			display.SanitizeCell(coin.Name),
			display.FormatSymbol(coin.Symbol),
			display.FormatPrice(quote.Price, convert),
			display.ColorPercent(quote.PercentChange24h),
		})
	}

	display.PrintTable(headers, rows)
	if withInfo {
		printPriceInfoSections(keys, quotes, infoByID)
	}
	if withChainStats {
		printPriceChainStatsSections(keys, quotes, statsByID)
	}
	return nil
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parsePriceIdentityInput(args []string, idStr, slugStr, symbolStr string) (string, string, bool, error) {
	if idStr != "" || slugStr != "" || symbolStr != "" {
		if len(args) > 0 {
			return "", "", false, fmt.Errorf("positional shorthand cannot be combined with --id, --slug, or --symbol")
		}
		if err := validateExactlyOneSelectorFamily(idStr, slugStr, symbolStr); err != nil {
			return "", "", false, err
		}
		switch {
		case idStr != "":
			return "id", idStr, false, nil
		case slugStr != "":
			return "slug", slugStr, false, nil
		default:
			return "symbol", symbolStr, false, nil
		}
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, splitTrim(arg)...)
	}
	if len(parts) == 0 {
		return "", "", false, nil
	}

	allDigits := true
	allSymbols := true
	for _, part := range parts {
		if isAllDigits(part) {
			allSymbols = false
			continue
		}
		allDigits = false
	}

	switch {
	case allDigits:
		return "id", strings.Join(parts, ","), true, nil
	case allSymbols:
		return "symbol", strings.Join(parts, ","), true, nil
	default:
		return "", "", false, fmt.Errorf("mixed positional shorthand is not allowed; use only numeric IDs or only symbols, or switch to --id/--slug/--symbol")
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func shouldPreferSlugFirst(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	return len(token) >= 5 || strings.Contains(token, "-")
}

type pricePositionalDryRunPlan struct {
	Command string                          `json:"command"`
	Mode    string                          `json:"mode"`
	Inputs  []string                        `json:"inputs"`
	Convert string                          `json:"convert"`
	Steps   []pricePositionalDryRunPlanStep `json:"steps"`
}

type pricePositionalDryRunPlanStep struct {
	Stage     string       `json:"stage"`
	Condition string       `json:"condition,omitempty"`
	Token     string       `json:"token,omitempty"`
	Request   dryRunOutput `json:"request"`
}

func buildDryRunHeaders(cfg *config.Config) map[string]string {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}
	headers["Accept"] = "application/json"
	headers["User-Agent"] = userAgent
	return headers
}

func newPriceDryRunRequest(cfg *config.Config, endpoint string, params map[string]string, note string, withMeta bool) dryRunOutput {
	out := dryRunOutput{
		Method:     "GET",
		URL:        cfg.BaseURL() + endpoint,
		Params:     params,
		Headers:    buildDryRunHeaders(cfg),
		Note:       note,
		Pagination: nil,
	}
	if withMeta {
		out.OASSpec = spec.SourceURL
		out.OASOperationID = operationID(endpoint)
	}
	return out
}

type priceQuoteWithInfo struct {
	api.QuoteCoin
	Info       *priceAssetInfo       `json:"info,omitempty"`
	ChainStats *priceAssetChainStats `json:"chain_stats,omitempty"`
}

type priceExplicitSymbolQuotes map[string][]priceQuoteWithInfo

type priceAssetInfo struct {
	Description string           `json:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	URLs        api.CoinInfoURLs `json:"urls,omitempty"`
}

type priceAssetChainStats struct {
	ID                  int64  `json:"id"`
	Slug                string `json:"slug"`
	Symbol              string `json:"symbol"`
	BlockRewardStatic   string `json:"block_reward_static"`
	ConsensusMechanism  string `json:"consensus_mechanism"`
	Difficulty          string `json:"difficulty"`
	Hashrate24h         string `json:"hashrate_24h"`
	PendingTransactions string `json:"pending_transactions"`
	ReductionRate       string `json:"reduction_rate"`
	TotalBlocks         string `json:"total_blocks"`
	TotalTransactions   string `json:"total_transactions"`
	TPS24h              string `json:"tps_24h"`
	FirstBlockTimestamp string `json:"first_block_timestamp"`
}

func mergePriceQuotesWithEnrichments(quotes map[string]api.QuoteCoin, infoByID map[string]api.CoinInfo, statsByID map[string]api.BlockchainStatistics) map[string]priceQuoteWithInfo {
	keys := make([]string, 0, len(quotes))
	for id := range quotes {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	out := make(map[string]priceQuoteWithInfo, len(keys))
	for _, id := range keys {
		coin := quotes[id]
		merged := priceQuoteWithInfo{QuoteCoin: coin}
		if info, ok := infoByID[id]; ok {
			merged.Info = &priceAssetInfo{
				Description: info.Description,
				Tags:        info.Tags,
				URLs:        info.URLs,
			}
		}
		if stats, ok := statsByID[id]; ok {
			merged.ChainStats = &priceAssetChainStats{
				ID:                  stats.ID,
				Slug:                stats.Slug,
				Symbol:              stats.Symbol,
				BlockRewardStatic:   stats.BlockRewardStatic,
				ConsensusMechanism:  stats.ConsensusMechanism,
				Difficulty:          stats.Difficulty,
				Hashrate24h:         stats.Hashrate24h,
				PendingTransactions: stats.PendingTransactions,
				ReductionRate:       stats.ReductionRate,
				TotalBlocks:         stats.TotalBlocks,
				TotalTransactions:   stats.TotalTransactions,
				TPS24h:              stats.TPS24h,
				FirstBlockTimestamp: stats.FirstBlockTimestamp,
			}
		}
		out[id] = merged
	}
	return out
}

func mergeExplicitSymbolQuotesWithEnrichments(
	symbols []string,
	candidates map[string][]api.ResolvedAsset,
	quotes map[string]api.QuoteCoin,
	infoByID map[string]api.CoinInfo,
	statsByID map[string]api.BlockchainStatistics,
) priceExplicitSymbolQuotes {
	out := make(priceExplicitSymbolQuotes, len(symbols))
	for _, symbol := range symbols {
		assets := candidates[symbol]
		enriched := make([]priceQuoteWithInfo, 0, len(assets))
		for _, asset := range assets {
			id := fmt.Sprintf("%d", asset.ID)
			coin, ok := quotes[id]
			if !ok {
				continue
			}
			merged := priceQuoteWithInfo{QuoteCoin: coin}
			if info, ok := infoByID[id]; ok {
				merged.Info = &priceAssetInfo{
					Description: info.Description,
					Tags:        info.Tags,
					URLs:        info.URLs,
				}
			}
			if stats, ok := statsByID[id]; ok {
				merged.ChainStats = &priceAssetChainStats{
					ID:                  stats.ID,
					Slug:                stats.Slug,
					Symbol:              stats.Symbol,
					BlockRewardStatic:   stats.BlockRewardStatic,
					ConsensusMechanism:  stats.ConsensusMechanism,
					Difficulty:          stats.Difficulty,
					Hashrate24h:         stats.Hashrate24h,
					PendingTransactions: stats.PendingTransactions,
					ReductionRate:       stats.ReductionRate,
					TotalBlocks:         stats.TotalBlocks,
					TotalTransactions:   stats.TotalTransactions,
					TPS24h:              stats.TPS24h,
					FirstBlockTimestamp: stats.FirstBlockTimestamp,
				}
			}
			enriched = append(enriched, merged)
		}
		out[symbol] = enriched
	}
	return out
}

func collectSortedQuoteIDs(quotes map[string]api.QuoteCoin) []string {
	keys := make([]string, 0, len(quotes))
	for id := range quotes {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	return keys
}

func printPriceInfoSections(ids []string, quotes map[string]api.QuoteCoin, infoByID map[string]api.CoinInfo) {
	for _, id := range ids {
		coin, ok := quotes[id]
		if !ok {
			continue
		}
		info, ok := infoByID[id]
		if !ok {
			continue
		}
		fmt.Printf("\n%s (%s)\n", display.SanitizeCell(coin.Name), id)
		fmt.Printf("Description: %s\n", display.SanitizeCell(info.Description))
		if len(info.Tags) > 0 {
			fmt.Printf("Tags: %s\n", strings.Join(info.Tags, ", "))
		}
		if len(info.URLs.Website) > 0 || len(info.URLs.TechnicalDoc) > 0 || len(info.URLs.Explorer) > 0 || len(info.URLs.Twitter) > 0 || len(info.URLs.Reddit) > 0 || len(info.URLs.MessageBoard) > 0 || len(info.URLs.Announcement) > 0 || len(info.URLs.Chat) > 0 || len(info.URLs.SourceCode) > 0 {
			fmt.Println("URLs:")
		}
		if len(info.URLs.Website) > 0 {
			fmt.Printf("  Website: %s\n", strings.Join(info.URLs.Website, ", "))
		}
		if len(info.URLs.TechnicalDoc) > 0 {
			fmt.Printf("  Technical doc: %s\n", strings.Join(info.URLs.TechnicalDoc, ", "))
		}
		if len(info.URLs.Explorer) > 0 {
			fmt.Printf("  Explorer: %s\n", strings.Join(info.URLs.Explorer, ", "))
		}
		if len(info.URLs.Twitter) > 0 {
			fmt.Printf("  Twitter: %s\n", strings.Join(info.URLs.Twitter, ", "))
		}
		if len(info.URLs.Reddit) > 0 {
			fmt.Printf("  Reddit: %s\n", strings.Join(info.URLs.Reddit, ", "))
		}
		if len(info.URLs.MessageBoard) > 0 {
			fmt.Printf("  Message board: %s\n", strings.Join(info.URLs.MessageBoard, ", "))
		}
		if len(info.URLs.Announcement) > 0 {
			fmt.Printf("  Announcement: %s\n", strings.Join(info.URLs.Announcement, ", "))
		}
		if len(info.URLs.Chat) > 0 {
			fmt.Printf("  Chat: %s\n", strings.Join(info.URLs.Chat, ", "))
		}
		if len(info.URLs.SourceCode) > 0 {
			fmt.Printf("  Source code: %s\n", strings.Join(info.URLs.SourceCode, ", "))
		}
	}
}

func printPriceChainStatsSections(ids []string, quotes map[string]api.QuoteCoin, statsByID map[string]api.BlockchainStatistics) {
	for _, id := range ids {
		coin, ok := quotes[id]
		if !ok {
			continue
		}
		stats, ok := statsByID[id]
		if !ok {
			continue
		}
		fmt.Printf("\n%s (%s)\n", display.SanitizeCell(coin.Name), id)
		fmt.Println("Chain stats:")
		fmt.Printf("  Symbol: %s\n", display.SanitizeCell(stats.Symbol))
		fmt.Printf("  Slug: %s\n", display.SanitizeCell(stats.Slug))
		fmt.Printf("  Consensus mechanism: %s\n", display.SanitizeCell(stats.ConsensusMechanism))
		fmt.Printf("  Block reward static: %s\n", display.SanitizeCell(stats.BlockRewardStatic))
		fmt.Printf("  Difficulty: %s\n", display.SanitizeCell(stats.Difficulty))
		fmt.Printf("  Hashrate 24h: %s\n", display.SanitizeCell(stats.Hashrate24h))
		fmt.Printf("  Pending transactions: %s\n", display.SanitizeCell(stats.PendingTransactions))
		fmt.Printf("  Reduction rate: %s\n", display.SanitizeCell(stats.ReductionRate))
		fmt.Printf("  Total blocks: %s\n", display.SanitizeCell(stats.TotalBlocks))
		fmt.Printf("  Total transactions: %s\n", display.SanitizeCell(stats.TotalTransactions))
		fmt.Printf("  TPS 24h: %s\n", display.SanitizeCell(stats.TPS24h))
		fmt.Printf("  First block timestamp: %s\n", display.SanitizeCell(stats.FirstBlockTimestamp))
	}
}

func printPricePositionalDryRunPlan(cfg *config.Config, tokens []string, convert string, withInfo, withChainStats bool) error {
	steps := make([]pricePositionalDryRunPlanStep, 0, len(tokens)*3+1)
	for _, token := range tokens {
		if shouldPreferSlugFirst(token) {
			steps = append(steps, pricePositionalDryRunPlanStep{
				Stage:     "resolve_primary",
				Condition: "tokens with length >= 5 or a hyphen prefer slug first, regardless of casing",
				Token:     token,
				Request: newPriceDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
					"slug": strings.ToLower(token),
				}, "Primary shorthand resolution attempt.", false),
			})
			steps = append(steps, pricePositionalDryRunPlanStep{
				Stage:     "resolve_fallback",
				Condition: "only if slug lookup misses",
				Token:     token,
				Request: newPriceDryRunRequest(cfg, "/v1/cryptocurrency/map", map[string]string{
					"symbol": strings.ToUpper(token),
				}, "Fallback shorthand resolution attempt.", false),
			})
			continue
		}
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "resolve_primary",
			Condition: "short token prefers symbol first",
			Token:     token,
			Request: newPriceDryRunRequest(cfg, "/v1/cryptocurrency/map", map[string]string{
				"symbol": strings.ToUpper(token),
			}, "Primary shorthand resolution attempt.", false),
		})
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "resolve_fallback",
			Condition: "only if symbol lookup misses",
			Token:     token,
			Request: newPriceDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
				"slug": strings.ToLower(token),
			}, "Fallback shorthand resolution attempt.", false),
		})
	}

	steps = append(steps, pricePositionalDryRunPlanStep{
		Stage:     "fetch_quotes",
		Condition: "resolved asset IDs are filled at runtime from the shorthand resolution steps",
		Request: newPriceDryRunRequest(cfg, "/v3/cryptocurrency/quotes/latest", map[string]string{
			"id":      "<resolved asset ids>",
			"convert": convert,
		}, "Final price fetch after runtime shorthand resolution.", true),
	})
	if withInfo {
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "fetch_info",
			Condition: "resolved asset IDs are reused after quotes fetch",
			Request: newPriceDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
				"id": "<resolved asset ids>",
			}, "Final info fetch after runtime shorthand resolution.", false),
		})
	}
	if withChainStats {
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "fetch_chain_stats",
			Condition: "resolved asset IDs are reused after quotes fetch",
			Request: newPriceDryRunRequest(cfg, "/v1/blockchain/statistics/latest", map[string]string{
				"id": "<resolved asset ids>",
			}, "Final chain-stats fetch after runtime shorthand resolution.", false),
		})
	}

	return printJSONRaw(pricePositionalDryRunPlan{
		Command: "price",
		Mode:    "positional_shorthand",
		Inputs:  tokens,
		Convert: convert,
		Steps:   steps,
	})
}

func printPriceExplicitSymbolDryRunPlan(cfg *config.Config, symbols []string, convert string, withInfo, withChainStats bool) error {
	steps := make([]pricePositionalDryRunPlanStep, 0, len(symbols)+3)
	for _, symbol := range symbols {
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "resolve_primary",
			Condition: "explicit --symbol resolves ranked candidates by symbol before quote fetch (up to 10 per symbol)",
			Token:     symbol,
			Request: newPriceDryRunRequest(cfg, "/v1/cryptocurrency/map", map[string]string{
				"symbol": strings.ToUpper(symbol),
			}, "Primary symbol resolution before quote fetch.", false),
		})
	}

	steps = append(steps, pricePositionalDryRunPlanStep{
		Stage:     "fetch_quotes",
		Condition: "resolved asset IDs are filled at runtime from the explicit symbol resolution step",
		Request: newPriceDryRunRequest(cfg, "/v3/cryptocurrency/quotes/latest", map[string]string{
			"id":      "<resolved asset ids>",
			"convert": convert,
		}, "Final price fetch after runtime symbol resolution.", true),
	})
	if withInfo {
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "fetch_info",
			Condition: "resolved asset IDs are reused after quotes fetch",
			Request: newPriceDryRunRequest(cfg, "/v2/cryptocurrency/info", map[string]string{
				"id": "<resolved asset ids>",
			}, "Final info fetch after runtime symbol resolution.", false),
		})
	}
	if withChainStats {
		steps = append(steps, pricePositionalDryRunPlanStep{
			Stage:     "fetch_chain_stats",
			Condition: "resolved asset IDs are reused after quotes fetch",
			Request: newPriceDryRunRequest(cfg, "/v1/blockchain/statistics/latest", map[string]string{
				"id": "<resolved asset ids>",
			}, "Final chain-stats fetch after runtime symbol resolution.", false),
		})
	}

	mode := "explicit_symbol"
	switch {
	case withInfo && withChainStats:
		mode = "explicit_symbol_with_info_and_chain_stats"
	case withInfo:
		mode = "explicit_symbol_with_info"
	case withChainStats:
		mode = "explicit_symbol_with_chain_stats"
	}
	return printJSONRaw(pricePositionalDryRunPlan{
		Command: "price",
		Mode:    mode,
		Inputs:  symbols,
		Convert: convert,
		Steps:   steps,
	})
}

func resolvePriceShorthandToken(ctx context.Context, client *api.Client, token string, positional bool) (api.ResolvedAsset, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return api.ResolvedAsset{}, api.ErrInvalidInput
	}

	lookupSymbol := func(value string) (api.ResolvedAsset, error) {
		asset, err := client.ResolveBySymbol(ctx, strings.ToUpper(value))
		if err == nil {
			return asset, nil
		}
		var ambigErr *api.ResolverAmbiguityError
		if errors.As(err, &ambigErr) {
			if positional && len(ambigErr.Candidates) > 0 {
				asset := ambigErr.Candidates[0]
				warnAutoPickedSymbol(value, asset)
				return asset, nil
			}
			return api.ResolvedAsset{}, fmt.Errorf("symbol %q is ambiguous; use --id or --slug instead (%w)", value, api.ErrResolverAmbiguous)
		}
		return api.ResolvedAsset{}, err
	}

	if !positional {
		return lookupSymbol(token)
	}

	if shouldPreferSlugFirst(token) {
		if asset, err := client.ResolveBySlug(ctx, strings.ToLower(token)); err == nil {
			return asset, nil
		} else if !errors.Is(err, api.ErrAssetNotFound) {
			return api.ResolvedAsset{}, err
		}
		return lookupSymbol(token)
	}

	if asset, err := lookupSymbol(token); err == nil {
		return asset, nil
	} else if !errors.Is(err, api.ErrAssetNotFound) {
		return api.ResolvedAsset{}, err
	}
	return client.ResolveBySlug(ctx, strings.ToLower(token))
}

func runPriceExplicitSymbolMode(
	ctx context.Context,
	cmd *cobra.Command,
	client *api.Client,
	symbols []string,
	convert string,
	withInfo bool,
	withChainStats bool,
	jsonOut bool,
) error {
	candidateMap := make(map[string][]api.ResolvedAsset, len(symbols))
	resolvedIDs := make([]string, 0, len(symbols)*maxExplicitSymbolMatches)
	seenIDs := make(map[string]struct{}, len(symbols)*maxExplicitSymbolMatches)

	for _, rawSymbol := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(rawSymbol))
		candidates, err := client.ResolveBySymbolCandidates(ctx, symbol, maxExplicitSymbolMatches)
		if err != nil {
			return err
		}
		candidateMap[symbol] = candidates
		for _, candidate := range candidates {
			id := fmt.Sprintf("%d", candidate.ID)
			if _, ok := seenIDs[id]; ok {
				continue
			}
			seenIDs[id] = struct{}{}
			resolvedIDs = append(resolvedIDs, id)
		}
	}

	quotes, err := client.QuotesLatestByID(ctx, resolvedIDs, convert)
	if err != nil {
		return err
	}

	var infoByID map[string]api.CoinInfo
	var statsByID map[string]api.BlockchainStatistics
	if withInfo {
		infoByID, err = client.InfoByIDs(ctx, resolvedIDs)
		if err != nil {
			return err
		}
	}
	if withChainStats {
		statsByID, err = client.BlockchainStatisticsLatestByIDs(ctx, resolvedIDs)
		if err != nil {
			return err
		}
	}

	grouped := mergeExplicitSymbolQuotesWithEnrichments(symbols, candidateMap, quotes, infoByID, statsByID)
	if jsonOut {
		return printJSONRaw(grouped)
	}

	headers := []string{"Query", "ID", "Name", "Slug", "Symbol", "Price", "24h Change"}
	rows := make([][]string, 0, len(resolvedIDs))
	displayCurrency := strings.ToLower(convert)
	seenPrinted := make(map[string]struct{}, len(resolvedIDs))

	for _, rawSymbol := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(rawSymbol))
		for _, coin := range grouped[symbol] {
			quote, ok := coin.Quote[convert]
			if !ok {
				continue
			}
			rows = append(rows, []string{
				symbol,
				fmt.Sprintf("%d", coin.ID),
				display.SanitizeCell(coin.Name),
				display.SanitizeCell(coin.Slug),
				display.FormatSymbol(coin.Symbol),
				display.FormatPrice(quote.Price, displayCurrency),
				display.ColorPercent(quote.PercentChange24h),
			})
			id := fmt.Sprintf("%d", coin.ID)
			if _, ok := seenPrinted[id]; ok {
				continue
			}
			seenPrinted[id] = struct{}{}
		}
	}

	display.PrintTable(headers, rows)
	if withInfo {
		printQuoteInfoSectionsFromGrouped(grouped)
	}
	if withChainStats {
		printQuoteChainStatsSectionsFromGrouped(grouped)
	}
	return nil
}

func printQuoteInfoSectionsFromGrouped(grouped priceExplicitSymbolQuotes) {
	seen := map[int64]struct{}{}
	keys := make([]string, 0, len(grouped))
	for symbol := range grouped {
		keys = append(keys, symbol)
	}
	sort.Strings(keys)
	for _, symbol := range keys {
		for _, coin := range grouped[symbol] {
			if coin.Info == nil {
				continue
			}
			if _, ok := seen[coin.ID]; ok {
				continue
			}
			seen[coin.ID] = struct{}{}
			fmt.Printf("\n%s (%d)\n", display.SanitizeCell(coin.Name), coin.ID)
			fmt.Printf("Description: %s\n", display.SanitizeCell(coin.Info.Description))
			if len(coin.Info.Tags) > 0 {
				fmt.Printf("Tags: %s\n", strings.Join(coin.Info.Tags, ", "))
			}
			printPriceInfoURLs(coin.Info.URLs)
		}
	}
}

func printPriceInfoURLs(urls api.CoinInfoURLs) {
	if len(urls.Website) > 0 || len(urls.TechnicalDoc) > 0 || len(urls.Explorer) > 0 || len(urls.Twitter) > 0 || len(urls.Reddit) > 0 || len(urls.MessageBoard) > 0 || len(urls.Announcement) > 0 || len(urls.Chat) > 0 || len(urls.SourceCode) > 0 {
		fmt.Println("URLs:")
	}
	if len(urls.Website) > 0 {
		fmt.Printf("  Website: %s\n", strings.Join(urls.Website, ", "))
	}
	if len(urls.TechnicalDoc) > 0 {
		fmt.Printf("  Technical doc: %s\n", strings.Join(urls.TechnicalDoc, ", "))
	}
	if len(urls.Explorer) > 0 {
		fmt.Printf("  Explorer: %s\n", strings.Join(urls.Explorer, ", "))
	}
	if len(urls.Twitter) > 0 {
		fmt.Printf("  Twitter: %s\n", strings.Join(urls.Twitter, ", "))
	}
	if len(urls.Reddit) > 0 {
		fmt.Printf("  Reddit: %s\n", strings.Join(urls.Reddit, ", "))
	}
	if len(urls.MessageBoard) > 0 {
		fmt.Printf("  Message board: %s\n", strings.Join(urls.MessageBoard, ", "))
	}
	if len(urls.Announcement) > 0 {
		fmt.Printf("  Announcement: %s\n", strings.Join(urls.Announcement, ", "))
	}
	if len(urls.Chat) > 0 {
		fmt.Printf("  Chat: %s\n", strings.Join(urls.Chat, ", "))
	}
	if len(urls.SourceCode) > 0 {
		fmt.Printf("  Source code: %s\n", strings.Join(urls.SourceCode, ", "))
	}
}

func printQuoteChainStatsSectionsFromGrouped(grouped priceExplicitSymbolQuotes) {
	seen := map[int64]struct{}{}
	keys := make([]string, 0, len(grouped))
	for symbol := range grouped {
		keys = append(keys, symbol)
	}
	sort.Strings(keys)
	for _, symbol := range keys {
		for _, coin := range grouped[symbol] {
			if coin.ChainStats == nil {
				continue
			}
			if _, ok := seen[coin.ID]; ok {
				continue
			}
			seen[coin.ID] = struct{}{}
			stats := coin.ChainStats
			fmt.Printf("\n%s (%d)\n", display.SanitizeCell(coin.Name), coin.ID)
			fmt.Println("Chain stats:")
			fmt.Printf("  Symbol: %s\n", display.SanitizeCell(stats.Symbol))
			fmt.Printf("  Slug: %s\n", display.SanitizeCell(stats.Slug))
			fmt.Printf("  Consensus mechanism: %s\n", display.SanitizeCell(stats.ConsensusMechanism))
			fmt.Printf("  Block reward static: %s\n", display.SanitizeCell(stats.BlockRewardStatic))
			fmt.Printf("  Difficulty: %s\n", display.SanitizeCell(stats.Difficulty))
			fmt.Printf("  Hashrate 24h: %s\n", display.SanitizeCell(stats.Hashrate24h))
			fmt.Printf("  Pending transactions: %s\n", display.SanitizeCell(stats.PendingTransactions))
			fmt.Printf("  Reduction rate: %s\n", display.SanitizeCell(stats.ReductionRate))
			fmt.Printf("  Total blocks: %s\n", display.SanitizeCell(stats.TotalBlocks))
			fmt.Printf("  Total transactions: %s\n", display.SanitizeCell(stats.TotalTransactions))
			fmt.Printf("  TPS 24h: %s\n", display.SanitizeCell(stats.TPS24h))
			fmt.Printf("  First block timestamp: %s\n", display.SanitizeCell(stats.FirstBlockTimestamp))
		}
	}
}
