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

func TestTopGainersLosers_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.URL, "/v1/cryptocurrency/trending/gainers-losers")
}

func TestTopGainersLosers_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/cryptocurrency/trending/gainers-losers", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("start"))
		assert.Equal(t, "100", r.URL.Query().Get("limit"))
		assert.Equal(t, "24h", r.URL.Query().Get("time_period"))
		assert.Equal(t, "USD", r.URL.Query().Get("convert"))

		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							Volume24h:        1000000000,
							MarketCap:        1000000000000,
							PercentChange24h: 5.5,
						},
					},
				},
				{
					ID:     2,
					Name:   "Ethereum",
					Symbol: "ETH",
					Slug:   "ethereum",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            3000,
							Volume24h:        200000000,
							MarketCap:        400000000000,
							PercentChange24h: -4.2,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers", "-o", "json")
	require.NoError(t, err)

	var result topGainersLosersJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "24h", result.TimePeriod)
	require.Len(t, result.Gainers, 1)
	require.Len(t, result.Losers, 1)
	assert.Equal(t, "Bitcoin", result.Gainers[0].Name)
	assert.Equal(t, "Ethereum", result.Losers[0].Name)
}

func TestTopGainersLosers_JSONOutput_UsesSelectedTimePeriodBuckets(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "7d", r.URL.Query().Get("time_period"))

		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							Volume24h:        1000000000,
							MarketCap:        1000000000000,
							PercentChange24h: 5.5,
							PercentChange7d:  -2.25,
						},
					},
				},
				{
					ID:     2,
					Name:   "Ethereum",
					Symbol: "ETH",
					Slug:   "ethereum",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            3000,
							Volume24h:        200000000,
							MarketCap:        400000000000,
							PercentChange24h: -4.2,
							PercentChange7d:  8.75,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers", "--time-period", "7d", "-o", "json")
	require.NoError(t, err)

	var result topGainersLosersJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "7d", result.TimePeriod)
	require.Len(t, result.Gainers, 1)
	require.Len(t, result.Losers, 1)
	assert.Equal(t, "Ethereum", result.Gainers[0].Name)
	assert.Equal(t, "Bitcoin", result.Losers[0].Name)
}

func TestTopGainersLosers_CustomParameters(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("start"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, "7d", r.URL.Query().Get("time_period"))
		assert.Equal(t, "EUR", r.URL.Query().Get("convert"))

		resp := api.TrendingGainersLosersResponse{Data: []api.TrendingCoin2{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "top-gainers-losers", "--start", "10", "--limit", "50", "--time-period", "7d", "--convert", "EUR", "-o", "json")
	require.NoError(t, err)
}

func TestTopGainersLosers_OneHourTimePeriod(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1h", r.URL.Query().Get("time_period"))
		resp := api.TrendingGainersLosersResponse{Data: []api.TrendingCoin2{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "top-gainers-losers", "--time-period", "1h", "-o", "json")
	require.NoError(t, err)
}

func TestTopGainersLosers_TimePeriodValidation(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "top-gainers-losers", "--time-period", "invalid", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --time-period")
}

func TestTopGainersLosers_LimitValidation(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "top-gainers-losers", "--limit", "0", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--limit must be between 1 and 200")

	_, _, err = executeCommand(t, "top-gainers-losers", "--limit", "201", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--limit must be between 1 and 200")
}

func TestTopGainersLosers_StartValidation(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "top-gainers-losers", "--start", "0", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--start must be at least 1")
}

func TestTopGainersLosers_TableOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							PercentChange24h: 5.5,
						},
					},
				},
				{
					ID:     2,
					Name:   "Ethereum",
					Symbol: "ETH",
					Slug:   "ethereum",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            3000,
							PercentChange24h: 3.2,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Bitcoin")
	assert.Contains(t, stdout, "Ethereum")
	assert.Contains(t, stdout, "BTC")
	assert.Contains(t, stdout, "ETH")
}

func TestTopGainersLosers_TableHeaderReflectsTimePeriod(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							PercentChange24h: 5.5,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers", "--time-period", "7d")
	require.NoError(t, err)
	assert.Contains(t, stdout, "7d %")
	assert.NotContains(t, stdout, "24h %")
}

func TestTopGainersLosers_TableUsesSelectedTimePeriodValue(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							PercentChange24h: 5.5,
							PercentChange7d:  -9.9,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "top-gainers-losers", "--time-period", "7d")
	require.NoError(t, err)
	assert.Contains(t, stdout, "-9.90%")
	assert.NotContains(t, stdout, "5.50%")
}

func TestTopGainersLosers_CommandsCatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var info commandInfo
	found := false
	for _, cmd := range catalog.Commands {
		if cmd.Name == "top-gainers-losers" {
			info = cmd
			found = true
			break
		}
	}
	require.True(t, found, "top-gainers-losers command should be present in catalog")
	assert.Equal(t, "/v1/cryptocurrency/trending/gainers-losers", info.APIEndpoint)
	assert.Equal(t, "getV1CryptocurrencyTrendingGainerslosers", info.OASOperationID)
	assert.False(t, info.PaidOnly)

	flags := map[string]flagInfo{}
	for _, flag := range info.Flags {
		flags[flag.Name] = flag
	}
	require.Contains(t, flags, "time-period")
	assert.Equal(t, []string{"1h", "24h", "7d", "30d"}, flags["time-period"].Enum)
	assert.Contains(t, info.Examples, "cmc top-gainers-losers --limit 50 --time-period 7d")
}

func TestTopGainersLosers_ExportWritesCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "gainers.csv")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.TrendingGainersLosersResponse{
			Data: []api.TrendingCoin2{
				{
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote: map[string]api.Quote2{
						"USD": {
							Price:            50000,
							Volume24h:        1e9,
							MarketCap:        1e12,
							PercentChange24h: 5.5,
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, stderr, err := executeCommand(t, "top-gainers-losers", "--limit", "1", "--export", csvPath, "-o", "table")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")

	body, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, "Name")
	assert.Contains(t, s, "Bitcoin")
}
