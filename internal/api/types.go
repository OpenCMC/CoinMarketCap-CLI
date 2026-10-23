package api

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// CMC listings/latest response types
type ListingsLatestResponse struct {
	Data []ListingCoin `json:"data"`
}

type CategoriesResponse struct {
	Data []CategorySummary `json:"data"`
}

type CategorySummary struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	Description     string  `json:"description"`
	Volume          float64 `json:"volume"`
	NumTokens       int     `json:"num_tokens"`
	AvgPriceChange  float64 `json:"avg_price_change"`
	MarketCap       float64 `json:"market_cap"`
	MarketCapChange float64 `json:"market_cap_change"`
	VolumeChange    float64 `json:"volume_change"`
	LastUpdated     string  `json:"last_updated"`
}

type CategoryResponse struct {
	Data CategoryDetail `json:"data"`
}

type CategoryDetail struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Volume      float64       `json:"volume"`
	Coins       []ListingCoin `json:"coins"`
}

type ListingCoin struct {
	ID      int64            `json:"id"`
	Name    string           `json:"name"`
	Symbol  string           `json:"symbol"`
	Slug    string           `json:"slug"`
	CMCRank int              `json:"cmc_rank"`
	Quote   QuoteMap[Quote2] `json:"quote"`
}

type Quote2 struct {
	Price            float64 `json:"price"`
	Volume24h        float64 `json:"volume_24h"`
	MarketCap        float64 `json:"market_cap"`
	PercentChange1h  float64 `json:"percent_change_1h"`
	PercentChange24h float64 `json:"percent_change_24h"`
	PercentChange7d  float64 `json:"percent_change_7d"`
	PercentChange30d float64 `json:"percent_change_30d"`
}

// CMC quotes/latest response types
type QuotesLatestResponse struct {
	Data QuoteCoins `json:"data"`
}

// QuoteCoins holds quotes keyed by CMC ID. It decodes both the v2 object keyed
// by ID and the v3 array, so output keeps the v2 shape.
type QuoteCoins map[string]QuoteCoin

func (m *QuoteCoins) UnmarshalJSON(b []byte) error {
	out, err := decodeKeyed[QuoteCoin](b, func(item json.RawMessage) (string, error) {
		var key struct {
			ID int64 `json:"id"`
		}
		err := json.Unmarshal(item, &key)
		return strconv.FormatInt(key.ID, 10), err
	})
	*m = out
	return err
}

type QuoteCoin struct {
	ID     int64           `json:"id"`
	Name   string          `json:"name"`
	Symbol string          `json:"symbol"`
	Slug   string          `json:"slug"`
	Quote  QuoteMap[Quote] `json:"quote"`
}

// QuoteMap holds quotes keyed by convert symbol ("USD"). It decodes both the
// v1/v2 object and the v3 array of quotes that carry a "symbol" field.
type QuoteMap[T any] map[string]T

func (m *QuoteMap[T]) UnmarshalJSON(b []byte) error {
	out, err := decodeKeyed[T](b, func(item json.RawMessage) (string, error) {
		var key struct {
			Symbol string `json:"symbol"`
		}
		err := json.Unmarshal(item, &key)
		return key.Symbol, err
	})
	*m = out
	return err
}

// decodeKeyed decodes a JSON object, or a JSON array whose items are keyed by keyOf, into a map.
func decodeKeyed[T any](b []byte, keyOf func(json.RawMessage) (string, error)) (map[string]T, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '[' {
		var obj map[string]T
		err := json.Unmarshal(b, &obj)
		return obj, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, err
	}
	out := make(map[string]T, len(items))
	for _, item := range items {
		key, err := keyOf(item)
		if err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal(item, &v); err != nil {
			return nil, err
		}
		out[key] = v
	}
	return out, nil
}

type Quote struct {
	Price            float64 `json:"price"`
	PercentChange24h float64 `json:"percent_change_24h"`
	Volume24h        float64 `json:"volume_24h"`
	MarketCap        float64 `json:"market_cap"`
}

// Simple price response: map[coinID]map[field]value
// Fields include currency price (float64) and 24h change (float64).
type PriceResponse map[string]map[string]float64

type MarketCoin struct {
	ID                       string  `json:"id"`
	Symbol                   string  `json:"symbol"`
	Name                     string  `json:"name"`
	CurrentPrice             float64 `json:"current_price"`
	MarketCap                float64 `json:"market_cap"`
	MarketCapRank            int     `json:"market_cap_rank"`
	TotalVolume              float64 `json:"total_volume"`
	PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
	High24h                  float64 `json:"high_24h"`
	Low24h                   float64 `json:"low_24h"`
	ATH                      float64 `json:"ath"`
	ATHChangePercentage      float64 `json:"ath_change_percentage"`
	ATL                      float64 `json:"atl"`
	ATLChangePercentage      float64 `json:"atl_change_percentage"`
	CirculatingSupply        float64 `json:"circulating_supply"`
	TotalSupply              float64 `json:"total_supply"`
}

type SearchResponse struct {
	Coins []SearchCoin `json:"coins"`
}

type SearchCoin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	MarketCapRank int    `json:"market_cap_rank"`
}

type TrendingResponse struct {
	Coins      []TrendingCoinWrapper `json:"coins"`
	NFTs       []TrendingNFT         `json:"nfts"`
	Categories []TrendingCategory    `json:"categories"`
}

type TrendingCoinWrapper struct {
	Item TrendingCoin `json:"item"`
}

