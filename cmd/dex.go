package cmd

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"
	"github.com/spf13/cobra"
)

const (
	dexDefaultConvertID     = "2781"
	dexDefaultListingsLimit = 50
	dexDefaultSearchPages   = 3
	dexMaxSearchPages       = 10
	dexDefaultSearchLimit   = 20
	dexDefaultPairsLimit    = 50
	dexDefaultTrendingLimit = 50
	dexDefaultTradesLimit   = 50
	dexNetworkPageLimit     = 200
)

var dexNow = time.Now

var dexCmd = &cobra.Command{
	Use:   "dex",
	Short: "DEX spot pairs and single-pair snapshots",
	Long:  "Look up DEX spot pairs by network and DEX, or one pair by contract address. Both work without an API key.",
}

var dexNetworksCmd = &cobra.Command{
	Use:   "networks",
	Short: "List supported DEX networks",
	Example: `  cmc dex networks
  cmc dex networks --start 201 --limit 100
  cmc dex networks -o table`,
	RunE: runDEXNetworks,
}

var dexSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search DEX pairs locally over paged listings and spot pairs",
	Example: `  cmc dex search usdc
  cmc dex search uniswap --pages 2 --limit 10
  cmc dex search 0xabc123 -o table`,
	Args: cobra.ExactArgs(1),
	RunE: runDEXSearch,
}

var dexPairsCmd = &cobra.Command{
	Use:   "pairs",
	Short: "List spot pairs for a DEX on a network",
	Example: `  cmc dex pairs --network ethereum --dex-slug uniswap-v3
  cmc dex pairs --network 1 --dex-slug uniswap-v3 --token 0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48 --limit 25
  cmc dex pairs --network ethereum --dex-slug uniswap-v3 -o table`,
	RunE: runDEXPairs,
}

var dexPairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Get a single DEX pair snapshot",
	Example: `  cmc dex pair --network ethereum --pair 0x11b815efb8f581194ae79006d24e0d814b7697f6
  cmc dex pair --network 1 --pair 0x11b815efb8f581194ae79006d24e0d814b7697f6`,
	RunE: runDEXPair,
}

var dexOHLCVCmd = &cobra.Command{
	Use:   "ohlcv",
	Short: "Get historical OHLCV for a DEX pair",
	Example: `  cmc dex ohlcv --network ethereum --pair 0x11b8... --window 24h
  cmc dex ohlcv --network 1 --pair 0x11b8... --window 30d -o table`,
	RunE: runDEXOHLCV,
}

var dexTradesCmd = &cobra.Command{
	Use:   "trades",
	Short: "Get latest trades for a DEX pair",
	Example: `  cmc dex trades --network ethereum --pair 0x11b8...
  cmc dex trades --network 1 --pair 0x11b8... --limit 25 -o table`,
	RunE: runDEXTrades,
}

var dexTrendingCmd = &cobra.Command{
	Use:   "trending",
	Short: "List active DEX pairs on a network",
	Example: `  cmc dex trending --network ethereum
  cmc dex trending --network 1 --limit 25 -o table`,
	RunE: runDEXTrending,
}

