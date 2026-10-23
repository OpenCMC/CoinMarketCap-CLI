package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keylessTestClient returns a client with no API key pointed at srv, recording
// backoff waits instead of sleeping.
func keylessTestClient(srv *httptest.Server) (*Client, *[]time.Duration) {
	c := NewClient(&config.Config{})
	c.SetBaseURL(srv.URL)
	var waits []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	return c, &waits
}

const v3QuotesBody = `{"data":[{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":[{"id":2781,"symbol":"USD","price":81315.05,"percent_change_24h":-2.41,"volume_24h":33023294185.96,"market_cap":1634604147340.88}]}],"status":{"error_code":"0","error_message":""}}`

func TestKeyless_UsesPublicAPIWithoutKeyHeader(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-CMC_PRO_API_KEY")
		_, _ = w.Write([]byte(v3QuotesBody))
	}))
	defer srv.Close()
	c, _ := keylessTestClient(srv)

	quotes, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.NoError(t, err)
	assert.Equal(t, "/public-api/v3/cryptocurrency/quotes/latest", gotPath)
	assert.Empty(t, gotKey)
	require.Contains(t, quotes, "1")
	assert.Equal(t, "BTC", quotes["1"].Symbol)
	assert.InDelta(t, 81315.05, quotes["1"].Quote["USD"].Price, 0.001)
}

func TestKeyless_KeyOnlyEndpointFailsWithoutRequest(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()
	c, _ := keylessTestClient(srv)

	_, err := c.NewsLatest(context.Background(), 1, 10, "en", "all")
	require.ErrorIs(t, err, ErrKeyRequired)
	assert.Contains(t, err.Error(), "/v1/content/latest")
	assert.Zero(t, atomic.LoadInt32(&calls), "key-only endpoints must not be called in keyless mode")
}

func TestKeyless_403KeyRequiredMapsToErrKeyRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":{"error_code":1005,"error_message":"An API Key is required for this call."}}`))
	}))
	defer srv.Close()
	c, _ := keylessTestClient(srv)

	// Not in the spec, so the client sends it and maps the API's answer.
	_, err := c.Raw(context.Background(), http.MethodGet, "/v9/not-in-spec", nil)
	require.ErrorIs(t, err, ErrKeyRequired)
}

func TestKeyless_RetriesRateLimitWithBackoff(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"status":{"error_code":"1022","error_message":"You've reached the limit for anonymous access."}}`))
			return
		}
		_, _ = w.Write([]byte(v3QuotesBody))
	}))
	defer srv.Close()
	c, waits := keylessTestClient(srv)

	_, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, *waits)
}

func TestKeyless_RateLimitGivesUpAfterBudgetWithKeyHint(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c, waits := keylessTestClient(srv)

	_, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.ErrorIs(t, err, ErrRateLimited)
	var rle *RateLimitError
	require.True(t, errors.As(err, &rle))
	assert.True(t, rle.Keyless)
	assert.Contains(t, err.Error(), "cmc auth")
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}, *waits)
	assert.Equal(t, int32(5), atomic.LoadInt32(&calls))
}

func TestKeyless_RetryAfterHeaderIsHonoured(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(v3QuotesBody))
	}))
	defer srv.Close()
	c, waits := keylessTestClient(srv)

	_, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{5 * time.Second}, *waits)
}

func TestKeyed_RateLimitIsNotRetried(t *testing.T) {
	var calls int32
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer srv.Close()

	_, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestKeyed_InvalidKeyDoesNotFallBackToKeyless(t *testing.T) {
	var paths []string
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":{"error_code":"1001","error_message":"This API Key is invalid."}}`))
	})
	defer srv.Close()

	_, err := c.QuotesLatestByID(context.Background(), []string{"1"}, "USD")
	require.ErrorIs(t, err, ErrInvalidAPIKey, "a string error_code must still classify as an auth error")
	assert.Equal(t, []string{"/v3/cryptocurrency/quotes/latest"}, paths, "an invalid key must not be retried keyless")
}

func TestEnvelopeErrorWithHTTP200(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":{"error_code":"500","error_message":"The system is busy, please try again later!"}}`))
	})
	defer srv.Close()

	_, err := c.Raw(context.Background(), http.MethodGet, "/v9/nope", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 500: The system is busy")
}

func TestEnvelopeWithDataIsNotAnError(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"value":48},"status":{"error_code":"0"}}`))
	})
	defer srv.Close()

	raw, err := c.Raw(context.Background(), http.MethodGet, "/v3/fear-and-greed/latest", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"data":{"value":48},"status":{"error_code":"0"}}`, string(raw))
}

func TestRawSendsPOSTBody(t *testing.T) {
	var gotMethod, gotType, gotBody string
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotType = r.Header.Get("Content-Type")
		b := make([]byte, 64)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()

	_, err := c.Raw(context.Background(), http.MethodPost, "/v1/dex/holders/list", []byte(`{"platform":"ethereum"}`))
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, `{"platform":"ethereum"}`, gotBody)
}

func TestListingsLatest_DecodesV3QuoteArray(t *testing.T) {
	var gotPath string
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"id":1027,"name":"Ethereum","symbol":"ETH","slug":"ethereum","cmc_rank":2,"quote":[{"id":2790,"symbol":"EUR","price":2200.5,"market_cap":268000000000}]}],"status":{"error_code":"0"}}`))
	})
	defer srv.Close()

	coins, err := c.ListingsLatest(context.Background(), 1, 1, "EUR", "", "")
	require.NoError(t, err)
	assert.Equal(t, "/v3/cryptocurrency/listings/latest", gotPath)
	require.Len(t, coins, 1)
	assert.Equal(t, 2, coins[0].CMCRank)
	assert.InDelta(t, 2200.5, coins[0].Quote["EUR"].Price, 0.001)
}

func TestListingsLatest_FixesReversedAscendingPage(t *testing.T) {
	page := func(prices ...string) string {
		var items []string
		for i, p := range prices {
			items = append(items, `{"id":`+strconv.Itoa(i+1)+`,"symbol":"C`+strconv.Itoa(i+1)+`","quote":[{"symbol":"USD","price":`+p+`}]}`)
		}
		return `{"data":[` + strings.Join(items, ",") + `]}`
	}
	cases := map[string]struct {
		body string
		dir  string
		want []float64
	}{
		"asc page returned descending is reversed": {page("3", "2", "1"), "asc", []float64{1, 2, 3}},
		"correct asc page is left alone":           {page("1", "2", "3"), "asc", []float64{1, 2, 3}},
		"desc page is never touched":               {page("3", "2", "1"), "desc", []float64{3, 2, 1}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			defer srv.Close()

			coins, err := c.ListingsLatest(context.Background(), 1, 3, "USD", "price", tc.dir)
			require.NoError(t, err)
			var got []float64
			for _, coin := range coins {
				got = append(got, coin.Quote["USD"].Price)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
