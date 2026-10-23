package cmd

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPrice_UsesEnvOnlyRuntimeAuth(t *testing.T) {
	withIsolatedUserConfigDir(t)

	const wantKey = "env-runtime-price-key-12345"
	t.Setenv("CMC_API_KEY", wantKey)

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-CMC_PRO_API_KEY"); got != wantKey {
			t.Fatalf("expected runtime auth to use env API key, got %q", got)
		}
		if r.URL.Path != "/v3/cryptocurrency/quotes/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"data":{"1":{"id":1,"name":"Bitcoin","symbol":"BTC","slug":"bitcoin","quote":{"USD":{"price":1,"percent_change_24h":2,"volume_24h":3,"market_cap":4}}}}}`)
	})
	defer srv.Close()

	origClient := newAPIClient
	newAPIClient = func(cfg *config.Config) *api.Client {
		c := api.NewClient(cfg)
		c.SetBaseURL(srv.URL)
		return c
	}
	t.Cleanup(func() { newAPIClient = origClient })

	stdout, stderr, err := executeCommandCLI(t, "price", "--id", "1", "-o", "json")
	require.NoError(t, err, "price should succeed with env-only runtime auth")
	require.Empty(t, stderr)
	require.Contains(t, stdout, `"id":1`)
	require.Contains(t, stdout, `"symbol":"BTC"`)
}
