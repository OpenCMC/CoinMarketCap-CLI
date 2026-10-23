package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"

	"github.com/spf13/cobra"
)

var runMonitorLoop = defaultRunMonitorLoop

// keylessMinMonitorInterval keeps keyless polling well inside the shared
// IP-based rate pool.
const keylessMinMonitorInterval = 30 * time.Second

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Poll latest quotes on a fixed interval",
	Long:  "Poll CoinMarketCap latest quotes on a fixed interval. This is polling, not streaming.",
	Example: `  cmc monitor --id 1,1027
  cmc monitor --slug bitcoin,ethereum
  cmc monitor --symbol BTC,ETH -o json
  cmc monitor --id 1 --interval 120s`,
	RunE: runMonitor,
}

func init() {
	monitorCmd.Flags().String("id", "", "Comma-separated CMC IDs (e.g. 1,1027)")
	monitorCmd.Flags().String("slug", "", "Comma-separated slugs (e.g. bitcoin,ethereum)")
	monitorCmd.Flags().String("symbol", "", "Comma-separated symbols (e.g. BTC,ETH)")
	monitorCmd.Flags().String("convert", "USD", "Target currency")
	monitorCmd.Flags().String("interval", "60s", "Polling interval duration")
	addOutputFlag(monitorCmd)
	addDryRunFlag(monitorCmd)
	rootCmd.AddCommand(monitorCmd)
}

type monitorRow struct {
	PolledAt         string  `json:"polled_at"`
	ID               int64   `json:"id"`
	Slug             string  `json:"slug"`
	Symbol           string  `json:"symbol"`
	Name             string  `json:"name"`
	Price            float64 `json:"price"`
	PercentChange24h float64 `json:"percent_change_24h"`
}

func runMonitor(cmd *cobra.Command, args []string) error {
	idStr, _ := cmd.Flags().GetString("id")
	slugStr, _ := cmd.Flags().GetString("slug")
	symbolStr, _ := cmd.Flags().GetString("symbol")
	convert, _ := cmd.Flags().GetString("convert")
	intervalStr, _ := cmd.Flags().GetString("interval")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	if err := validateExactlyOneSelectorFamily(idStr, slugStr, symbolStr); err != nil {
		return err
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil || interval <= 0 {
		return fmt.Errorf("invalid --interval %q", intervalStr)
	}

	convert = strings.ToUpper(strings.TrimSpace(convert))
	if convert == "" {
		convert = "USD"
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.IsKeyless() && interval < keylessMinMonitorInterval {
		warnf("Warning: keyless mode polls at most every %s; using --interval %s. Add a free API key with `cmc auth` to poll faster.\n", keylessMinMonitorInterval, keylessMinMonitorInterval)
		interval = keylessMinMonitorInterval
	}

	if isDryRun(cmd) {
		params := map[string]string{
			"convert":  convert,
			"interval": interval.String(),
		}
		switch {
		case idStr != "":
			params["id"] = idStr
		case slugStr != "":
			params["slug"] = slugStr
		default:
			params["symbol"] = symbolStr
		}
		return printDryRunFull(cfg, "monitor", "", "/v3/cryptocurrency/quotes/latest", params, nil, "Polling command; emits one JSON object per asset per poll.")
	}

	client := newAPIClient(cfg)
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ids, err := resolveMonitorIDs(ctx, client, idStr, slugStr, symbolStr)
	if err != nil {
		return err
	}

	poll := func(ctx context.Context) error {
		quotes, err := client.QuotesLatestByID(ctx, ids, convert)
		if err != nil {
			return err
		}
		if len(quotes) == 0 {
			return fmt.Errorf("no valid assets found")
		}

		rows := buildMonitorRows(quotes, convert)
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			for _, row := range rows {
				if err := enc.Encode(row); err != nil {
					return err
				}
			}
			return nil
		}

		printMonitorTable(rows, convert)
		return nil
	}

	return runMonitorLoop(ctx, interval, poll)
}

func resolveMonitorIDs(ctx context.Context, client *api.Client, idStr, slugStr, symbolStr string) ([]string, error) {
	switch {
	case idStr != "":
		return splitTrim(idStr), nil
	case slugStr != "":
		slugs := splitTrim(slugStr)
		ids := make([]string, 0, len(slugs))
		for _, slug := range slugs {
			asset, err := client.ResolveBySlug(ctx, slug)
			if err != nil {
				return nil, err
			}
			ids = append(ids, strconv.FormatInt(asset.ID, 10))
		}
		return ids, nil
	default:
		symbols := splitTrim(symbolStr)
		ids := make([]string, 0, len(symbols))
		for _, symbol := range symbols {
			asset, err := client.ResolveBySymbol(ctx, symbol)
			if err != nil {
				var ambig *api.ResolverAmbiguityError
				if errors.As(err, &ambig) {
					return nil, fmt.Errorf("symbol %q is ambiguous; use --id or --slug instead (%w)", symbol, api.ErrResolverAmbiguous)
				}
				return nil, err
			}
			ids = append(ids, strconv.FormatInt(asset.ID, 10))
		}
		return ids, nil
	}
}

func defaultRunMonitorLoop(ctx context.Context, interval time.Duration, poll func(context.Context) error) error {
	if err := poll(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := poll(ctx); err != nil {
				return err
			}
		}
	}
}

func buildMonitorRows(quotes map[string]api.QuoteCoin, convert string) []monitorRow {
	keys := make([]string, 0, len(quotes))
	for key := range quotes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left, leftErr := strconv.ParseInt(keys[i], 10, 64)
		right, rightErr := strconv.ParseInt(keys[j], 10, 64)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return keys[i] < keys[j]
	})

	polledAt := time.Now().UTC().Format(time.RFC3339)
	rows := make([]monitorRow, 0, len(keys))
	for _, key := range keys {
		coin := quotes[key]
		quote, ok := coin.Quote[convert]
		if !ok {
			continue
		}
		rows = append(rows, monitorRow{
			PolledAt:         polledAt,
			ID:               coin.ID,
			Slug:             coin.Slug,
			Symbol:           coin.Symbol,
			Name:             coin.Name,
			Price:            quote.Price,
			PercentChange24h: quote.PercentChange24h,
		})
	}
	return rows
}

func printMonitorTable(rows []monitorRow, convert string) {
	fmt.Printf("Polled At: %s\n", time.Now().UTC().Format(time.RFC3339))
	headers := []string{"ID", "Name", "Symbol", "Price", "24h Change"}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			strconv.FormatInt(row.ID, 10),
			display.SanitizeCell(row.Name),
			display.FormatSymbol(row.Symbol),
			display.FormatPrice(row.Price, convert),
			display.ColorPercent(row.PercentChange24h),
		})
	}
	display.PrintTable(headers, tableRows)
	fmt.Println()
}