func init() {
	dexNetworksCmd.Flags().Int("start", 1, "1-based index of the first network to return")
	dexNetworksCmd.Flags().Int("limit", 50, "Number of networks to return")
	dexNetworksCmd.Flags().String("sort", "id", "Sort field")
	dexNetworksCmd.Flags().String("sort-dir", "asc", "Sort direction")

	dexSearchCmd.Flags().Int("pages", dexDefaultSearchPages, "Number of DEX listing pages to scan locally (max 10)")
	dexSearchCmd.Flags().Int("limit", dexDefaultSearchLimit, "Maximum number of matching pairs to return")
	dexSearchCmd.Flags().String("network", "", "Optional network slug or numeric id for local post-filtering")
	dexSearchCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")

	dexPairsCmd.Flags().String("network", "", "Network slug or numeric id")
	dexPairsCmd.Flags().String("dex-id", "", "Optional DEX id filter")
	dexPairsCmd.Flags().String("dex-slug", "", "Optional DEX slug filter")
	dexPairsCmd.Flags().String("token", "", "Optional token contract address filter (matches base or quote asset exactly)")
	dexPairsCmd.Flags().String("address", "", "Hidden back-compat alias for pair contract address filter")
	dexPairsCmd.Flags().Int("limit", dexDefaultPairsLimit, "Maximum number of pairs to return")
	dexPairsCmd.Flags().String("sort", "volume_24h", "Sort field")
	dexPairsCmd.Flags().String("sort-dir", "desc", "Sort direction")
	dexPairsCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")
	_ = dexPairsCmd.Flags().MarkHidden("address")

	dexPairCmd.Flags().String("network", "", "Network slug or numeric id")
	dexPairCmd.Flags().String("pair", "", "Pair contract address")
	dexPairCmd.Flags().String("address", "", "Hidden back-compat alias for pair contract address")
	dexPairCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")
	_ = dexPairCmd.Flags().MarkHidden("address")

	dexOHLCVCmd.Flags().String("network", "", "Network slug or numeric id")
	dexOHLCVCmd.Flags().String("pair", "", "Pair contract address")
	dexOHLCVCmd.Flags().String("address", "", "Hidden back-compat alias for pair contract address")
	dexOHLCVCmd.Flags().String("window", "24h", "Window to fetch (1h, 24h, 30d)")
	dexOHLCVCmd.Flags().String("time-period", "hourly", "Hidden low-level historical window granularity label")
	dexOHLCVCmd.Flags().String("interval", "1h", "Hidden low-level candle interval")
	dexOHLCVCmd.Flags().String("from", "", "Hidden low-level start time in RFC3339 format")
	dexOHLCVCmd.Flags().String("to", "", "Hidden low-level end time in RFC3339 format")
	dexOHLCVCmd.Flags().Int("count", 24, "Hidden low-level maximum number of candles to return")
	dexOHLCVCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")
	_ = dexOHLCVCmd.Flags().MarkHidden("address")
	_ = dexOHLCVCmd.Flags().MarkHidden("time-period")
	_ = dexOHLCVCmd.Flags().MarkHidden("interval")
	_ = dexOHLCVCmd.Flags().MarkHidden("from")
	_ = dexOHLCVCmd.Flags().MarkHidden("to")
	_ = dexOHLCVCmd.Flags().MarkHidden("count")

	dexTradesCmd.Flags().String("network", "", "Network slug or numeric id")
	dexTradesCmd.Flags().String("pair", "", "Pair contract address")
	dexTradesCmd.Flags().String("address", "", "Hidden back-compat alias for pair contract address")
	dexTradesCmd.Flags().Int("limit", dexDefaultTradesLimit, "Maximum number of trades to return (max 100)")
	dexTradesCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")
	_ = dexTradesCmd.Flags().MarkHidden("address")

	dexTrendingCmd.Flags().String("network", "", "Network slug or numeric id")
	dexTrendingCmd.Flags().Int("limit", dexDefaultTrendingLimit, "Maximum number of pairs to return")
	dexTrendingCmd.Flags().String("convert-id", dexDefaultConvertID, "Conversion currency id")

	addOutputFlag(dexPairsCmd)
	addDryRunFlag(dexPairsCmd)
	addOutputFlag(dexPairCmd)
	addDryRunFlag(dexPairCmd)

	// Only pairs and pair are public. networks, search, ohlcv and trades call
	// /v4/dex endpoints that the spec files under Deprecated and that currently
	// return errors, and trending needs a DEX it does not take. They stay
	// unregistered until they move to the /v1/dex and /v1/k-line endpoints.
	dexCmd.AddCommand(dexPairsCmd, dexPairCmd)
	rootCmd.AddCommand(dexCmd)
}

