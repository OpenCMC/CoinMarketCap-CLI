package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkets_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "markets", "--limit", "100", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Equal(t, "USD", out.Params["convert"])
	assert.Equal(t, "1", out.Params["start"])
	assert.Equal(t, "100", out.Params["limit"])
	assert.Contains(t, out.URL, "/v3/cryptocurrency/listings/latest")
}

func TestMarkets_DryRun_WithCategory(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "markets", "--category", "layer-2", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out []dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	require.Len(t, out, 2)

	assert.Contains(t, out[0].URL, "/v1/cryptocurrency/categories")
	assert.Equal(t, "1", out[0].Params["start"])
	assert.Equal(t, "5000", out[0].Params["limit"])
	assert.Contains(t, out[0].Note, "Resolve the category token")

	assert.Contains(t, out[1].URL, "/v1/cryptocurrency/category")
	assert.Equal(t, "<resolved category id>", out[1].Params["id"])
	assert.Equal(t, "1", out[1].Params["start"])
	assert.Equal(t, "100", out[1].Params["limit"])
	assert.Equal(t, "USD", out[1].Params["convert"])
	assert.Contains(t, out[1].Note, "resolved from the category token at runtime")
}

func TestMarkets_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/listings/latest", r.URL.Path)
		assert.Equal(t, "USD", r.URL.Query().Get("convert"))
		assert.Equal(t, "1", r.URL.Query().Get("start"))
		assert.Equal(t, "100", r.URL.Query().Get("limit"))

		resp := api.ListingsLatestResponse{
			Data: []api.ListingCoin{
				{
					ID:      1,
					Name:    "Bitcoin",
					Symbol:  "BTC",
					CMCRank: 1,
					Quote: map[string]api.Quote2{
						"USD": {Price: 50000, MarketCap: 1000000000000, Volume24h: 45000000000, PercentChange24h: 2.5},
					},
				},
				{
					ID:      1027,
					Name:    "Ethereum",
					Symbol:  "ETH",
					CMCRank: 2,
					Quote: map[string]api.Quote2{
						"USD": {Price: 3000, MarketCap: 400000000000, Volume24h: 25000000000, PercentChange24h: -1.2},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "markets", "--limit", "100", "-o", "json")
	require.NoError(t, err)

	var coins []api.ListingCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &coins))
	assert.Len(t, coins, 2)
	assert.Equal(t, "Bitcoin", coins[0].Name)
}

func TestMarkets_WithStartAndSort(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "51", r.URL.Query().Get("start"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, "volume_24h", r.URL.Query().Get("sort"))
		assert.Equal(t, "desc", r.URL.Query().Get("sort_dir"))

		resp := api.ListingsLatestResponse{Data: []api.ListingCoin{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "markets", "--start", "51", "--limit", "50", "--sort", "volume_24h", "--sort-dir", "desc", "-o", "json")
	require.NoError(t, err)
}

func TestMarkets_WithCategory(t *testing.T) {
	callCount := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			assert.Equal(t, "/v1/cryptocurrency/categories", r.URL.Path)
			assert.Equal(t, "1", r.URL.Query().Get("start"))
			assert.Equal(t, "5000", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "604f2776ebccdd50cd175fdc", "name": "Layer 2", "title": "Layer 2"},
				},
			})
		case 2:
			assert.Equal(t, "/v1/cryptocurrency/category", r.URL.Path)
			assert.Equal(t, "604f2776ebccdd50cd175fdc", r.URL.Query().Get("id"))
			assert.Equal(t, "USD", r.URL.Query().Get("convert"))
			resp := api.CategoryResponse{Data: api.CategoryDetail{Coins: []api.ListingCoin{}}}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected request %d", callCount)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "markets", "--category", "layer-2", "-o", "json")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestMarkets_TotalAutoPagination(t *testing.T) {
	callCount := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/v3/cryptocurrency/listings/latest", r.URL.Path)
		switch callCount {
		case 1:
			assert.Equal(t, "11", r.URL.Query().Get("start"))
			assert.Equal(t, "5000", r.URL.Query().Get("limit"))
			batch := make([]api.ListingCoin, 0, 5000)
			for i := 0; i < 5000; i++ {
				batch = append(batch, api.ListingCoin{
					ID:      int64(i + 1),
					Name:    "Coin",
					Symbol:  "C",
					CMCRank: 11 + i,
					Quote:   map[string]api.Quote2{"USD": {Price: float64(i + 1)}},
				})
			}
			resp := api.ListingsLatestResponse{Data: batch}
			_ = json.NewEncoder(w).Encode(resp)
		case 2:
			assert.Equal(t, "5011", r.URL.Query().Get("start"))
			assert.Equal(t, "1", r.URL.Query().Get("limit"))
			resp := api.ListingsLatestResponse{Data: []api.ListingCoin{
				{ID: 5001, Name: "Coin 5001", Symbol: "C5001", CMCRank: 5011, Quote: map[string]api.Quote2{"USD": {Price: 5001}}},
			}}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected extra request %d", callCount)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "markets", "--start", "11", "--total", "5001", "-o", "json")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)

	var coins []api.ListingCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &coins))
	require.Len(t, coins, 5001)
	assert.Equal(t, "Coin 5001", coins[5000].Name)
}

