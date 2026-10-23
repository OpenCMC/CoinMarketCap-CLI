package cmd

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_RequiresIdentityFlag(t *testing.T) {
	// Flag validation runs after the keyless gate, so these need a key.
	withStubLoadConfig(t, &config.Config{APIKey: "test-key"})
	_, _, err := executeCommand(t, "history", "--date", "2024-01-01", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify exactly one of --id, --slug, or --symbol")
}

func TestHistory_RequiresExactlyOneMode(t *testing.T) {
	withStubLoadConfig(t, &config.Config{APIKey: "test-key"})
	_, _, err := executeCommand(t, "history", "--id", "1", "--date", "2024-01-01", "--days", "7", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify exactly one mode")
}

func TestHistory_Date_ByID_NormalizesToSingleUTCLogicalRow(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/cryptocurrency/quotes/historical", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		assert.Equal(t, "USD", r.URL.Query().Get("convert"))
		assert.Equal(t, "2023-12-31T00:00:00Z", r.URL.Query().Get("time_start"))
		assert.Equal(t, "2024-01-01T23:59:59Z", r.URL.Query().Get("time_end"))
		assert.Equal(t, "2", r.URL.Query().Get("count"))
		assert.Equal(t, "daily", r.URL.Query().Get("interval"))

		resp := map[string]any{
			"data": map[string]any{
				"1": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"slug":   "bitcoin",
					"quotes": []map[string]any{
						{
							"timestamp": "2023-12-31T23:59:59.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":      41000.0,
									"market_cap": 810000000000.0,
									"volume_24h": 23000000000.0,
								},
							},
						},
						{
							"timestamp": "2024-01-01T00:00:00.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":      42000.0,
									"market_cap": 820000000000.0,
									"volume_24h": 24000000000.0,
								},
							},
						},
						{
							"timestamp": "2024-01-01T12:00:00.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":      42500.0,
									"market_cap": 825000000000.0,
									"volume_24h": 24500000000.0,
								},
							},
						},
						{
							"timestamp": "2024-01-02T00:00:00.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":      43000.0,
									"market_cap": 830000000000.0,
									"volume_24h": 25000000000.0,
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--date", "2024-01-01", "-o", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "Bitcoin", rows[0]["name"])
	assert.Equal(t, "2024-01-01T12:00:00.000Z", rows[0]["timestamp"])
	assert.Equal(t, 42500.0, rows[0]["price"])
}

func TestHistory_DryRun_UsesQuotesHistoricalEndpoint(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "7", "-o", "json", "--dry-run")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.URL, "/v1/cryptocurrency/quotes/historical")
	assert.Equal(t, "1", out.Params["id"])
	assert.Equal(t, "USD", out.Params["convert"])
	assert.Equal(t, "daily", out.Params["interval"])
}

func TestHistory_DryRun_5mInterval_UsesQuotesHistoricalEndpoint(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "12", "--interval", "5m", "-o", "json", "--dry-run")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.URL, "/v1/cryptocurrency/quotes/historical")
	assert.Equal(t, "5m", out.Params["interval"])
}

func TestHistoryHelpTextMentions5m(t *testing.T) {
	require.Contains(t, historyCmd.Flags().Lookup("interval").Usage, "5m")
	require.Contains(t, historyCmd.Example, "--days 1 --interval 5m")
}

func TestHistory_DryRun_5mDaysModeAlignsToBoundary(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "1", "--interval", "5m", "-o", "json", "--dry-run")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	start, err := time.Parse(time.RFC3339, out.Params["time_start"])
	require.NoError(t, err)
	end, err := time.Parse(time.RFC3339, out.Params["time_end"])
	require.NoError(t, err)

	assert.Equal(t, 2, mustAtoi(t, out.Params["count"]))
	assert.Zero(t, end.Second())
	assert.Zero(t, end.Nanosecond())
	assert.Equal(t, 0, end.Minute()%5)
	assert.Equal(t, 5*time.Minute, end.Sub(start))
	assert.Zero(t, start.Second())
	assert.Zero(t, start.Nanosecond())
	assert.Equal(t, 0, start.Minute()%5)
}