func runDEXNetworks(cmd *cobra.Command, args []string) error {
	start, _ := cmd.Flags().GetInt("start")
	limit, _ := cmd.Flags().GetInt("limit")
	sortField, _ := cmd.Flags().GetString("sort")
	sortDir, _ := cmd.Flags().GetString("sort-dir")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		return printDryRun(cfg, "dex networks", "/v4/dex/networks/list", map[string]string{
			"start":    strconv.Itoa(start),
			"limit":    strconv.Itoa(limit),
			"sort":     sortField,
			"sort_dir": sortDir,
		}, nil)
	}

	client := newAPIClient(cfg)
	networks, err := client.DEXNetworksList(cmd.Context(), start, limit, sortField, sortDir)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONRaw(networks)
	}
	headers := []string{"ID", "Slug", "Name"}
	rows := make([][]string, len(networks))
	for i, network := range networks {
		rows[i] = []string{
			strconv.FormatInt(network.ID, 10),
			display.SanitizeCell(network.Slug),
			display.SanitizeCell(network.Name),
		}
	}
	display.PrintTable(headers, rows)
	return nil
}

func runDEXSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(args[0])
	pages, _ := cmd.Flags().GetInt("pages")
	limit, _ := cmd.Flags().GetInt("limit")
	network, _ := cmd.Flags().GetString("network")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}
	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}
	if pages < 1 {
		return fmt.Errorf("--pages must be at least 1")
	}
	if pages > dexMaxSearchPages {
		return fmt.Errorf("--pages must be <= %d", dexMaxSearchPages)
	}
	if limit < 1 {
		return fmt.Errorf("--limit must be at least 1")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		requests := make([]struct {
			opKey    string
			endpoint string
			params   map[string]string
		}, 0, pages)
		for page := 0; page < pages; page++ {
			start := page*dexDefaultListingsLimit + 1
			requests = append(requests, struct {
				opKey    string
				endpoint string
				params   map[string]string
			}{
				opKey:    "listings",
				endpoint: "/v4/dex/listings/quotes",
				params: map[string]string{
					"start":      strconv.Itoa(start),
					"limit":      strconv.Itoa(dexDefaultListingsLimit),
					"sort":       "volume_24h",
					"sort_dir":   "desc",
					"type":       "all",
					"convert_id": convertID,
				},
			})
		}
		if network != "" {
			requests[0].params["network_filter"] = network
		}
		return printDryRunMulti(cfg, "dex search", requests)
	}

	client := newAPIClient(cfg)
	networkID := ""
	if strings.TrimSpace(network) != "" {
		networkID, err = resolveDEXNetworkID(cmd.Context(), client, network)
		if err != nil {
			return err
		}
	}
	results, err := dexSearchPairs(cmd.Context(), client, query, networkID, pages, limit, convertID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONRaw(results)
	}
	renderDEXPairsTable(results, convertID)
	return nil
}

func runDEXPairs(cmd *cobra.Command, args []string) error {
	network, _ := cmd.Flags().GetString("network")
	dexID, _ := cmd.Flags().GetString("dex-id")
	dexSlug, _ := cmd.Flags().GetString("dex-slug")
	address, _ := cmd.Flags().GetString("address")
	token, _ := cmd.Flags().GetString("token")
	limit, _ := cmd.Flags().GetInt("limit")
	sortField, _ := cmd.Flags().GetString("sort")
	sortDir, _ := cmd.Flags().GetString("sort-dir")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}
	networkID, networkSlug, err := dexNetworkSelector(network)
	if err != nil {
		return err
	}
	if strings.TrimSpace(dexID) == "" && strings.TrimSpace(dexSlug) == "" {
		return fmt.Errorf("--dex-id or --dex-slug is required: the API lists spot pairs per DEX")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		params := map[string]string{
			"limit":      strconv.Itoa(limit),
			"sort":       sortField,
			"sort_dir":   sortDir,
			"convert_id": convertID,
		}
		addDEXNetworkParam(params, networkID, networkSlug)
		if dexID != "" {
			params["dex_id"] = dexID
		}
		if dexSlug != "" {
			params["dex_slug"] = dexSlug
		}
		if token != "" {
			params["token_filter"] = token
		}
		if address != "" {
			params["contract_address"] = address
		}
		return printDryRun(cfg, "dex pairs", "/v4/dex/spot-pairs/latest", params, nil)
	}

	client := newAPIClient(cfg)
	resp, err := client.DEXSpotPairsLatest(cmd.Context(), api.DEXSpotPairsLatestRequest{
		NetworkID:       networkID,
		NetworkSlug:     networkSlug,
		DEXID:           dexID,
		DEXSlug:         dexSlug,
		ContractAddress: address,
		Limit:           limit,
		Sort:            sortField,
		SortDir:         sortDir,
		ConvertID:       convertID,
	})
	if err != nil {
		return err
	}
	if token != "" {
		resp.Data = filterDEXPairsByToken(resp.Data, token)
	}
	// The API returns a fixed page of 100 whatever limit is sent.
	if limit > 0 && len(resp.Data) > limit {
		resp.Data = resp.Data[:limit]
	}
	if jsonOut {
		return printJSONRaw(resp.Data)
	}
	renderDEXPairsTable(resp.Data, convertID)
	return nil
}