func TestMarkets_TotalAndLimitAreMutuallyExclusive(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "markets", "--limit", "50", "--total", "100", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--total and --limit cannot be used together")
}

func TestMarkets_InvalidSortDirFailsLocally(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when sort-dir validation fails")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommandCLI(t, "markets", "--sort-dir", "sideways", "--dry-run")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, err.Error(), "--sort-dir must be asc or desc")
}

func TestMarkets_CommandsCatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var info commandInfo
	found := false
	for _, cmd := range catalog.Commands {
		if cmd.Name == "markets" {
			info = cmd
			found = true
			break
		}
	}
	require.True(t, found, "markets command should be present in catalog")
	assert.Equal(t, "/v3/cryptocurrency/listings/latest", info.APIEndpoint)
	assert.Equal(t, "getV3CryptocurrencyListingsLatest", info.OASOperationID)

	flags := map[string]flagInfo{}
	for _, flag := range info.Flags {
		flags[flag.Name] = flag
	}
	require.Contains(t, flags, "sort")
	assert.Equal(t, []string{"market_cap", "price", "volume_24h"}, flags["sort"].Enum)
	require.Contains(t, flags, "sort-dir")
	assert.Equal(t, []string{"asc", "desc"}, flags["sort-dir"].Enum)
	assert.Contains(t, info.Examples, "cmc markets --start 51 --limit 50")
	require.Contains(t, info.APIEndpoints, "category")
	assert.Equal(t, "/v1/cryptocurrency/category", info.APIEndpoints["category"])
	require.Contains(t, info.APIEndpoints, "category_lookup")
	assert.Equal(t, "/v1/cryptocurrency/categories", info.APIEndpoints["category_lookup"])
}

func TestMarkets_ExportWritesCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "markets.csv")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.ListingsLatestResponse{
			Data: []api.ListingCoin{
				{
					ID:      1,
					Name:    "Bitcoin",
					Symbol:  "BTC",
					CMCRank: 1,
					Quote: map[string]api.Quote2{
						"USD": {Price: 50000, MarketCap: 1e12, Volume24h: 4.5e10, PercentChange24h: 2.5},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, stderr, err := executeCommand(t, "markets", "--limit", "1", "--export", csvPath, "-o", "table")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")
	assert.Contains(t, stderr, csvPath)

	body, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, "Rank")
	assert.Contains(t, s, "Bitcoin")
	assert.Contains(t, s, "BTC")
}

func TestMarkets_ExportWritesCSV_WithDefaultJSONOutput(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "markets.json-default.csv")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.ListingsLatestResponse{
			Data: []api.ListingCoin{
				{
					ID:      1,
					Name:    "Bitcoin",
					Symbol:  "BTC",
					CMCRank: 1,
					Quote: map[string]api.Quote2{
						"USD": {Price: 50000, MarketCap: 1e12, Volume24h: 4.5e10, PercentChange24h: 2.5},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommandCLI(t, "markets", "--limit", "1", "--export", csvPath)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")
	assert.Contains(t, stderr, csvPath)
	assert.Contains(t, stdout, "\"Bitcoin\"")

	body, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, "Rank")
	assert.Contains(t, s, "Bitcoin")
	assert.Contains(t, s, "BTC")
}
