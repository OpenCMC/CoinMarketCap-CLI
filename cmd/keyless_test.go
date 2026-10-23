package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTestClientKeyless runs commands with no API key against srv.
func withTestClientKeyless(t *testing.T, srv *httptest.Server) {
	t.Helper()

	origLoad := loadConfig
	loadConfig = func() (*config.Config, error) { return &config.Config{}, nil }
	t.Cleanup(func() { loadConfig = origLoad })

	origClient := newAPIClient
	newAPIClient = func(cfg *config.Config) *api.Client {
		c := api.NewClient(cfg)
		c.SetBaseURL(srv.URL)
		return c
	}
	t.Cleanup(func() { newAPIClient = origClient })
}

const keylessV3QuotesBody = `{"data":[{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","is_active":1,"quote":[{"id":2781,"symbol":"USD","price":81315.05,"percent_change_24h":-2.41,"volume_24h":33023294185.96,"market_cap":1634604147340.88}]}],"status":{"error_code":"0","error_message":""}}`

func TestKeyless_PriceCallsPublicAPIWithoutKey(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/public-api/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Empty(t, r.Header.Get("X-CMC_PRO_API_KEY"), "keyless requests must not send the key header")
		_, _ = w.Write([]byte(keylessV3QuotesBody))
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, _, err := executeCommandCLI(t, "price", "--id", "1", "-o", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":{"USD":{"price":81315.05,"percent_change_24h":-2.41,"volume_24h":33023294185.96,"market_cap":1634604147340.88}}}}`, stdout)
}

func TestKeyless_PriceJSONMatchesKeyedJSON(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(keylessV3QuotesBody))
	})
	defer srv.Close()

	withTestClientKeyless(t, srv)
	keyless, _, err := executeCommandCLI(t, "price", "--id", "1", "-o", "json")
	require.NoError(t, err)

	withTestClientDemo(t, srv)
	keyed, _, err := executeCommandCLI(t, "price", "--id", "1", "-o", "json")
	require.NoError(t, err)

	assert.JSONEq(t, keyed, keyless, "adding a key must not change price output")
}

func TestKeyless_MarketsUsesV3Listings(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/public-api/v3/cryptocurrency/listings/latest", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","cmc_rank":1,"quote":[{"id":2781,"symbol":"USD","price":81315.05,"volume_24h":1,"market_cap":2,"percent_change_1h":0.1,"percent_change_24h":-2.4,"percent_change_7d":-0.9,"percent_change_30d":-5.8}]}],"status":{"error_code":"0"}}`))
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, _, err := executeCommandCLI(t, "markets", "--limit", "1", "-o", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `[{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","cmc_rank":1,"quote":{"USD":{"price":81315.05,"volume_24h":1,"market_cap":2,"percent_change_1h":0.1,"percent_change_24h":-2.4,"percent_change_7d":-0.9,"percent_change_30d":-5.8}}}]`, stdout)
}

func TestKeyless_KeyOnlyCommandsStopBeforeAnyRequest(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("key-only command sent a request in keyless mode: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	for _, args := range [][]string{
		{"trending"},
		{"top-gainers-losers"},
		{"history", "--id", "1", "--days", "7"},
		{"history", "--id", "1", "--days", "7", "--dry-run"},
		{"news"},
		{"pairs", "bitcoin"},
		{"price", "--id", "1", "--with-chain-stats"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, _, err := executeCommandCLI(t, append(args, "-o", "json")...)
			require.Error(t, err)
			require.ErrorIs(t, err, api.ErrKeyRequired)
			assert.Contains(t, err.Error(), "cmc auth")

			cliErr := classifyError(err)
			assert.Equal(t, "api_key_required", cliErr.Error)
		})
	}
}

func TestKeyless_DryRunShowsPublicAPIAndNoKeyHeader(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, _, err := executeCommandCLI(t, "markets", "--dry-run", "-o", "json")
	require.NoError(t, err)
	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out))
	assert.Equal(t, "https://pro-api.coinmarketcap.com/public-api/v3/cryptocurrency/listings/latest", out.URL)
	assert.NotContains(t, out.Headers, "X-CMC_PRO_API_KEY")
	assert.Equal(t, "getV3CryptocurrencyListingsLatest", out.OASOperationID)
}

func TestKeyless_MonitorIntervalHasAFloor(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, stderr, err := executeCommandCLI(t, "monitor", "--id", "1", "--interval", "5s", "--dry-run")
	require.NoError(t, err)
	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out))
	assert.Equal(t, "30s", out.Params["interval"])
	assert.Contains(t, stderr, "keyless mode polls at most every 30s")
}

func TestKeyless_StatusReportsMode(t *testing.T) {
	withIsolatedUserConfigDir(t)
	t.Setenv("CMC_API_KEY", "")

	stdout, _, err := executeCommandCLI(t, "status", "-o", "json")
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, "keyless", got["mode"])
	assert.Equal(t, "https://pro-api.coinmarketcap.com/public-api", got["base_url"])
	assert.Nil(t, got["api_key"])
}

func TestKeyless_CatalogRequiresAuthComesFromSpec(t *testing.T) {
	cat := parseCommandsJSONCatalog(t)

	for name, want := range map[string]bool{
		"price":              false,
		"markets":            false,
		"metrics":            false,
		"resolve":            false,
		"search":             false,
		"monitor":            false,
		"api":                false,
		"dex pair":           false,
		"dex pairs":          false,
		"trending":           true,
		"top-gainers-losers": true,
		"history":            true,
		"news":               true,
		"pairs":              true,
	} {
		assert.Equal(t, want, commandInfoByName(t, cat, name).RequiresAuth, "requires_auth for %q", name)
	}
}

func TestKeyed_InvalidKeyReportsAuthError(t *testing.T) {
	var paths []string
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":{"error_code":1001,"error_message":"This API Key is invalid."}}`))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommandCLI(t, "metrics", "-o", "json")
	require.Error(t, err)
	assert.True(t, errors.Is(err, api.ErrInvalidAPIKey))
	assert.Equal(t, []string{"/v1/global-metrics/quotes/latest"}, paths, "an invalid key must not fall back to keyless")
}