func TestHistory_DryRun_HourlyDaysModeAlignsToBoundary(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "2", "--interval", "hourly", "-o", "json", "--dry-run")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	start, err := time.Parse(time.RFC3339, out.Params["time_start"])
	require.NoError(t, err)
	end, err := time.Parse(time.RFC3339, out.Params["time_end"])
	require.NoError(t, err)

	assert.Equal(t, 3, mustAtoi(t, out.Params["count"]))
	assert.Zero(t, end.Minute())
	assert.Zero(t, end.Second())
	assert.Zero(t, end.Nanosecond())
	assert.Equal(t, 2*time.Hour, end.Sub(start))
	assert.Zero(t, start.Minute())
	assert.Zero(t, start.Second())
	assert.Zero(t, start.Nanosecond())
}

func TestHistory_DaysRangeRequestsOneExtraRawInterval(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	ranges := buildHistoryRangesAt(historyMode{days: 1}, "5m", now)
	require.Len(t, ranges, 1)
	assert.Equal(t, now.Add(-5*time.Minute), ranges[0].start)
	assert.Equal(t, now, ranges[0].end)
	assert.Equal(t, 2, ranges[0].count)

	ranges = buildHistoryRangesAt(historyMode{days: 7}, "daily", now)
	require.Len(t, ranges, 1)
	assert.Equal(t, time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), ranges[0].start)
	assert.Equal(t, time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC), ranges[0].end)
	assert.Equal(t, 8, ranges[0].count)
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	require.NoError(t, err)
	return v
}

func TestHistory_SymbolAmbiguousFailsClosed(t *testing.T) {
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

	_, _, err := executeCommand(t, "history", "--symbol", "BTC", "--days", "7", "-o", "json")
	require.Error(t, err)
	assert.ErrorContains(t, err, "ambiguous")
	assert.Equal(t, 1, callCount)
}

