package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withMonitorLoop(t *testing.T, fn func(ctx context.Context, interval time.Duration, poll func(context.Context) error) error) {
	t.Helper()
	orig := runMonitorLoop
	runMonitorLoop = fn
	t.Cleanup(func() { runMonitorLoop = orig })
}

func TestMonitor_MissingIdentity(t *testing.T) {
	_, _, err := executeCommand(t, "monitor", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide exactly one of --id, --slug, or --symbol")
}

func TestMonitor_MixedSelectorFamiliesFailClosed(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mixed selectors should fail before any HTTP request")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	cases := []struct {
		name string
		args []string
	}{
		{name: "id and symbol", args: []string{"monitor", "--id", "1", "--symbol", "BTC", "-o", "json"}},
		{name: "slug and symbol", args: []string{"monitor", "--slug", "bitcoin", "--symbol", "BTC", "-o", "json"}},
		{name: "id and slug", args: []string{"monitor", "--id", "1", "--slug", "bitcoin", "-o", "json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeCommand(t, tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "provide exactly one of --id, --slug, or --symbol")
		})
	}
}

func TestMonitor_DefaultIntervalIs60Seconds(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.QuotesLatestResponse{
			Data: map[string]api.QuoteCoin{
				"1": {
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote:  map[string]api.Quote{"USD": {Price: 50000, PercentChange24h: 2.5}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	var gotInterval time.Duration
	withMonitorLoop(t, func(ctx context.Context, interval time.Duration, poll func(context.Context) error) error {
		gotInterval = interval
		return poll(ctx)
	})

	_, _, err := executeCommand(t, "monitor", "--id", "1", "-o", "json")
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, gotInterval)
}

func TestMonitor_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "monitor", "--id", "1,1027", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "1,1027", out.Params["id"])
	assert.Equal(t, "USD", out.Params["convert"])
	assert.Equal(t, "1m0s", out.Params["interval"])
}

func TestMonitor_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
		resp := api.QuotesLatestResponse{
			Data: map[string]api.QuoteCoin{
				"1": {
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
					Quote:  map[string]api.Quote{"USD": {Price: 50000, PercentChange24h: 2.5}},
				},
				"1027": {
					ID:     1027,
					Name:   "Ethereum",
					Symbol: "ETH",
					Slug:   "ethereum",
					Quote:  map[string]api.Quote{"USD": {Price: 3500, PercentChange24h: -1.0}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	withMonitorLoop(t, func(ctx context.Context, interval time.Duration, poll func(context.Context) error) error {
		return poll(ctx)
	})

	stdout, _, err := executeCommand(t, "monitor", "--id", "1,1027", "-o", "json")
	require.NoError(t, err)

	lines := splitNonEmpty(stdout)
	require.Len(t, lines, 2)

	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row))
	assert.Equal(t, "Bitcoin", row["name"])
	assert.Equal(t, 50000.0, row["price"])
	assert.NotEmpty(t, row["polled_at"])
}

func TestMonitor_SymbolAmbiguityFailsClosed(t *testing.T) {
	callCount := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/v1/cryptocurrency/map" {
			resp := map[string]any{
				"data": []map[string]any{
					{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
					{"id": 99, "name": "BTC Proxy", "symbol": "BTC", "slug": "btc-proxy", "rank": 0, "is_active": 1},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	_, _, err := executeCommand(t, "monitor", "--symbol", "BTC", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
	assert.Equal(t, 1, callCount)
}

func TestMonitor_CommandsCatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var info commandInfo
	found := false
	for _, cmd := range catalog.Commands {
		if cmd.Name == "monitor" {
			info = cmd
			found = true
			break
		}
	}

	require.True(t, found)
	assert.Equal(t, "/v3/cryptocurrency/quotes/latest", info.APIEndpoint)
	assert.Equal(t, "rest", info.Transport)
}

func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