func runDEXPair(cmd *cobra.Command, args []string) error {
	network, _ := cmd.Flags().GetString("network")
	pairAddress, _ := cmd.Flags().GetString("pair")
	address, _ := cmd.Flags().GetString("address")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		return fmt.Errorf("table output is not supported for dex pair; use -o json")
	}
	if pairAddress == "" {
		pairAddress = address
	}
	networkID, networkSlug, err := dexNetworkSelector(network)
	if err != nil {
		return err
	}
	if strings.TrimSpace(pairAddress) == "" {
		return fmt.Errorf("--pair is required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		params := map[string]string{
			"contract_address": pairAddress,
			"convert_id":       convertID,
		}
		addDEXNetworkParam(params, networkID, networkSlug)
		return printDryRun(cfg, "dex pair", "/v4/dex/pairs/quotes/latest", params, nil)
	}

	client := newAPIClient(cfg)
	pairs, err := client.DEXPairQuotesLatest(cmd.Context(), api.DEXPairLookupRequest{
		NetworkID:       networkID,
		NetworkSlug:     networkSlug,
		ContractAddress: pairAddress,
		ConvertID:       convertID,
	})
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return api.ErrAssetNotFound
	}
	return printJSONRaw(pairs[0])
}

func runDEXOHLCV(cmd *cobra.Command, args []string) error {
	network, _ := cmd.Flags().GetString("network")
	pairAddress, _ := cmd.Flags().GetString("pair")
	address, _ := cmd.Flags().GetString("address")
	window, _ := cmd.Flags().GetString("window")
	timePeriod, _ := cmd.Flags().GetString("time-period")
	interval, _ := cmd.Flags().GetString("interval")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	count, _ := cmd.Flags().GetInt("count")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}
	if pairAddress == "" {
		pairAddress = address
	}
	if strings.TrimSpace(network) == "" {
		return fmt.Errorf("--network is required")
	}
	if strings.TrimSpace(pairAddress) == "" {
		return fmt.Errorf("--pair is required")
	}
	if !cmd.Flags().Changed("time-period") && !cmd.Flags().Changed("interval") && !cmd.Flags().Changed("from") && !cmd.Flags().Changed("to") && !cmd.Flags().Changed("count") {
		resolvedTimePeriod, resolvedInterval, resolvedFrom, resolvedTo, resolvedCount, err := dexResolveWindow(window)
		if err != nil {
			return err
		}
		timePeriod, interval, from, to, count = resolvedTimePeriod, resolvedInterval, resolvedFrom, resolvedTo, resolvedCount
	} else if from == "" || to == "" {
		return fmt.Errorf("both --from and --to are required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		networkID, err := dryResolveNetwork(network)
		if err != nil {
			return err
		}
		return printDryRun(cfg, "dex ohlcv", "/v4/dex/pairs/ohlcv/historical", map[string]string{
			"network_id":       networkID,
			"contract_address": pairAddress,
			"time_period":      timePeriod,
			"interval":         interval,
			"time_start":       from,
			"time_end":         to,
			"count":            strconv.Itoa(count),
			"convert_id":       convertID,
		}, nil)
	}

	client := newAPIClient(cfg)
	networkID, err := resolveDEXNetworkID(cmd.Context(), client, network)
	if err != nil {
		return err
	}
	points, err := client.DEXPairsOHLCVHistorical(cmd.Context(), api.DEXOHLCVHistoricalRequest{
		NetworkID:       networkID,
		ContractAddress: pairAddress,
		TimePeriod:      timePeriod,
		Interval:        interval,
		TimeStart:       from,
		TimeEnd:         to,
		Count:           count,
		ConvertID:       convertID,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONRaw(points)
	}
	headers := []string{"Open Time", "Close Time", "Open", "High", "Low", "Close", "Volume", "Liquidity"}
	rows := make([][]string, len(points))
	for i, point := range points {
		quote := point.Quote[convertID]
		rows[i] = []string{
			point.TimeOpen,
			point.TimeClose,
			display.FormatPrice(quote.Open, "usd"),
			display.FormatPrice(quote.High, "usd"),
			display.FormatPrice(quote.Low, "usd"),
			display.FormatPrice(quote.Close, "usd"),
			display.FormatLargeNumber(quote.Volume24h, "usd"),
			display.FormatLargeNumber(quote.Liquidity, "usd"),
		}
	}
	display.PrintTable(headers, rows)
	return nil
}

func runDEXTrades(cmd *cobra.Command, args []string) error {
	network, _ := cmd.Flags().GetString("network")
	pairAddress, _ := cmd.Flags().GetString("pair")
	address, _ := cmd.Flags().GetString("address")
	limit, _ := cmd.Flags().GetInt("limit")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}
	if pairAddress == "" {
		pairAddress = address
	}
	if strings.TrimSpace(network) == "" {
		return fmt.Errorf("--network is required")
	}
	if strings.TrimSpace(pairAddress) == "" {
		return fmt.Errorf("--pair is required")
	}
	if limit < 1 || limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		networkID, err := dryResolveNetwork(network)
		if err != nil {
			return err
		}
		return printDryRun(cfg, "dex trades", "/v4/dex/pairs/trade/latest", map[string]string{
			"network_id":       networkID,
			"contract_address": pairAddress,
			"limit":            strconv.Itoa(limit),
			"convert_id":       convertID,
		}, nil)
	}

	client := newAPIClient(cfg)
	networkID, err := resolveDEXNetworkID(cmd.Context(), client, network)
	if err != nil {
		return err
	}
	trades, err := client.DEXPairsTradeLatest(cmd.Context(), api.DEXTradeLatestRequest{
		NetworkID:       networkID,
		ContractAddress: pairAddress,
		Limit:           limit,
		ConvertID:       convertID,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONRaw(trades)
	}
	headers := []string{"Timestamp", "Type", "Price", "Txn Hash"}
	rows := make([][]string, len(trades))
	for i, trade := range trades {
		quote := trade.QuoteFor(convertID)
		rows[i] = []string{
			trade.TradeTimestamp,
			display.SanitizeCell(strings.ToUpper(trade.Type)),
			display.FormatPrice(quote.Price, "usd"),
			display.SanitizeCell(trade.TransactionHash),
		}
	}
	display.PrintTable(headers, rows)
	return nil
}

