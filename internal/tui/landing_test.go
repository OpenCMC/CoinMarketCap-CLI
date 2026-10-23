package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func newLandingTestClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()

	client := api.NewClientWithHTTP(&config.Config{APIKey: "test-key", Tier: config.TierEnterprise}, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			return recorder.Result(), nil
		}),
	})
	client.SetBaseURL("https://cmc.test")
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLandingViewRendersBrandAndEntries(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.hero = landingHeroCard{
		Available: true,
		Name:      "CoinMarketCap 20",
		Symbol:    "CMC20",
		Price:     20.25,
		Change24h: 1.8,
		Change7d:  -0.4,
		Has7d:     true,
		Volume24h: 123456789,
	}

	view := m.View()
	require.Contains(t, view, "CoinMarketCap")
	require.Contains(t, view, "CMC20")
	require.Contains(t, view, "Top 50")
	require.Contains(t, view, "Trending 50")
}

func TestLandingViewRendersLargeLogoBlock(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.width = 160
	m.height = 48
	m.hero = landingHeroCard{
		Available: true,
		Name:      "CoinMarketCap 20",
		Symbol:    "CMC20",
		Price:     20.25,
		Change24h: 1.8,
		Change7d:  -0.4,
		Has7d:     true,
		Volume24h: 123456789,
		Chart:     sampleLandingChart(),
	}

	view := m.View()
	require.Greater(t, strings.Count(view, "█"), 80)
	require.Contains(t, view, "Professional data for AI agents and terminal workflows")
}

func TestLandingViewRendersCompactLogoFallback(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.width = 92
	m.height = 40
	m.hero = landingHeroCard{
		Available: true,
		Name:      "CMC20",
		Symbol:    "CMC20",
		Price:     20.25,
		Change24h: 1.8,
		Change7d:  -0.4,
		Has7d:     true,
		Volume24h: 123456789,
	}

	view := m.View()
	require.Contains(t, view, "COINMARKETCAP")
	require.NotContains(t, view, "█████")
}

func TestLandingViewRendersChartCard(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.width = 160
	m.height = 48
	m.hero = landingHeroCard{
		Available: true,
		Name:      "CoinMarketCap 20",
		Symbol:    "CMC20",
		Price:     20.25,
		Change24h: 1.8,
		Change7d:  -0.4,
		Has7d:     true,
		Volume24h: 123456789,
		Chart:     sampleLandingChart(),
	}

	view := m.View()
	require.Contains(t, view, "CMC20 7D")
	require.Contains(t, view, "0d")
	require.Contains(t, view, "7d")
	require.NotContains(t, view, "No chart data available")
}

func TestLandingInitialViewShowsLoadingState(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	view := m.View()
	require.Contains(t, view, "Loading CMC20")
	require.NotContains(t, view, "CMC20 unavailable")
}

func TestLandingEnterOpensMarketsAndTrending(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.width = 132
	m.height = 44

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	lm := updated.(LandingModel)
	require.Equal(t, landingMarkets, lm.state)
	require.Equal(t, closeToLanding, lm.markets.close)
	require.Equal(t, 50, lm.markets.total)
	require.Equal(t, 132, lm.markets.width)
	require.Equal(t, 44, lm.markets.height)
	require.NotNil(t, cmd)

	m.selection = 1
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	lm = updated.(LandingModel)
	require.Equal(t, landingTrending, lm.state)
	require.Equal(t, closeToLanding, lm.trending.close)
	require.Equal(t, 50, lm.trending.limit)
	require.Equal(t, 132, lm.trending.width)
	require.Equal(t, 44, lm.trending.height)
	require.NotNil(t, cmd)
}

