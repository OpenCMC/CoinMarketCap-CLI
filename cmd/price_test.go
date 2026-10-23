package cmd

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type priceInfoURLsJSON struct {
	Website      []string `json:"website,omitempty"`
	TechnicalDoc []string `json:"technical_doc,omitempty"`
	Twitter      []string `json:"twitter,omitempty"`
	Reddit       []string `json:"reddit,omitempty"`
	MessageBoard []string `json:"message_board,omitempty"`
	Announcement []string `json:"announcement,omitempty"`
	Chat         []string `json:"chat,omitempty"`
	Explorer     []string `json:"explorer,omitempty"`
	SourceCode   []string `json:"source_code,omitempty"`
}

type priceWithInfoJSON struct {
	ID         int64                 `json:"id"`
	Name       string                `json:"name"`
	Symbol     string                `json:"symbol"`
	Slug       string                `json:"slug"`
	Quote      map[string]api.Quote  `json:"quote"`
	Info       *priceInfoDetailsJSON `json:"info,omitempty"`
	ChainStats *priceChainStatsJSON  `json:"chain_stats,omitempty"`
}

type priceInfoDetailsJSON struct {
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	URLs        priceInfoURLsJSON `json:"urls,omitempty"`
}

type priceChainStatsJSON struct {
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

func TestPrice_MissingIDsSlugSymbol(t *testing.T) {
	_, _, err := executeCommand(t, "price", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide --id, --slug, or --symbol")
}

func TestPrice_ByID_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
		assert.Equal(t, "USD", r.URL.Query().Get("convert"))

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
				"1027": {
					ID:     1027,
					Name:   "Ethereum",
					Symbol: "ETH",
					Quote: map[string]api.Quote{
						"USD": {Price: 3400, PercentChange24h: -1.2},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1,1027", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
	assert.Equal(t, 50000.0, quotes["1"].Quote["USD"].Price)
}

func TestPrice_BySlug_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Equal(t, "bitcoin,ethereum", r.URL.Query().Get("slug"))

		resp := api.QuotesLatestResponse{
			Data: map[string]api.QuoteCoin{
				"1": {
					ID:     1,
					Name:   "Bitcoin",
					Symbol: "BTC",
					Slug:   "bitcoin",
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

	stdout, _, err := executeCommand(t, "price", "--slug", "bitcoin,ethereum", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
}

func TestPrice_ByID_WithInfo_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Slug:   "ethereum",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
			resp := api.InfoResponse{
				Data: map[string]api.CoinInfo{
					"1": {
						ID:          1,
						Name:        "Bitcoin",
						Symbol:      "BTC",
						Slug:        "bitcoin",
						Description: "Bitcoin is a decentralized digital currency.",
						Tags:        []string{"mineable", "pow"},
						URLs: api.CoinInfoURLs{
							Website:  []string{"https://bitcoin.org"},
							Explorer: []string{"https://blockchair.com/bitcoin"},
						},
					},
					"1027": {
						ID:          1027,
						Name:        "Ethereum",
						Symbol:      "ETH",
						Slug:        "ethereum",
						Description: "Ethereum is a smart contract platform.",
						Tags:        []string{"smart-contracts", "defi"},
						URLs: api.CoinInfoURLs{
							Website:      []string{"https://ethereum.org"},
							TechnicalDoc: []string{"https://ethereum.org/en/whitepaper/"},
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
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1,1027", "--with-info", "-o", "json")
	require.NoError(t, err)

	var got map[string]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	require.Len(t, got, 2)
	require.NotNil(t, got["1"].Info)
	assert.Equal(t, "Bitcoin", got["1"].Name)
	assert.Equal(t, "Bitcoin is a decentralized digital currency.", got["1"].Info.Description)
	assert.Equal(t, []string{"mineable", "pow"}, got["1"].Info.Tags)
	assert.Equal(t, []string{"https://bitcoin.org"}, got["1"].Info.URLs.Website)
	assert.Equal(t, []string{"https://blockchair.com/bitcoin"}, got["1"].Info.URLs.Explorer)
	require.NotNil(t, got["1027"].Info)
	assert.Equal(t, []string{"https://ethereum.org"}, got["1027"].Info.URLs.Website)
	assert.Equal(t, []string{"https://ethereum.org/en/whitepaper/"}, got["1027"].Info.URLs.TechnicalDoc)
}

func TestPrice_BySlug_WithInfo_TableOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "bitcoin,ethereum", r.URL.Query().Get("slug"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Slug:   "ethereum",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
			resp := api.InfoResponse{
				Data: map[string]api.CoinInfo{
					"1": {
						ID:          1,
						Name:        "Bitcoin",
						Symbol:      "BTC",
						Slug:        "bitcoin",
						Description: "Bitcoin is a decentralized digital currency.",
						Tags:        []string{"mineable", "pow"},
						URLs: api.CoinInfoURLs{
							Website: []string{"https://bitcoin.org"},
						},
					},
					"1027": {
						ID:          1027,
						Name:        "Ethereum",
						Symbol:      "ETH",
						Slug:        "ethereum",
						Description: "Ethereum is a smart contract platform.",
						Tags:        []string{"smart-contracts", "defi"},
						URLs: api.CoinInfoURLs{
							Website: []string{"https://ethereum.org"},
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
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "price", "--slug", "bitcoin,ethereum", "--with-info")
	require.NoError(t, err)

	assert.Contains(t, stderr, "CoinMarketCap CLI")
	assert.Contains(t, stdout, "Bitcoin")
	assert.Contains(t, stdout, "Description:")
	assert.Contains(t, stdout, "Bitcoin is a decentralized digital currency.")
	assert.Contains(t, stdout, "Tags:")
	assert.Contains(t, stdout, "mineable")
	assert.Contains(t, stdout, "URLs:")
	assert.Contains(t, stdout, "https://bitcoin.org")
	assert.Contains(t, stdout, "Ethereum is a smart contract platform.")
}

func TestPrice_ByID_WithChainStats_TableOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v1/blockchain/statistics/latest":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"1": map[string]any{
						"id":                    1,
						"slug":                  "bitcoin",
						"symbol":                "BTC",
						"block_reward_static":   "3.125",
						"consensus_mechanism":   "proof-of-work",
						"difficulty":            "11890594958796",
						"hashrate_24h":          "85116194130018810000",
						"pending_transactions":  "1177",
						"reduction_rate":        "50%",
						"total_blocks":          "595165",
						"total_transactions":    "455738994",
						"tps_24h":               "3.808090277777778",
						"first_block_timestamp": "2009-01-09T02:54:25.000Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "price", "--id", "1", "--with-chain-stats")
	require.NoError(t, err)

	assert.Contains(t, stderr, "CoinMarketCap CLI")
	assert.Contains(t, stdout, "Bitcoin")
	assert.Contains(t, stdout, "Chain stats:")
	assert.Contains(t, stdout, "Consensus mechanism:")
	assert.Contains(t, stdout, "proof-of-work")
	assert.Contains(t, stdout, "Hashrate 24h:")
}

func TestPrice_ByID_WithChainStats_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Slug:   "ethereum",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v1/blockchain/statistics/latest":
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"1": map[string]any{
						"id":                    1,
						"slug":                  "bitcoin",
						"symbol":                "BTC",
						"block_reward_static":   "3.125",
						"consensus_mechanism":   "proof-of-work",
						"difficulty":            "11890594958796",
						"hashrate_24h":          "85116194130018810000",
						"pending_transactions":  "1177",
						"reduction_rate":        "50%",
						"total_blocks":          "595165",
						"total_transactions":    "455738994",
						"tps_24h":               "3.808090277777778",
						"first_block_timestamp": "2009-01-09T02:54:25.000Z",
					},
					"1027": map[string]any{
						"id":                    1027,
						"slug":                  "ethereum",
						"symbol":                "ETH",
						"block_reward_static":   "2.0",
						"consensus_mechanism":   "proof-of-stake",
						"difficulty":            "0",
						"hashrate_24h":          "0",
						"pending_transactions":  "42",
						"reduction_rate":        "0%",
						"total_blocks":          "22000000",
						"total_transactions":    "2000000000",
						"tps_24h":               "12.5",
						"first_block_timestamp": "2015-07-30T15:26:13.000Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1,1027", "--with-chain-stats", "-o", "json")
	require.NoError(t, err)

	var got map[string]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	require.NotNil(t, got["1"].ChainStats)
	assert.Equal(t, "Bitcoin", got["1"].Name)
	assert.Equal(t, "proof-of-work", got["1"].ChainStats.ConsensusMechanism)
	assert.Equal(t, "85116194130018810000", got["1"].ChainStats.Hashrate24h)
	assert.Equal(t, "Ethereum", got["1027"].Name)
	assert.Equal(t, "proof-of-stake", got["1027"].ChainStats.ConsensusMechanism)
}

func TestPrice_ByID_WithInfoAndChainStats_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			resp := api.InfoResponse{
				Data: map[string]api.CoinInfo{
					"1": {
						ID:          1,
						Name:        "Bitcoin",
						Symbol:      "BTC",
						Slug:        "bitcoin",
						Description: "Bitcoin is a decentralized digital currency.",
						Tags:        []string{"mineable"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v1/blockchain/statistics/latest":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"1": map[string]any{
						"id":                    1,
						"slug":                  "bitcoin",
						"symbol":                "BTC",
						"block_reward_static":   "3.125",
						"consensus_mechanism":   "proof-of-work",
						"difficulty":            "11890594958796",
						"hashrate_24h":          "85116194130018810000",
						"pending_transactions":  "1177",
						"reduction_rate":        "50%",
						"total_blocks":          "595165",
						"total_transactions":    "455738994",
						"tps_24h":               "3.808090277777778",
						"first_block_timestamp": "2009-01-09T02:54:25.000Z",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1", "--with-info", "--with-chain-stats", "-o", "json")
	require.NoError(t, err)

	var got map[string]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	require.NotNil(t, got["1"].Info)
	require.NotNil(t, got["1"].ChainStats)
	assert.Equal(t, "Bitcoin is a decentralized digital currency.", got["1"].Info.Description)
	assert.Equal(t, "proof-of-work", got["1"].ChainStats.ConsensusMechanism)
}

func TestPrice_BySymbol_JSONOutput_ReturnsRankedCandidates(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/cryptocurrency/map" {
			assert.Equal(t, "BTC", r.URL.Query().Get("symbol"))
			resp := map[string]any{
				"data": []map[string]any{
					{"id": 12345, "name": "Bitcoin Cash", "symbol": "BTC", "slug": "bitcoin-cash", "rank": 10, "is_active": 1},
					{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
			assert.Equal(t, "1,12345", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"12345": {
						ID:     12345,
						Name:   "Bitcoin Cash",
						Symbol: "BTC",
						Slug:   "bitcoin-cash",
						Quote: map[string]api.Quote{
							"USD": {Price: 350.25, PercentChange24h: -1.1},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "-o", "json")
	require.NoError(t, err)

	var got map[string][]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	require.Len(t, got, 1)
	require.Len(t, got["BTC"], 2)
	assert.Equal(t, int64(1), got["BTC"][0].ID)
	assert.Equal(t, int64(12345), got["BTC"][1].ID)
	assert.Equal(t, "Bitcoin", got["BTC"][0].Name)
	assert.Equal(t, "Bitcoin Cash", got["BTC"][1].Name)
}

func TestPrice_BySymbol_MultipleSymbols_JSONOutput(t *testing.T) {
	mapCalls := make([]string, 0, 2)
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/cryptocurrency/map" {
			symbol := r.URL.Query().Get("symbol")
			mapCalls = append(mapCalls, symbol)
			switch symbol {
			case "BTC":
				resp := map[string]any{
					"data": []map[string]any{
						{"id": 12345, "name": "Bitcoin Cash", "symbol": "BTC", "slug": "bitcoin-cash", "rank": 10, "is_active": 1},
						{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			case "ETH":
				resp := map[string]any{
					"data": []map[string]any{
						{"id": 1027, "name": "Ethereum", "symbol": "ETH", "slug": "ethereum", "rank": 2, "is_active": 1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			default:
				t.Fatalf("unexpected resolver symbol: %s", symbol)
			}
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
			assert.Equal(t, "1,12345,1027", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"12345": {
						ID:     12345,
						Name:   "Bitcoin Cash",
						Symbol: "BTC",
						Slug:   "bitcoin-cash",
						Quote: map[string]api.Quote{
							"USD": {Price: 350.25, PercentChange24h: -1.1},
						},
					},
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Slug:   "ethereum",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
			t.Fatalf("unexpected quotes request: %s", r.URL.RawQuery)
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC,ETH", "-o", "json")
	require.NoError(t, err)

	var got map[string][]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	assert.Equal(t, []string{"BTC", "ETH"}, mapCalls)
	require.Len(t, got, 2)
	require.Len(t, got["BTC"], 2)
	require.Len(t, got["ETH"], 1)
	assert.Equal(t, int64(1), got["BTC"][0].ID)
	assert.Equal(t, int64(12345), got["BTC"][1].ID)
	assert.Equal(t, int64(1027), got["ETH"][0].ID)
}

func TestPrice_BySymbol_JSONOutput_CapsCandidatesAt10(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/cryptocurrency/map":
			assets := make([]map[string]any, 0, 12)
			for i := 1; i <= 12; i++ {
				assets = append(assets, map[string]any{
					"id":        i,
					"name":      "Bitcoin Variant",
					"symbol":    "BTC",
					"slug":      "bitcoin-variant",
					"rank":      i,
					"is_active": 1,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": assets})
		case "/v3/cryptocurrency/quotes/latest":
			ids := strings.Split(r.URL.Query().Get("id"), ",")
			require.Len(t, ids, 10)
			assert.Equal(t, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}, ids)
			data := make(map[string]api.QuoteCoin, 10)
			for i := 1; i <= 10; i++ {
				id := int64(i)
				data[strconv.Itoa(i)] = api.QuoteCoin{
					ID:     id,
					Name:   "Bitcoin Variant",
					Symbol: "BTC",
					Slug:   "bitcoin-variant",
					Quote: map[string]api.Quote{
						"USD": {Price: float64(1000 - i), PercentChange24h: float64(i)},
					},
				}
			}
			_ = json.NewEncoder(w).Encode(api.QuotesLatestResponse{Data: data})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "-o", "json")
	require.NoError(t, err)

	var got map[string][]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	require.Len(t, got["BTC"], 10)
}

func TestPrice_BySymbol_WithInfoAndChainStats_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/cryptocurrency/map":
			assert.Equal(t, "BTC", r.URL.Query().Get("symbol"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": 12345, "name": "Bitcoin Cash", "symbol": "BTC", "slug": "bitcoin-cash", "rank": 10, "is_active": 1},
					{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
				},
			})
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1,12345", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
						},
					},
					"12345": {
						ID:     12345,
						Name:   "Bitcoin Cash",
						Symbol: "BTC",
						Slug:   "bitcoin-cash",
						Quote: map[string]api.Quote{
							"USD": {Price: 350.25, PercentChange24h: -1.1},
						},
					},
				},
			})
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "1,12345", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(api.InfoResponse{
				Data: map[string]api.CoinInfo{
					"1": {
						ID:          1,
						Name:        "Bitcoin",
						Symbol:      "BTC",
						Slug:        "bitcoin",
						Description: "Bitcoin description.",
						Tags:        []string{"mineable"},
					},
					"12345": {
						ID:          12345,
						Name:        "Bitcoin Cash",
						Symbol:      "BTC",
						Slug:        "bitcoin-cash",
						Description: "Bitcoin Cash description.",
						Tags:        []string{"fork"},
					},
				},
			})
		case "/v1/blockchain/statistics/latest":
			assert.Equal(t, "1,12345", r.URL.Query().Get("id"))
			_ = json.NewEncoder(w).Encode(api.BlockchainStatisticsLatestResponse{
				Data: map[string]api.BlockchainStatistics{
					"1": {
						ID:                 1,
						Slug:               "bitcoin",
						Symbol:             "BTC",
						BlockRewardStatic:  "3.125",
						ConsensusMechanism: "proof-of-work",
					},
					"12345": {
						ID:                 12345,
						Slug:               "bitcoin-cash",
						Symbol:             "BTC",
						BlockRewardStatic:  "6.25",
						ConsensusMechanism: "proof-of-work",
					},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "--with-info", "--with-chain-stats", "-o", "json")
	require.NoError(t, err)

	var got map[string][]priceWithInfoJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))
	require.Len(t, got["BTC"], 2)
	require.NotNil(t, got["BTC"][0].Info)
	require.NotNil(t, got["BTC"][0].ChainStats)
	require.NotNil(t, got["BTC"][1].Info)
	require.NotNil(t, got["BTC"][1].ChainStats)
	assert.Equal(t, "Bitcoin description.", got["BTC"][0].Info.Description)
	assert.Equal(t, "proof-of-work", got["BTC"][0].ChainStats.ConsensusMechanism)
}

func TestPrice_PositionalSymbolShorthand(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/cryptocurrency/map" {
			assert.Equal(t, "BTC", r.URL.Query().Get("symbol"))
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
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
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "btc", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
}

func TestPrice_PositionalSymbolShorthandMultiple(t *testing.T) {
	mapCalls := make([]string, 0, 2)
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/cryptocurrency/map" {
			symbol := r.URL.Query().Get("symbol")
			mapCalls = append(mapCalls, symbol)
			switch symbol {
			case "BTC":
				resp := map[string]interface{}{
					"data": []map[string]interface{}{
						{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			case "ETH":
				resp := map[string]interface{}{
					"data": []map[string]interface{}{
						{"id": 1027, "name": "Ethereum", "symbol": "ETH", "slug": "ethereum", "rank": 2, "is_active": 1},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			default:
				t.Fatalf("unexpected resolver symbol: %s", symbol)
			}
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
			assert.Equal(t, "1,1027", r.URL.Query().Get("id"))
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
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "btc", "eth", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, []string{"BTC", "ETH"}, mapCalls)
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
	assert.Equal(t, "Ethereum", quotes["1027"].Name)
}

func TestPrice_PositionalSymbolShorthandAmbiguousChoosesHighestRankedWithWarning(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/cryptocurrency/map" {
			assert.Equal(t, "BTC", r.URL.Query().Get("symbol"))
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": 12345, "name": "Bitcoin Clone", "symbol": "BTC", "slug": "bitcoin-clone", "rank": 250, "is_active": 1},
					{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin", "rank": 1, "is_active": 1},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v3/cryptocurrency/quotes/latest" {
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
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "price", "btc", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
	assert.Contains(t, stderr, `Warning: symbol "BTC" matched multiple assets`)
	assert.Contains(t, stderr, "selected top-ranked candidate Bitcoin")
}

func TestPrice_PositionalIDShorthand(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		assert.Equal(t, "1,1027", r.URL.Query().Get("id"))

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
				"1027": {
					ID:     1027,
					Name:   "Ethereum",
					Symbol: "ETH",
					Quote: map[string]api.Quote{
						"USD": {Price: 3400, PercentChange24h: -1.2},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "1", "1027", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
	assert.Equal(t, "Ethereum", quotes["1027"].Name)
}

func TestPrice_PositionalShorthandMixedTypes_Fails(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP calls for mixed shorthand")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "price", "1", "btc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mixed positional shorthand")
}

func TestPrice_CannotCombineIDFlagWithPositionalShorthand(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP calls when flags conflict with positional shorthand")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "price", "--id", "1", "btc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positional shorthand cannot be combined with --id, --slug, or --symbol")
}

func TestPrice_MixedSelectorFamiliesFailClosed(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mixed selectors should fail before any HTTP request")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	cases := []struct {
		name string
		args []string
	}{
		{name: "id and symbol", args: []string{"price", "--id", "1", "--symbol", "BTC", "-o", "json"}},
		{name: "slug and symbol", args: []string{"price", "--slug", "bitcoin", "--symbol", "BTC", "-o", "json"}},
		{name: "id and slug", args: []string{"price", "--id", "1", "--slug", "bitcoin", "-o", "json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := executeCommand(t, tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "provide exactly one of --id, --slug, or --symbol")
		})
	}
}

func TestPrice_HelpDocumentsPositionalSymbolAutoPick(t *testing.T) {
	stdout, _, err := executeCommand(t, "price", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Explicit --symbol returns up to 10 best-ranked matches per symbol.")
	assert.Contains(t, stdout, "Positional shorthand: all-digit args are treated as CMC IDs")
	assert.Contains(t, stdout, "tokens with length >= 5 or a hyphen try slug first, then symbol, regardless of casing")
}

func TestPrice_PositionalSlugFirstBitcoin(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "bitcoin", r.URL.Query().Get("slug"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"1": map[string]any{"id": 1, "name": "Bitcoin", "symbol": "BTC", "slug": "bitcoin"},
				},
			})
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1": {
						ID:     1,
						Name:   "Bitcoin",
						Symbol: "BTC",
						Slug:   "bitcoin",
						Quote: map[string]api.Quote{
							"USD": {Price: 50000, PercentChange24h: 2.5},
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
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "bitcoin", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bitcoin", quotes["1"].Name)
	assert.Equal(t, "BTC", quotes["1"].Symbol)
}

func TestPrice_PositionalSlugFirstEthereum(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "ethereum", r.URL.Query().Get("slug"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"1027": map[string]any{"id": 1027, "name": "Ethereum", "symbol": "ETH", "slug": "ethereum"},
				},
			})
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "1027", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"1027": {
						ID:     1027,
						Name:   "Ethereum",
						Symbol: "ETH",
						Slug:   "ethereum",
						Quote: map[string]api.Quote{
							"USD": {Price: 3400, PercentChange24h: -1.2},
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
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "ethereum", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Ethereum", quotes["1027"].Name)
	assert.Equal(t, "ETH", quotes["1027"].Symbol)
}

func TestPrice_PositionalSlugFirstUppercaseLongWord(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			assert.Equal(t, "bittensor", r.URL.Query().Get("slug"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"23036": map[string]any{"id": 23036, "name": "Bittensor", "symbol": "TAO", "slug": "bittensor"},
				},
			})
		case "/v3/cryptocurrency/quotes/latest":
			assert.Equal(t, "23036", r.URL.Query().Get("id"))
			resp := api.QuotesLatestResponse{
				Data: map[string]api.QuoteCoin{
					"23036": {
						ID:     23036,
						Name:   "Bittensor",
						Symbol: "TAO",
						Slug:   "bittensor",
						Quote: map[string]api.Quote{
							"USD": {Price: 250.12, PercentChange24h: 3.4},
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
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "BITTENSOR", "-o", "json")
	require.NoError(t, err)

	var quotes map[string]api.QuoteCoin
	require.NoError(t, json.Unmarshal([]byte(stdout), &quotes))
	assert.Equal(t, "Bittensor", quotes["23036"].Name)
	assert.Equal(t, "TAO", quotes["23036"].Symbol)
}

func TestPrice_NoResults(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// API returns empty response
		resp := api.QuotesLatestResponse{Data: map[string]api.QuoteCoin{}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "price", "--id", "999999", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid coins found")
}

func TestPrice_DryRun_ByID(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Equal(t, "1", out.Params["id"])
	assert.Equal(t, "USD", out.Params["convert"])
	assert.Contains(t, out.URL, "/v3/cryptocurrency/quotes/latest")
	assert.NotEmpty(t, out.OASOperationID, "dry-run JSON must include oas_operation_id for scripting contract")
	assert.Equal(t, "getV3CryptocurrencyQuotesLatest", out.OASOperationID)
	assert.NotEmpty(t, out.OASSpec, "dry-run JSON must include oas_spec for scripting contract")
	assert.Equal(t, spec.SourceURL, out.OASSpec)
}

func TestPrice_DryRun_WithInfo_ByID(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("with-info dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1,1027", "--with-info", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan []dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Len(t, plan, 2)
	assert.Equal(t, "/v3/cryptocurrency/quotes/latest", plan[0].URL[len(plan[0].URL)-len("/v3/cryptocurrency/quotes/latest"):])
	assert.Equal(t, "1,1027", plan[0].Params["id"])
	assert.Equal(t, "/v2/cryptocurrency/info", plan[1].URL[len(plan[1].URL)-len("/v2/cryptocurrency/info"):])
	assert.Equal(t, "1,1027", plan[1].Params["id"])
}

func TestPrice_DryRun_WithInfo_ExplicitSymbol(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("with-info dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "--with-info", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "explicit_symbol_with_info", plan.Mode)
	assert.Equal(t, []string{"BTC"}, plan.Inputs)
	require.Len(t, plan.Steps, 3)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BTC", plan.Steps[0].Request.Params["symbol"])
	assert.Equal(t, "fetch_quotes", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[1].Request.Params["id"])
	assert.Equal(t, "fetch_info", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[2].Request.Params["id"])
}

func TestPrice_DryRun_ExplicitSymbol(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("explicit-symbol dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "explicit_symbol", plan.Mode)
	require.Len(t, plan.Steps, 2)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BTC", plan.Steps[0].Request.Params["symbol"])
	assert.Contains(t, plan.Steps[0].Condition, "ranked candidates")
	assert.Equal(t, "fetch_quotes", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[1].Request.Params["id"])
}

func TestPrice_DryRun_WithChainStats_ByID(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("with-chain-stats dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1,1027", "--with-chain-stats", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan []dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Len(t, plan, 2)
	assert.Contains(t, plan[0].URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "1,1027", plan[0].Params["id"])
	assert.Contains(t, plan[1].URL, "/v1/blockchain/statistics/latest")
	assert.Equal(t, "1,1027", plan[1].Params["id"])
}

func TestPrice_DryRun_WithInfoAndChainStats_ByID(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("with-info and with-chain-stats dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--id", "1", "--with-info", "--with-chain-stats", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan []dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Len(t, plan, 3)
	assert.Contains(t, plan[0].URL, "/v3/cryptocurrency/quotes/latest")
	assert.Contains(t, plan[1].URL, "/v2/cryptocurrency/info")
	assert.Contains(t, plan[2].URL, "/v1/blockchain/statistics/latest")
}

func TestPrice_DryRun_ExplicitSymbol_WithChainStats(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("with-chain-stats dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "--with-chain-stats", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "explicit_symbol_with_chain_stats", plan.Mode)
	require.Len(t, plan.Steps, 3)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BTC", plan.Steps[0].Request.Params["symbol"])
	assert.Equal(t, "fetch_quotes", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "fetch_chain_stats", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v1/blockchain/statistics/latest")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[2].Request.Params["id"])
}

func TestPrice_DryRun_ExplicitSymbol_WithInfoAndChainStats(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("combined dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "--symbol", "BTC", "--with-info", "--with-chain-stats", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "explicit_symbol_with_info_and_chain_stats", plan.Mode)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Equal(t, "fetch_quotes", plan.Steps[1].Stage)
	assert.Equal(t, "fetch_info", plan.Steps[2].Stage)
	assert.Equal(t, "fetch_chain_stats", plan.Steps[3].Stage)
}

func TestPrice_PositionalDryRun_SymbolFirstBTC_WithChainStats(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("positional dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "btc", "--with-chain-stats", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "positional_shorthand", plan.Mode)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BTC", plan.Steps[0].Request.Params["symbol"])
	assert.Equal(t, "resolve_fallback", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "btc", plan.Steps[1].Request.Params["slug"])
	assert.Equal(t, "fetch_quotes", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "fetch_chain_stats", plan.Steps[3].Stage)
	assert.Contains(t, plan.Steps[3].Request.URL, "/v1/blockchain/statistics/latest")
}

func TestPrice_PositionalDryRun_SlugFirstBitcoin(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("positional dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "bitcoin", "--with-info", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "price", plan.Command)
	assert.Equal(t, "positional_shorthand", plan.Mode)
	assert.Equal(t, []string{"bitcoin"}, plan.Inputs)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "bitcoin", plan.Steps[0].Request.Params["slug"])
	assert.Equal(t, "resolve_fallback", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BITCOIN", plan.Steps[1].Request.Params["symbol"])
	assert.Equal(t, "fetch_quotes", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[2].Request.Params["id"])
	assert.Equal(t, "USD", plan.Steps[2].Request.Params["convert"])
	assert.Equal(t, "fetch_info", plan.Steps[3].Stage)
	assert.Contains(t, plan.Steps[3].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[3].Request.Params["id"])
}

func TestPrice_PositionalDryRun_SlugFirstUppercaseLongWord(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("positional dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "BITTENSOR", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "positional_shorthand", plan.Mode)
	require.Len(t, plan.Steps, 3)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "bittensor", plan.Steps[0].Request.Params["slug"])
	assert.Equal(t, "resolve_fallback", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BITTENSOR", plan.Steps[1].Request.Params["symbol"])
	assert.Equal(t, "fetch_quotes", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v3/cryptocurrency/quotes/latest")
}

func TestPrice_PositionalDryRun_SymbolFirstBTC(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("positional dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "price", "btc", "--with-info", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var plan pricePositionalDryRunPlan
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	assert.Equal(t, "price", plan.Command)
	assert.Equal(t, "positional_shorthand", plan.Mode)
	assert.Equal(t, []string{"btc"}, plan.Inputs)
	require.Len(t, plan.Steps, 4)
	assert.Equal(t, "resolve_primary", plan.Steps[0].Stage)
	assert.Contains(t, plan.Steps[0].Request.URL, "/v1/cryptocurrency/map")
	assert.Equal(t, "BTC", plan.Steps[0].Request.Params["symbol"])
	assert.Equal(t, "resolve_fallback", plan.Steps[1].Stage)
	assert.Contains(t, plan.Steps[1].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "btc", plan.Steps[1].Request.Params["slug"])
	assert.Equal(t, "fetch_quotes", plan.Steps[2].Stage)
	assert.Contains(t, plan.Steps[2].Request.URL, "/v3/cryptocurrency/quotes/latest")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[2].Request.Params["id"])
	assert.Equal(t, "fetch_info", plan.Steps[3].Stage)
	assert.Contains(t, plan.Steps[3].Request.URL, "/v2/cryptocurrency/info")
	assert.Equal(t, "<resolved asset ids>", plan.Steps[3].Request.Params["id"])
}

func TestPrice_CommandsCatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands", "-o", "json")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var info commandInfo
	found := false
	for _, cmd := range catalog.Commands {
		if cmd.Name == "price" {
			info = cmd
			found = true
			break
		}
	}
	require.True(t, found, "price command should be present in catalog")
	assert.Equal(t, "/v3/cryptocurrency/quotes/latest", info.APIEndpoint)
	assert.Equal(t, "getV3CryptocurrencyQuotesLatest", info.OASOperationID)
	require.NotEmpty(t, info.APIEndpoints, "price command should advertise multi-step API endpoints")
	assert.Equal(t, "/v3/cryptocurrency/quotes/latest", info.APIEndpoints["explicit"])
	assert.Equal(t, "/v2/cryptocurrency/info", info.APIEndpoints["positional_slug_first"])
	assert.Equal(t, "/v1/cryptocurrency/map", info.APIEndpoints["positional_symbol"])
	require.NotEmpty(t, info.OASOperationIDs, "price command should advertise multi-step OAS operation IDs")
	assert.Equal(t, "getV3CryptocurrencyQuotesLatest", info.OASOperationIDs["explicit"])
	assert.Equal(t, "getV2CryptocurrencyInfo", info.OASOperationIDs["positional_slug_first"])
	assert.Equal(t, "getV1CryptocurrencyMap", info.OASOperationIDs["positional_symbol"])
	assert.Contains(t, info.Examples, "cmc price --id 1,1027")
}