func runDEXTrending(cmd *cobra.Command, args []string) error {
	network, _ := cmd.Flags().GetString("network")
	limit, _ := cmd.Flags().GetInt("limit")
	convertID, _ := cmd.Flags().GetString("convert-id")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}
	if strings.TrimSpace(network) == "" {
		return fmt.Errorf("--network is required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if isDryRun(cmd) {
		networkID, err := dryResolveNetwork(network)
		if err != nil {
			return err
		}
		return printDryRun(cfg, "dex trending", "/v4/dex/spot-pairs/latest", map[string]string{
			"network_id": networkID,
			"limit":      strconv.Itoa(limit),
			"sort":       "no_of_transactions_24h",
			"sort_dir":   "desc",
			"convert_id": convertID,
		}, nil)
	}

	client := newAPIClient(cfg)
	networkID, err := resolveDEXNetworkID(cmd.Context(), client, network)
	if err != nil {
		return err
	}
	resp, err := client.DEXSpotPairsLatest(cmd.Context(), api.DEXSpotPairsLatestRequest{
		NetworkID: networkID,
		Limit:     limit,
		Sort:      "no_of_transactions_24h",
		SortDir:   "desc",
		ConvertID: convertID,
	})
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONRaw(resp.Data)
	}
	renderDEXPairsTable(resp.Data, convertID)
	return nil
}