func TestLandingEscFromMarketsDetailReturnsToLanding(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewMarketsModel(client, "USD", 50, closeToLanding)
	m.state = marketsDetail
	m.detail = DetailModel{vs: "USD"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(MarketsModel)
	require.True(t, mm.ReturnToLanding)
	require.False(t, mm.state == marketsLoaded)
	require.Nil(t, cmd)
}

func TestLandingEscFromTrendingDetailReturnsToLanding(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewTrendingModel(client, "USD", closeToLanding)
	m.state = trendingDetail
	m.detail = DetailModel{vs: "USD"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm := updated.(TrendingModel)
	require.True(t, tm.ReturnToLanding)
	require.False(t, tm.state == trendingLoaded)
	require.Nil(t, cmd)
}

func TestDirectDeepLinkEscQuits(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewMarketsModel(client, "USD", 50)
	m.state = marketsLoaded

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm := updated.(MarketsModel)
	require.False(t, mm.ReturnToLanding)
	require.Nil(t, mm.err)
	require.NotNil(t, cmd)

	tm := NewTrendingModel(client, "USD")
	tm.state = trendingLoaded
	updated, cmd = tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updatedTrending := updated.(TrendingModel)
	require.False(t, updatedTrending.ReturnToLanding)
	require.NotNil(t, cmd)
}

func TestTrendingDefaultLimitIsFifty(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewTrendingModel(client, "USD")
	require.Equal(t, 50, m.limit)
}

func TestLandingHeroDegradedState(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	m := NewLandingModel(client)
	card, err := m.loadHero(context.Background())
	require.Error(t, err)
	require.False(t, card.Available)
	require.True(t, strings.Contains(card.Error, "500") || strings.Contains(card.Error, "boom"))

	m.hero = card
	m.heroErr = err
	view := m.View()
	require.Contains(t, view, "CMC20 unavailable")
}

func TestLandingViewShowsChartFallbackWhenHistoryMissing(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	m := NewLandingModel(client)
	m.width = 160
	m.height = 48
	m.hero = landingHeroCard{
		Available: true,
		Name:      "CoinMarketCap 20",
		Symbol:    "CMC20",
		Price:     20.25,
		Change24h: 1.8,
		Volume24h: 123456789,
		ChartNote: "history unavailable",
	}

	view := m.View()
	require.Contains(t, view, "No chart data available")
	require.Contains(t, view, "history unavailable")
}

func sampleLandingChart() api.OHLCData {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	out := make(api.OHLCData, 0, 7)
	for i := 0; i < 7; i++ {
		ts := now.Add(time.Duration(i) * 24 * time.Hour).UnixMilli()
		price := 140.0 + float64(i*2)
		out = append(out, []float64{
			float64(ts),
			price,
			price + 1,
			price - 1,
			price,
		})
	}
	return out
}

func TestLandingChartDataLoadUsesHistory(t *testing.T) {
	historyCall := 0
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			if _, err := fmt.Fprint(w, `{"data":{"coinmarketcap-20-index":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index"}}}`); err != nil {
				t.Errorf("write info response: %v", err)
			}
		case "/v3/cryptocurrency/quotes/latest":
			if _, err := fmt.Fprint(w, `{"data":{"1":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index","quote":{"USD":{"price":140.76,"volume_24h":6740000,"market_cap":0,"percent_change_24h":-0.67}}}}}`); err != nil {
				t.Errorf("write latest quote response: %v", err)
			}
		case "/v1/cryptocurrency/quotes/historical":
			historyCall++
			require.Equal(t, "daily", r.URL.Query().Get("interval"))
			require.Equal(t, "8", r.URL.Query().Get("count"))
			if _, err := fmt.Fprint(w, `{"data":{"1":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index","quotes":[{"timestamp":"2026-03-16T00:00:00Z","quote":{"USD":{"price":130,"market_cap":0,"volume_24h":0}}},{"timestamp":"2026-03-17T00:00:00Z","quote":{"USD":{"price":135,"market_cap":0,"volume_24h":0}}}]}}}`); err != nil {
				t.Errorf("write historical quote response: %v", err)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	m := NewLandingModel(client)
	card, err := m.loadHero(context.Background())
	require.NoError(t, err)
	require.True(t, card.Available)
	require.Len(t, card.Chart, 2)
	require.Equal(t, 1, historyCall)
	require.True(t, card.Has7d)
}

func TestLandingChartFallbackUsesFriendlyMessage(t *testing.T) {
	client := newLandingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/cryptocurrency/info":
			if _, err := fmt.Fprint(w, `{"data":{"coinmarketcap-20-index":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index"}}}`); err != nil {
				t.Errorf("write info response: %v", err)
			}
		case "/v3/cryptocurrency/quotes/latest":
			if _, err := fmt.Fprint(w, `{"data":{"1":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index","quote":{"USD":{"price":140.76,"volume_24h":6740000,"market_cap":0,"percent_change_24h":-0.67}}}}}`); err != nil {
				t.Errorf("write latest quote response: %v", err)
			}
		case "/v1/cryptocurrency/quotes/historical":
			http.Error(w, "upstream unavailable", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	m := NewLandingModel(client)
	card, err := m.loadHero(context.Background())
	require.NoError(t, err)
	require.True(t, card.Available)
	require.Empty(t, card.Chart)
	require.Equal(t, "history unavailable", card.ChartNote)
}

func TestLandingChartKeylessExplainsKeyRequirement(t *testing.T) {
	client := api.NewClientWithHTTP(&config.Config{}, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			switch req.URL.Path {
			case "/public-api/v2/cryptocurrency/info":
				_, _ = fmt.Fprint(recorder, `{"data":{"coinmarketcap-20-index":{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","slug":"coinmarketcap-20-index"}}}`)
			case "/public-api/v3/cryptocurrency/quotes/latest":
				_, _ = fmt.Fprint(recorder, `{"data":[{"id":1,"name":"CoinMarketCap 20","symbol":"CMC20","quote":[{"symbol":"USD","price":140.76}]}]}`)
			default:
				t.Errorf("unexpected keyless request: %s", req.URL.Path)
			}
			return recorder.Result(), nil
		}),
	})
	client.SetBaseURL("https://cmc.test")

	card, err := NewLandingModel(client).loadHero(context.Background())
	require.NoError(t, err)
	require.True(t, card.Available)
	require.Empty(t, card.Chart)
	require.Equal(t, "7D history needs a free API key (cmc auth)", card.ChartNote)
}
