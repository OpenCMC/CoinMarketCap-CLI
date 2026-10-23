package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_PassesResponseThroughUnchanged(t *testing.T) {
	// A field the CLI has never heard of must reach -o json output.
	const body = `{"data":{"value":48,"value_classification":"Neutral","brand_new_field":{"x":1}},"status":{"error_code":"0","credit_count":1}}`
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v3/fear-and-greed/latest", r.URL.Path)
		_, _ = w.Write([]byte(body))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommandCLI(t, "api", "/v3/fear-and-greed/latest")
	require.NoError(t, err)
	assert.Equal(t, body+"\n", stdout)
}

func TestAPI_ResolvesOperationIDAndParams(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/altcoin-season-index/historical", r.URL.Path)
		assert.Equal(t, "30d", r.URL.Query().Get("timeframe"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, stderr, err := executeCommandCLI(t, "api", "getV1AltcoinSeasonIndexHistorical", "-p", "timeframe=30d")
	require.NoError(t, err)
	assert.Empty(t, stderr)
}

func TestAPI_AcceptsFullURLsFromTheDocs(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/public-api/v1/global-metrics/quotes/latest", r.URL.Path)
		assert.Equal(t, "EUR", r.URL.Query().Get("convert"))
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	_, _, err := executeCommandCLI(t, "api", "https://pro-api.coinmarketcap.com/public-api/v1/global-metrics/quotes/latest?convert=EUR")
	require.NoError(t, err)
}

func TestAPI_KeylessRejectsKeyOnlyEndpoint(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("key-only endpoint must not be called keyless")
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	_, _, err := executeCommandCLI(t, "api", "/v1/content/latest")
	require.ErrorIs(t, err, api.ErrKeyRequired)
	assert.Contains(t, err.Error(), "GET /v1/content/latest")
}

func TestAPI_ValidatesParams(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, stderr, err := executeCommandCLI(t, "api", "/v1/altcoin-season-index/historical", "-p", "timeframe=1y", "-p", "brand_new=1")
	require.NoError(t, err, "unknown params and enum values warn but are sent")
	assert.Contains(t, stderr, `"brand_new"`)
	assert.Contains(t, stderr, `timeframe="1y" is not one of 7d, 30d, 90d`)

	_, _, err = executeCommandCLI(t, "api", "/v3/fear-and-greed/latest", "-p", "novalue")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name=value")

	_, _, err = executeCommandCLI(t, "api", "notAnOperation")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cmc api --list")
}

func TestAPI_RequiredParamsAreChecked(t *testing.T) {
	var op spec.Operation
	for _, candidate := range spec.Operations() {
		for _, p := range candidate.Params {
			if p.Required && candidate.Method == http.MethodGet {
				op = candidate
			}
		}
	}
	require.NotEmpty(t, op.Path, "spec should have a GET endpoint with a required parameter")

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request with missing required params must not be sent")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommandCLI(t, "api", op.Path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-p name=value")
}

func TestAPI_POSTBody(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/public-api/v1/dex/holders/list", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"platform":"ethereum"}`, string(b))
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	_, _, err := executeCommandCLI(t, "api", "/v1/dex/holders/list", "--body", `{"platform":"ethereum"}`)
	require.NoError(t, err)

	_, _, err = executeCommandCLI(t, "api", "/v1/dex/holders/list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--body")

	_, _, err = executeCommandCLI(t, "api", "/v3/fear-and-greed/latest", "--body", `{}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only for POST")
}

func TestAPI_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, _, err := executeCommandCLI(t, "api", "/v3/index/cmc20-latest", "--dry-run")
	require.NoError(t, err)
	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Equal(t, "https://pro-api.coinmarketcap.com/public-api/v3/index/cmc20-latest", out.URL)
	assert.Equal(t, "getV3IndexCMC20Latest", out.OASOperationID)
	assert.NotContains(t, out.Headers, "X-CMC_PRO_API_KEY")
}

func TestAPI_ListFilters(t *testing.T) {
	stdout, _, err := executeCommandCLI(t, "api", "--list", "--keyless", "-o", "json")
	require.NoError(t, err)
	var ops []spec.Operation
	require.NoError(t, json.Unmarshal([]byte(stdout), &ops))
	require.NotEmpty(t, ops)
	for _, op := range ops {
		assert.True(t, op.Keyless, "%s listed as keyless", op.Path)
		assert.False(t, op.Deprecated, "deprecated endpoints are hidden by default")
	}

	stdout, _, err = executeCommandCLI(t, "api", "--list", "fear-and-greed", "-o", "json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(stdout), &ops))
	require.NotEmpty(t, ops)
	for _, op := range ops {
		assert.Contains(t, op.Path, "fear-and-greed")
	}
}

func TestAPI_TableRendersArrayData(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"symbol":"BTC","id":1,"weight":0.5,"nested":{"a":1}}]}`))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "api", "/v3/index/cmc20-latest")
	require.NoError(t, err)
	assert.Contains(t, stdout, "BTC")
	assert.Contains(t, stdout, "weight")
	assert.NotContains(t, stdout, "nested", "nested values are left to -o json")
}