func resolveDEXNetworkID(ctx context.Context, client *api.Client, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--network is required")
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value, nil
	}

	start := 1
	for {
		networks, err := client.DEXNetworksList(ctx, start, dexNetworkPageLimit, "id", "asc")
		if err != nil {
			return "", err
		}
		if len(networks) == 0 {
			break
		}
		for _, network := range networks {
			if strings.EqualFold(network.Slug, value) {
				return strconv.FormatInt(network.ID, 10), nil
			}
		}
		if len(networks) < dexNetworkPageLimit {
			break
		}
		start += dexNetworkPageLimit
	}
	return "", fmt.Errorf("network %q not found (%w)", value, api.ErrAssetNotFound)
}

// dexNetworkSelector maps --network to the API's network_id (numeric) or
// network_slug. The API accepts both, so no lookup request is needed.
func dexNetworkSelector(value string) (id, slug string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("--network is required")
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value, "", nil
	}
	return "", strings.ToLower(value), nil
}

func addDEXNetworkParam(params map[string]string, id, slug string) {
	if id != "" {
		params["network_id"] = id
		return
	}
	params["network_slug"] = slug
}

func dryResolveNetwork(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--network is required")
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value, nil
	}
	return url.QueryEscape(value), nil
}

type dexSearchResult struct {
	pair          api.DEXPair
	matchStrength int
}

func dexSearchPairs(ctx context.Context, client *api.Client, query string, networkID string, pages, limit int, convertID string) ([]api.DEXPair, error) {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	seen := map[string]dexSearchResult{}
	for page := 0; page < pages; page++ {
		start := page*dexDefaultListingsLimit + 1
		listings, err := client.DEXListingsQuotes(ctx, start, dexDefaultListingsLimit, "volume_24h", "desc", "all", convertID)
		if err != nil {
			return nil, err
		}
		if listings == nil || len(listings.Data) == 0 {
			break
		}
		for _, listing := range listings.Data {
			resp, err := client.DEXSpotPairsLatest(ctx, api.DEXSpotPairsLatestRequest{
				DEXID:     strconv.FormatInt(listing.ID, 10),
				Limit:     dexDefaultPairsLimit,
				Sort:      "volume_24h",
				SortDir:   "desc",
				ConvertID: convertID,
			})
			if err != nil {
				return nil, err
			}
			for _, pair := range resp.Data {
				if networkID != "" && strconv.FormatInt(pair.NetworkID, 10) != networkID {
					continue
				}
				matchStrength := dexPairMatchStrength(lowerQuery, pair)
				if matchStrength == -1 {
					continue
				}
				key := fmt.Sprintf("%d:%s", pair.NetworkID, strings.ToLower(pair.ContractAddress))
				existing, ok := seen[key]
				if !ok || betterDEXSearchResult(matchStrength, pair, existing.matchStrength, existing.pair, convertID) {
					seen[key] = dexSearchResult{pair: pair, matchStrength: matchStrength}
				}
			}
		}
	}

	results := make([]dexSearchResult, 0, len(seen))
	for _, result := range seen {
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].matchStrength != results[j].matchStrength {
			return results[i].matchStrength < results[j].matchStrength
		}
		iq := results[i].pair.QuoteFor(convertID)
		jq := results[j].pair.QuoteFor(convertID)
		if iq.NoOfTransactions24h != jq.NoOfTransactions24h {
			return iq.NoOfTransactions24h > jq.NoOfTransactions24h
		}
		if iq.Volume24h != jq.Volume24h {
			return iq.Volume24h > jq.Volume24h
		}
		if iq.Liquidity != jq.Liquidity {
			return iq.Liquidity > jq.Liquidity
		}
		return results[i].pair.ContractAddress < results[j].pair.ContractAddress
	})

	if len(results) > limit {
		results = results[:limit]
	}
	out := make([]api.DEXPair, len(results))
	for i, result := range results {
		out[i] = result.pair
	}
	return out, nil
}

