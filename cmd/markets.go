package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"

	"github.com/spf13/cobra"
)

var marketsCmd = &cobra.Command{
	Use:   "markets",
	Short: "List top coins by market cap",
	Long:  "Fetch paginated list of cryptocurrencies with latest market data from CMC listings/latest endpoint. Uses start+limit pagination.",
	Example: `  cmc markets
  cmc markets --start 51 --limit 50
  cmc markets --category layer-2
  cmc markets --total 1000
  cmc markets --sort volume_24h --sort-dir desc --convert EUR
  cmc markets --export coins.csv`,
	RunE: runMarkets,
}

func init() {
	marketsCmd.Flags().Int("start", 1, "Offset start (1-based)")
	marketsCmd.Flags().Int("limit", 100, "Number of results to return (1-5000)")
	marketsCmd.Flags().Int("total", 0, "Automatically paginate until N results have been collected")
	marketsCmd.Flags().String("category", "", "Filter by CoinMarketCap category slug")
	marketsCmd.Flags().String("convert", "USD", "Target currency")
	marketsCmd.Flags().String("sort", "", "Sort by: market_cap, price, volume_24h, etc.")
	marketsCmd.Flags().String("sort-dir", "", "Sort direction: asc or desc")
	marketsCmd.Flags().String("export", "", "Export to CSV file path")
	addOutputFlag(marketsCmd)
	addDryRunFlag(marketsCmd)
	rootCmd.AddCommand(marketsCmd)
}

func runMarkets(cmd *cobra.Command, args []string) error {
	start, _ := cmd.Flags().GetInt("start")
	limit, _ := cmd.Flags().GetInt("limit")
	total, _ := cmd.Flags().GetInt("total")
	category, _ := cmd.Flags().GetString("category")
	convert, _ := cmd.Flags().GetString("convert")
	sort, _ := cmd.Flags().GetString("sort")
	sortDir, _ := cmd.Flags().GetString("sort-dir")
	exportPath, _ := cmd.Flags().GetString("export")

	if err := validateMarketsSortDir(sortDir); err != nil {
		return err
	}

	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if start < 1 {
		return fmt.Errorf("--start must be at least 1")
	}
	if limit < 1 || limit > 5000 {
		return fmt.Errorf("--limit must be between 1 and 5000")
	}
	if total < 0 {
		return fmt.Errorf("--total must be a positive integer")
	}
	if total > 0 && cmd.Flags().Changed("limit") {
		return fmt.Errorf("--total and --limit cannot be used together")
	}
	category = strings.TrimSpace(category)
	convert = strings.ToUpper(strings.TrimSpace(convert))
	if convert == "" {
		return fmt.Errorf("--convert must not be empty")
	}

	if isDryRun(cmd) {
		if category != "" {
			outputs := buildMarketsCategoryDryRunOutputs(cfg, start, limit, total, convert, sort, sortDir, category)
			return printJSONRaw(outputs)
		}
		requests := buildMarketsDryRunRequests(start, limit, total, convert, sort, sortDir, category)
		if len(requests) == 1 {
			req := requests[0]
			return printDryRun(cfg, "markets", "/v3/cryptocurrency/listings/latest", req, paginationForTotal(total))
		}
		multi := make([]struct {
			opKey    string
			endpoint string
			params   map[string]string
		}, 0, len(requests))
		for _, params := range requests {
			multi = append(multi, struct {
				opKey    string
				endpoint string
				params   map[string]string
			}{endpoint: "/v3/cryptocurrency/listings/latest", params: params})
		}
		return printDryRunMulti(cfg, "markets", multi)
	}

	client := newAPIClient(cfg)
	ctx := cmd.Context()
	coins, err := fetchMarkets(ctx, client, start, limit, total, convert, sort, sortDir, category)
	if err != nil {
		return err
	}

	headers := []string{"Rank", "Name", "Symbol", "Price", "Market Cap", "Volume", "24h Change"}
	rows := make([][]string, len(coins))
	var csvRows [][]string
	if exportPath != "" {
		csvRows = make([][]string, len(coins))
	}
	for i, c := range coins {
		quote, ok := c.Quote[convert]
		if !ok {
			continue
		}
		rank := fmt.Sprintf("%d", c.CMCRank)
		name := display.SanitizeCell(c.Name)
		symbol := display.FormatSymbol(c.Symbol)
		rows[i] = []string{
			rank, name, symbol,
			display.FormatPrice(quote.Price, convert),
			display.FormatLargeNumber(quote.MarketCap, convert),
			display.FormatLargeNumber(quote.Volume24h, convert),
			display.ColorPercent(quote.PercentChange24h),
		}
		if exportPath != "" {
			csvRows[i] = []string{
				rank, name, symbol,
				fmt.Sprintf("%.8f", quote.Price),
				fmt.Sprintf("%.2f", quote.MarketCap),
				fmt.Sprintf("%.2f", quote.Volume24h),
				fmt.Sprintf("%.2f", quote.PercentChange24h),
			}
		}
	}

	if jsonOut {
		if exportPath != "" {
			if err := exportCSV(exportPath, headers, csvRows); err != nil {
				return err
			}
		}
		// Output compact default: just the Data array
		return printJSONRaw(coins)
	}

	display.PrintTable(headers, rows)

	if exportPath != "" {
		if err := exportCSV(exportPath, headers, csvRows); err != nil {
			return err
		}
	}

	return nil
}