type TrendingCoin struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Symbol        string            `json:"symbol"`
	MarketCapRank int               `json:"market_cap_rank"`
	Score         int               `json:"score"`
	Data          *TrendingCoinData `json:"data"`
}

type TrendingCoinData struct {
	Price                    float64            `json:"price"`
	PriceChangePercentage24h map[string]float64 `json:"price_change_percentage_24h"`
}

type TrendingNFT struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Symbol               string  `json:"symbol"`
	FloorPriceInUSD24hPC float64 `json:"floor_price_24h_percentage_change"`
}

type TrendingCategory struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	MarketCap1hChange float64 `json:"market_cap_1h_change"`
}

type HistoricalData struct {
	ID         string            `json:"id"`
	Symbol     string            `json:"symbol"`
	Name       string            `json:"name"`
	MarketData *HistoricalMarket `json:"market_data"`
}

type HistoricalMarket struct {
	CurrentPrice map[string]float64 `json:"current_price"`
	MarketCap    map[string]float64 `json:"market_cap"`
	TotalVolume  map[string]float64 `json:"total_volume"`
}

// OHLC data: each entry is [timestamp, open, high, low, close]
type OHLCData [][]float64

type MarketChartResponse struct {
	Prices       [][]float64 `json:"prices"`
	MarketCaps   [][]float64 `json:"market_caps"`
	TotalVolumes [][]float64 `json:"total_volumes"`
}

type GainersLosersResponse struct {
	TopGainers []GainerCoin `json:"top_gainers"`
	TopLosers  []GainerCoin `json:"top_losers"`
}

// GainerCoin uses dynamic JSON keys for price fields based on the vs_currency
// parameter. The API returns {currency} and {currency}_24h_change as keys
// (e.g. "usd", "usd_24h_change" or "eur", "eur_24h_change").
type GainerCoin struct {
	ID            string                 `json:"id"`
	Symbol        string                 `json:"symbol"`
	Name          string                 `json:"name"`
	Image         string                 `json:"image"`
	MarketCapRank int                    `json:"market_cap_rank"`
	Extra         map[string]interface{} `json:"-"`
}

// Price returns the price in the given vs currency.
func (g *GainerCoin) Price(vs string) float64 {
	v, _ := g.Extra[vs].(float64)
	return v
}

// PriceChange returns the 24h price change percentage in the given vs currency.
func (g *GainerCoin) PriceChange(vs string) float64 {
	v, _ := g.Extra[vs+"_24h_change"].(float64)
	return v
}

func (g *GainerCoin) UnmarshalJSON(data []byte) error {
	// Single-pass: unmarshal into flat map, then extract known fields.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["id"].(string); ok {
		g.ID = v
	}
	if v, ok := raw["symbol"].(string); ok {
		g.Symbol = v
	}
	if v, ok := raw["name"].(string); ok {
		g.Name = v
	}
	if v, ok := raw["image"].(string); ok {
		g.Image = v
	}
	if v, ok := raw["market_cap_rank"].(float64); ok {
		g.MarketCapRank = int(v)
	}
	g.Extra = raw
	return nil
}

func (g GainerCoin) MarshalJSON() ([]byte, error) {
	// Re-serialize by merging Extra (which has all original fields) back out.
	if g.Extra == nil {
		type Alias GainerCoin
		return json.Marshal(Alias(g))
	}
	return json.Marshal(g.Extra)
}

type CoinDetail struct {
	ID            string `json:"id"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	MarketCapRank int    `json:"market_cap_rank"`
	Description   struct {
		EN string `json:"en"`
	} `json:"description"`
	MarketData *CoinDetailMarket `json:"market_data"`
}

type CoinDetailMarket struct {
	CurrentPrice             map[string]float64 `json:"current_price"`
	MarketCap                map[string]float64 `json:"market_cap"`
	TotalVolume              map[string]float64 `json:"total_volume"`
	High24h                  map[string]float64 `json:"high_24h"`
	Low24h                   map[string]float64 `json:"low_24h"`
	PriceChangePercentage24h float64            `json:"price_change_percentage_24h"`
	ATH                      map[string]float64 `json:"ath"`
	ATHChangePercentage      map[string]float64 `json:"ath_change_percentage"`
	ATHDate                  map[string]string  `json:"ath_date"`
	ATL                      map[string]float64 `json:"atl"`
	ATLChangePercentage      map[string]float64 `json:"atl_change_percentage"`
	ATLDate                  map[string]string  `json:"atl_date"`
	CirculatingSupply        float64            `json:"circulating_supply"`
	TotalSupply              float64            `json:"total_supply"`
}

// CMC Trending Gainers/Losers response types
type TrendingGainersLosersResponse struct {
	Data []TrendingCoin2 `json:"data"`
}

type TrendingCoin2 struct {
	ID     int64             `json:"id"`
	Name   string            `json:"name"`
	Symbol string            `json:"symbol"`
	Slug   string            `json:"slug"`
	Quote  map[string]Quote2 `json:"quote"`
}

type InfoResponse struct {
	Data map[string]CoinInfo `json:"data"`
}

type CoinInfo struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Symbol      string       `json:"symbol"`
	Slug        string       `json:"slug"`
	Category    string       `json:"category"`
	Description string       `json:"description"`
	Tags        []string     `json:"tags,omitempty"`
	URLs        CoinInfoURLs `json:"urls,omitempty"`
	Logo        string       `json:"logo"`
	DateAdded   string       `json:"date_added"`
}