func dexResolveWindow(window string) (timePeriod, interval, from, to string, count int, err error) {
	now := dexNow().UTC()
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "1h":
		return "hourly", "5m", now.Add(-1 * time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), 12, nil
	case "24h":
		return "hourly", "1h", now.Add(-24 * time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), 24, nil
	case "30d":
		return "daily", "1d", now.AddDate(0, 0, -30).Format(time.RFC3339), now.Format(time.RFC3339), 30, nil
	default:
		return "", "", "", "", 0, fmt.Errorf("invalid --window %q — must be one of: 1h, 24h, 30d", window)
	}
}

func betterDEXSearchResult(newStrength int, newPair api.DEXPair, oldStrength int, oldPair api.DEXPair, convertID string) bool {
	if newStrength != oldStrength {
		return newStrength < oldStrength
	}
	newQuote := newPair.QuoteFor(convertID)
	oldQuote := oldPair.QuoteFor(convertID)
	if newQuote.NoOfTransactions24h != oldQuote.NoOfTransactions24h {
		return newQuote.NoOfTransactions24h > oldQuote.NoOfTransactions24h
	}
	if newQuote.Volume24h != oldQuote.Volume24h {
		return newQuote.Volume24h > oldQuote.Volume24h
	}
	return newQuote.Liquidity > oldQuote.Liquidity
}

func dexPairMatchStrength(query string, pair api.DEXPair) int {
	fields := []string{
		strings.ToLower(pair.ContractAddress),
		strings.ToLower(pair.DEXName),
		strings.ToLower(pair.DEXSlug),
		strings.ToLower(pair.BaseAsset.Name),
		strings.ToLower(pair.BaseAsset.Symbol),
		strings.ToLower(pair.BaseAsset.ContractAddress),
		strings.ToLower(pair.QuoteAsset.Name),
		strings.ToLower(pair.QuoteAsset.Symbol),
		strings.ToLower(pair.QuoteAsset.ContractAddress),
		strings.ToLower(pair.PairLabel()),
	}

	for _, field := range fields {
		if field != "" && field == query {
			return 0
		}
	}
	for _, field := range fields {
		if field != "" && strings.HasPrefix(field, query) {
			return 1
		}
	}
	for _, field := range fields {
		if field != "" && strings.Contains(field, query) {
			return 2
		}
	}
	return -1
}

func renderDEXPairsTable(pairs []api.DEXPair, convertID string) {
	headers := []string{"Network", "DEX", "Pair", "Address", "Txns 24h", "Volume 24h", "Liquidity", "Price"}
	rows := make([][]string, len(pairs))
	for i, pair := range pairs {
		quote := pair.QuoteFor(convertID)
		rows[i] = []string{
			display.SanitizeCell(pair.NetworkSlug),
			display.SanitizeCell(pair.DEXName),
			display.SanitizeCell(pair.PairLabel()),
			display.SanitizeCell(pair.ContractAddress),
			fmt.Sprintf("%.0f", quote.NoOfTransactions24h),
			display.FormatLargeNumber(quote.Volume24h, "usd"),
			display.FormatLargeNumber(quote.Liquidity, "usd"),
			display.FormatPrice(quote.Price, "usd"),
		}
	}
	display.PrintTable(headers, rows)
}

func filterDEXPairsByToken(pairs []api.DEXPair, token string) []api.DEXPair {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return pairs
	}
	filtered := make([]api.DEXPair, 0, len(pairs))
	for _, pair := range pairs {
		if strings.ToLower(pair.BaseAsset.ContractAddress) == token || strings.ToLower(pair.QuoteAsset.ContractAddress) == token {
			filtered = append(filtered, pair)
		}
	}
	return filtered
}
