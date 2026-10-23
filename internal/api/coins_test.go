package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrendingGainersLosers(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/cryptocurrency/trending/gainers-losers", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "1", q.Get("start"))
		assert.Equal(t, "100", q.Get("limit"))
		assert.Equal(t, "24h", q.Get("time_period"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":[
				{
					"id":1,
					"name":"Bitcoin",
					"symbol":"BTC",
					"slug":"bitcoin",
					"quote":{"USD":{"price":65000,"percent_change_24h":15.5}}
				}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.TrendingGainersLosers(context.Background(), 1, 100, "24h", "USD")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Bitcoin", result[0].Name)
	assert.Equal(t, 15.5, result[0].Quote["USD"].PercentChange24h)
}

func TestTrendingLatest(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/cryptocurrency/trending/latest", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "1", q.Get("start"))
		assert.Equal(t, "50", q.Get("limit"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":[
				{
					"id":1,
					"name":"Bitcoin",
					"symbol":"BTC",
					"slug":"bitcoin",
					"cmc_rank":1,
					"quote":{"USD":{"price":65000,"volume_24h":45000000000,"market_cap":1300000000000,"percent_change_24h":2.5}}
				}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.TrendingLatest(context.Background(), 1, 50, "USD")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Bitcoin", result[0].Name)
	assert.Equal(t, 2.5, result[0].Quote["USD"].PercentChange24h)
}

func TestListingsLatest(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/listings/latest", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "1", q.Get("start"))
		assert.Equal(t, "100", q.Get("limit"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":[
				{
					"id":1,
					"name":"Bitcoin",
					"symbol":"BTC",
					"slug":"bitcoin",
					"cmc_rank":1,
					"quote":{"USD":{"price":65000,"volume_24h":45000000000,"market_cap":1300000000000,"percent_change_24h":2.5}}
				}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.ListingsLatest(context.Background(), 1, 100, "USD", "", "")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Bitcoin", result[0].Name)
	assert.Equal(t, 1, result[0].CMCRank)
	assert.Equal(t, 65000.0, result[0].Quote["USD"].Price)
}

func TestListingsLatestWithCategoryResolvesCategoryID(t *testing.T) {
	callCount := 0
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch callCount {
		case 1:
			assert.Equal(t, "/v1/cryptocurrency/categories", r.URL.Path)
			assert.Equal(t, "1", r.URL.Query().Get("start"))
			assert.Equal(t, "5000", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{
				"data":[
					{"id":"604f2776ebccdd50cd175fdc","name":"Layer 2","title":"Layer 2"}
				]
			}`))
		case 2:
			assert.Equal(t, "/v1/cryptocurrency/category", r.URL.Path)
			q := r.URL.Query()
			assert.Equal(t, "604f2776ebccdd50cd175fdc", q.Get("id"))
			assert.Equal(t, "1", q.Get("start"))
			assert.Equal(t, "100", q.Get("limit"))
			assert.Equal(t, "USD", q.Get("convert"))
			_, _ = w.Write([]byte(`{
				"data":{
					"id":"604f2776ebccdd50cd175fdc",
					"name":"Layer 2",
					"title":"Layer 2",
					"coins":[
						{
							"id":1,
							"name":"Bitcoin",
							"symbol":"BTC",
							"slug":"bitcoin",
							"cmc_rank":1,
							"quote":{"USD":{"price":65000,"volume_24h":45000000000,"market_cap":1300000000000,"percent_change_24h":2.5}}
						}
					]
				}
			}`))
		default:
			t.Fatalf("unexpected request %d", callCount)
		}
	})
	defer srv.Close()

	result, err := c.ListingsLatestWithCategory(context.Background(), 1, 100, "USD", "", "", "layer-2")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Bitcoin", result[0].Name)
	assert.Equal(t, 2, callCount)
}

func TestResolveCategoryID_BySlugLikeToken(t *testing.T) {
	callCount := 0
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, "/v1/cryptocurrency/categories", r.URL.Path)
		_, _ = w.Write([]byte(`{
			"data":[
				{"id":"604f2776ebccdd50cd175fdc","name":"Layer 2","title":"Layer 2"}
			]
		}`))
	})
	defer srv.Close()

	id, err := c.ResolveCategoryID(context.Background(), "layer-2")
	require.NoError(t, err)
	assert.Equal(t, "604f2776ebccdd50cd175fdc", id)
	assert.Equal(t, 1, callCount)
}

func TestQuotesLatestByID(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "1,1027", q.Get("id"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":{
				"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":{"USD":{"price":65000,"percent_change_24h":2.5}}},
				"1027":{"id":1027,"name":"Ethereum","symbol":"ETH","slug":"ethereum","quote":{"USD":{"price":3400,"percent_change_24h":-1.2}}}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.QuotesLatestByID(context.Background(), []string{"1", "1027"}, "USD")
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Bitcoin", result["1"].Name)
	assert.Equal(t, 65000.0, result["1"].Quote["USD"].Price)
	assert.Equal(t, 2.5, result["1"].Quote["USD"].PercentChange24h)
}

func TestQuotesLatestBySlug(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "bitcoin,ethereum", q.Get("slug"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":{
				"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":{"USD":{"price":65000,"percent_change_24h":2.5}}}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.QuotesLatestBySlug(context.Background(), []string{"bitcoin", "ethereum"}, "USD")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Bitcoin", result["1"].Name)
}

func TestQuotesLatestBySymbol(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/cryptocurrency/quotes/latest", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "BTC,ETH", q.Get("symbol"))
		assert.Equal(t, "USD", q.Get("convert"))

		_, _ = w.Write([]byte(`{
			"data":{
				"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":{"USD":{"price":65000,"percent_change_24h":2.5}}},
				"1027":{"id":1027,"name":"Ethereum","symbol":"ETH","slug":"ethereum","quote":{"USD":{"price":3400,"percent_change_24h":-1.2}}}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.QuotesLatestBySymbol(context.Background(), []string{"BTC", "ETH"}, "USD")
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Bitcoin", result["1"].Name)
	assert.Equal(t, "Ethereum", result["1027"].Name)
}