func validateMarketsSortDir(sortDir string) error {
	if sortDir == "" {
		return nil
	}
	switch sortDir {
	case "asc", "desc":
		return nil
	default:
		return fmt.Errorf("--sort-dir must be asc or desc")
	}
}

func fetchMarkets(ctx context.Context, client *api.Client, start, limit, total int, convert, sort, sortDir, category string) ([]api.ListingCoin, error) {
	if total <= 0 {
		return client.ListingsLatestWithCategory(ctx, start, limit, convert, sort, sortDir, category)
	}

	remaining := total
	nextStart := start
	coins := make([]api.ListingCoin, 0, total)
	for remaining > 0 {
		pageLimit := remaining
		if pageLimit > 5000 {
			pageLimit = 5000
		}
		batch, err := client.ListingsLatestWithCategory(ctx, nextStart, pageLimit, convert, sort, sortDir, category)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		coins = append(coins, batch...)
		remaining -= len(batch)
		nextStart += len(batch)
	}
	return coins, nil
}

func buildMarketsDryRunRequests(start, limit, total int, convert, sort, sortDir, category string) []map[string]string {
	addCommon := func(params map[string]string, pageStart, pageLimit int) map[string]string {
		params["start"] = fmt.Sprintf("%d", pageStart)
		params["limit"] = fmt.Sprintf("%d", pageLimit)
		params["convert"] = convert
		if sort != "" {
			params["sort"] = sort
		}
		if sortDir != "" {
			params["sort_dir"] = sortDir
		}
		return params
	}

	if category != "" {
		if total <= 0 {
			return []map[string]string{addCommon(map[string]string{"id": category}, start, limit)}
		}

		requests := []map[string]string{}
		remaining := total
		nextStart := start
		for remaining > 0 {
			pageLimit := remaining
			if pageLimit > 5000 {
				pageLimit = 5000
			}
			requests = append(requests, addCommon(map[string]string{"id": category}, nextStart, pageLimit))
			remaining -= pageLimit
			nextStart += pageLimit
		}
		return requests
	}

	if total <= 0 {
		return []map[string]string{addCommon(map[string]string{}, start, limit)}
	}

	requests := []map[string]string{}
	remaining := total
	nextStart := start
	for remaining > 0 {
		pageLimit := remaining
		if pageLimit > 5000 {
			pageLimit = 5000
		}
		requests = append(requests, addCommon(map[string]string{}, nextStart, pageLimit))
		remaining -= pageLimit
		nextStart += pageLimit
	}
	return requests
}

func buildMarketsCategoryDryRunOutputs(cfg *config.Config, start, limit, total int, convert, sort, sortDir, category string) []dryRunOutput {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{
		"Accept":     "application/json",
		"User-Agent": userAgent,
	}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}

	makeCategoryLookup := func() dryRunOutput {
		return dryRunOutput{
			Method:  "GET",
			URL:     cfg.BaseURL() + "/v1/cryptocurrency/categories",
			Params:  map[string]string{"start": "1", "limit": "5000"},
			Headers: headers,
			Note:    "Resolve the category token to a CMC category id before fetching category coins.",
		}
	}

	makeCategoryFetch := func(pageStart, pageLimit int) dryRunOutput {
		params := map[string]string{
			"id":      "<resolved category id>",
			"start":   fmt.Sprintf("%d", pageStart),
			"limit":   fmt.Sprintf("%d", pageLimit),
			"convert": convert,
		}
		if sort != "" {
			params["sort"] = sort
		}
		if sortDir != "" {
			params["sort_dir"] = sortDir
		}
		return dryRunOutput{
			Method:  "GET",
			URL:     cfg.BaseURL() + "/v1/cryptocurrency/category",
			Params:  params,
			Headers: headers,
			Note:    "Category id is resolved from the category token at runtime.",
		}
	}

	outputs := []dryRunOutput{makeCategoryLookup()}
	if total <= 0 {
		outputs = append(outputs, makeCategoryFetch(start, limit))
		return outputs
	}

	remaining := total
	nextStart := start
	for remaining > 0 {
		pageLimit := remaining
		if pageLimit > 5000 {
			pageLimit = 5000
		}
		outputs = append(outputs, makeCategoryFetch(nextStart, pageLimit))
		remaining -= pageLimit
		nextStart += pageLimit
	}
	return outputs
}

func paginationForTotal(total int) *paginationInfo {
	if total <= 0 {
		return nil
	}
	perPage := total
	if perPage > 5000 {
		perPage = 5000
	}
	pages := total / perPage
	if total%perPage != 0 {
		pages++
	}
	return &paginationInfo{
		TotalRequested: total,
		PerPage:        perPage,
		Pages:          pages,
	}
}
