package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOutputDefault_Markets_WithoutOutputFlag_CompactJSON locks the contract that
// data commands default to json when -o/--output is omitted via command-local flags.
// Most cmd tests use executeCommand, which injects -o table to stabilize assertions.
func TestOutputDefault_Markets_WithoutOutputFlag_CompactJSON(t *testing.T) {
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

	stdout, _, err := executeCommandCLI(t, "markets", "--limit", "1")
	require.NoError(t, err)

	var coins []api.ListingCoin
	err = json.Unmarshal([]byte(stdout), &coins)
	require.NoError(t, err, "default output should be JSON array: %q", stdout)
	require.Len(t, coins, 1)
	assert.Equal(t, "Bitcoin", coins[0].Name)
}

func TestOutputDefault_Price_WithoutOutputFlag_CompactJSON(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		resp := api.QuotesLatestResponse{
			Data: map[string]api.QuoteCoin{
				"1": {
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Quote: map[string]api.Quote{
						"USD": {Price: 50000, PercentChange24h: 2.5},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommandCLI(t, "price", "--id", "1")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	err = json.Unmarshal([]byte(stdout), &quotes)
	require.NoError(t, err, "default output should be JSON object: %q", stdout)
	require.Contains(t, quotes, "1")
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
}

func TestOutputDefault_Resolve_WithoutOutputFlag_CompactJSON(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/cryptocurrency/info", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"1": map[string]any{
					"id":   1,
					"name": "Bitcoin",
					"slug": "bitcoin",
					"symbol": "BTC",
				},
			},
		})
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommandCLI(t, "resolve", "--id", "1")
	require.NoError(t, err)

	var assets []api.ResolvedAsset
	err = json.Unmarshal([]byte(stdout), &assets)
	require.NoError(t, err, "default output should be JSON array: %q", stdout)
	require.Len(t, assets, 1)
	assert.Equal(t, int64(1), assets[0].ID)
	assert.Equal(t, "bitcoin", assets[0].Slug)
}