func TestHistory_Range_ChunksAndDeduplicates(t *testing.T) {
	callCount := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/v1/cryptocurrency/quotes/historical", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		assert.Equal(t, "daily", r.URL.Query().Get("interval"))

		var quotes []map[string]any
		if callCount == 1 {
			quotes = []map[string]any{
				{
					"timestamp": "2024-01-01T00:00:00.000Z",
					"quote":     map[string]any{"USD": map[string]any{"price": 1.0, "market_cap": 10.0, "volume_24h": 100.0}},
				},
				{
					"timestamp": "2024-01-02T00:00:00.000Z",
					"quote":     map[string]any{"USD": map[string]any{"price": 2.0, "market_cap": 20.0, "volume_24h": 200.0}},
				},
			}
		} else {
			quotes = []map[string]any{
				{
					"timestamp": "2024-01-02T00:00:00.000Z",
					"quote":     map[string]any{"USD": map[string]any{"price": 2.0, "market_cap": 20.0, "volume_24h": 200.0}},
				},
				{
					"timestamp": "2024-01-03T00:00:00.000Z",
					"quote":     map[string]any{"USD": map[string]any{"price": 3.0, "market_cap": 30.0, "volume_24h": 300.0}},
				},
			}
		}

		resp := map[string]any{
			"data": map[string]any{
				"1": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"slug":   "bitcoin",
					"quotes": quotes,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--from", "2024-01-01", "--to", "2024-04-15", "-o", "json")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 3)
	assert.Equal(t, "2024-01-03T00:00:00.000Z", rows[2]["timestamp"])
}

func TestHistory_ExportWritesCSVInJSONAndTablePaths_WithRawValues(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "history-json.csv")
	tablePath := filepath.Join(dir, "history-table.csv")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/cryptocurrency/quotes/historical", r.URL.Path)
		resp := map[string]any{
			"data": map[string]any{
				"1": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"slug":   "bitcoin",
					"quotes": []map[string]any{
						{
							"timestamp": "2024-01-01T00:00:00.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":      42000.0,
									"market_cap": 8.2e11,
									"volume_24h": 2.4e10,
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, stderr, err := executeCommand(t, "history", "--id", "1", "--date", "2024-01-01", "--export", jsonPath, "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")
	requireFileHasCSVRow(t, jsonPath, []string{"Timestamp", "ID", "Name", "Symbol", "Price", "Market Cap", "Volume 24h"}, []string{"2024-01-01T00:00:00.000Z", "1", "Bitcoin", "BTC", "42000", "820000000000", "24000000000"})

	var jsonRows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &jsonRows))
	require.Len(t, jsonRows, 1)
	assert.Equal(t, 42000.0, jsonRows[0]["price"])

	stdout, stderr, err = executeCommand(t, "history", "--id", "1", "--date", "2024-01-01", "--export", tablePath, "-o", "table")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")
	assert.Contains(t, stdout, "Bitcoin")
	requireFileHasCSVRow(t, tablePath, []string{"Timestamp", "ID", "Name", "Symbol", "Price", "Market Cap", "Volume 24h"}, []string{"2024-01-01T00:00:00.000Z", "1", "Bitcoin", "BTC", "42000", "820000000000", "24000000000"})
}

func TestHistory_OHLC_BySlug_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/cryptocurrency/info" {
			resp := map[string]any{
				"data": map[string]any{
					"1": map[string]any{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "category": "coin"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		assert.Equal(t, "/v2/cryptocurrency/ohlcv/historical", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		assert.Equal(t, "hourly", r.URL.Query().Get("interval"))
		assert.Equal(t, "hourly", r.URL.Query().Get("time_period"))
		assert.Equal(t, "USD", r.URL.Query().Get("convert"))

		resp := map[string]any{
			"data": map[string]any{
				"bitcoin": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"slug":   "bitcoin",
					"quotes": []map[string]any{
						{
							"time_open":  "2024-01-01T01:00:00.000Z",
							"time_close": "2024-01-01T01:59:59.999Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"open":       42000.0,
									"high":       42100.0,
									"low":        41900.0,
									"close":      42050.0,
									"volume":     1234567.0,
									"market_cap": 820000000000.0,
									"timestamp":  "2024-01-01T01:59:59.999Z",
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClient(t, srv, config.TierHobbyist)

	stdout, _, err := executeCommand(t, "history", "--slug", "bitcoin", "--days", "1", "--interval", "hourly", "--ohlc", "-o", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, 42000.0, rows[0]["open"])
	assert.Equal(t, 42050.0, rows[0]["close"])
}

func TestHistory_OHLC_ExportWritesCSVWithRawValues(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "history-ohlc.csv")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/cryptocurrency/info" {
			resp := map[string]any{
				"data": map[string]any{
					"1": map[string]any{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "category": "coin"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		assert.Equal(t, "/v2/cryptocurrency/ohlcv/historical", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("id"))
		assert.Equal(t, "hourly", r.URL.Query().Get("interval"))
		assert.Equal(t, "hourly", r.URL.Query().Get("time_period"))
		resp := map[string]any{
			"data": map[string]any{
				"bitcoin": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"slug":   "bitcoin",
					"quotes": []map[string]any{
						{
							"time_open":  "2024-01-01T01:00:00.000Z",
							"time_close": "2024-01-01T01:59:59.999Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"open":       42000.0,
									"high":       42100.0,
									"low":        41900.0,
									"close":      42050.0,
									"volume":     1234567.0,
									"market_cap": 820000000000.0,
									"timestamp":  "2024-01-01T01:59:59.999Z",
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClient(t, srv, config.TierHobbyist)

	stdout, stderr, err := executeCommand(t, "history", "--slug", "bitcoin", "--days", "1", "--interval", "hourly", "--ohlc", "--export", csvPath, "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, stderr, "Exported to")
	requireFileHasCSVRow(t, csvPath, []string{"Timestamp", "ID", "Name", "Symbol", "Open", "High", "Low", "Close", "Market Cap", "Volume 24h"}, []string{"2024-01-01T01:00:00.000Z", "1", "Bitcoin", "BTC", "42000", "42100", "41900", "42050", "820000000000", "1234567"})

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, 42000.0, rows[0]["open"])
	assert.Equal(t, 42050.0, rows[0]["close"])
}

func requireFileHasCSVRow(t *testing.T, path string, wantHeader, wantRow []string) {
	t.Helper()

	body, err := os.ReadFile(path)
	require.NoError(t, err)

	r := csv.NewReader(strings.NewReader(string(body)))
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.Equal(t, wantHeader, records[0])
	require.GreaterOrEqual(t, len(records), 2)
	require.Equal(t, wantRow, records[1])
	for _, field := range records[1] {
		assert.NotContains(t, field, "USD ")
	}
}

func TestHistory_OHLC_5mRejectedInCommandLayer(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClient(t, srv, config.TierHobbyist)

	_, _, err := executeCommand(t, "history", "--id", "1", "--days", "1", "--interval", "5m", "--ohlc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--ohlc does not support --interval 5m")
}

func TestHistory_DaysMax_DailyQuoteUsesDateAdded(t *testing.T) {
	callCount := 0
	quoteCalls := 0
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			resp := map[string]any{
				"data": map[string]any{
					"1": map[string]any{
						"id":         1,
						"name":       "Bitcoin",
						"symbol":     "BTC",
						"slug":       "bitcoin",
						"category":   "coin",
						"date_added": "2020-01-15T00:00:00.000Z",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v1/cryptocurrency/quotes/historical":
			assert.Equal(t, "daily", r.URL.Query().Get("interval"))
			quoteCalls++
			if quoteCalls == 1 {
				assert.Equal(t, "2020-01-15T00:00:00Z", r.URL.Query().Get("time_start"))
			}
			resp := map[string]any{
				"data": map[string]any{
					"1": map[string]any{
						"id":     1,
						"name":   "Bitcoin",
						"symbol": "BTC",
						"slug":   "bitcoin",
						"quotes": []map[string]any{
							{
								"timestamp": "2020-01-15T00:00:00.000Z",
								"quote": map[string]any{
									"USD": map[string]any{
										"price":      9000.0,
										"market_cap": 100.0,
										"volume_24h": 10.0,
									},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "max", "-o", "json")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 2)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, 9000.0, rows[0]["price"])
}

func TestHistory_DaysMax_HourlyRejectedInCommandLayer(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	_, _, err := executeCommand(t, "history", "--id", "1", "--days", "max", "--interval", "hourly", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--days max only supports --interval daily")
}

func TestHistory_DaysMax_5mRejectedInCommandLayer(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	_, _, err := executeCommand(t, "history", "--id", "1", "--days", "max", "--interval", "5m", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--days max only supports --interval daily")
}

func TestHistory_DaysMax_OHLCRejectedInCommandLayer(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call when validation fails")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	_, _, err := executeCommand(t, "history", "--id", "1", "--days", "max", "--ohlc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--days max does not support --ohlc")
}

func TestHistory_BasicTierDoesNotHardGateBeforeServerResponse(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/cryptocurrency/quotes/historical":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":     1,
					"name":   "Bitcoin",
					"symbol": "BTC",
					"quotes": []map[string]any{
						{
							"timestamp": "2024-01-01T00:00:00.000Z",
							"quote": map[string]any{
								"USD": map[string]any{
									"price":              44187.14,
									"market_cap":         865000000000.0,
									"volume_24h":         12000000000.0,
									"percent_change_24h": 1.5,
								},
							},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClient(t, srv, config.TierBasic)

	stdout, _, err := executeCommand(t, "history", "--id", "1", "--days", "1", "-o", "json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, 1.0, rows[0]["id"])
}

func TestHistory_CommandsCatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var info commandInfo
	found := false
	for _, cmd := range catalog.Commands {
		if cmd.Name == "history" {
			info = cmd
			found = true
			break
		}
	}

	require.True(t, found)
	assert.Equal(t, "/v1/cryptocurrency/quotes/historical", info.APIEndpoints["--date"])
	assert.Equal(t, "/v2/cryptocurrency/ohlcv/historical", info.APIEndpoints["--days --ohlc"])
}
