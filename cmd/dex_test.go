package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDEX_ListedInRootHelp(t *testing.T) {
	stdout, stderr, err := executeCommandCLI(t, "--help")
	require.NoError(t, err)
	assert.True(t, helpListsCommandUnderAvailableCommands(stdout+stderr, "dex"))
}

func TestDEX_OnlyWorkingSubcommandsArePublic(t *testing.T) {
	stdout, stderr, err := executeCommandCLI(t, "dex", "--help")
	require.NoError(t, err)
	help := stdout + stderr
	for _, name := range []string{"pair", "pairs"} {
		assert.True(t, helpListsCommandUnderAvailableCommands(help, name), "dex help must list %q", name)
	}
	for _, name := range []string{"networks", "search", "ohlcv", "trades", "trending"} {
		assert.False(t, helpListsCommandUnderAvailableCommands(help, name), "dex %s calls a broken endpoint and must stay unregistered", name)
	}

	cat := parseCommandsJSONCatalog(t)
	var dexNames []string
	for _, c := range cat.Commands {
		if strings.HasPrefix(c.Name, "dex ") {
			dexNames = append(dexNames, c.Name)
		}
	}
	assert.ElementsMatch(t, []string{"dex pair", "dex pairs"}, dexNames)
}

func TestDEX_PairKeylessSendsNetworkSlugWithoutLookup(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/public-api/v4/dex/pairs/quotes/latest", r.URL.Path, "network slugs must not be resolved through /v4/dex/networks/list")
		assert.Equal(t, "ethereum", r.URL.Query().Get("network_slug"))
		assert.Empty(t, r.URL.Query().Get("network_id"))
		assert.Equal(t, "0xpair", r.URL.Query().Get("contract_address"))
		_, _ = w.Write([]byte(`{"data":[{"contract_address":"0xpair","name":"WETH/USDC"}],"status":{"error_code":"0"}}`))
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	stdout, _, err := executeCommandCLI(t, "dex", "pair", "--network", "Ethereum", "--pair", "0xpair", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"contract_address":"0xpair"`)
}

func TestDEX_PairsRequiresADEX(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request without a DEX must not be sent")
	})
	defer srv.Close()
	withTestClientKeyless(t, srv)

	_, _, err := executeCommandCLI(t, "dex", "pairs", "--network", "ethereum", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--dex-id or --dex-slug")
}

func TestDEX_PairsDryRunUsesNetworkIDOrSlug(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP requests")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	for network, wantKey := range map[string]string{"1": "network_id", "ethereum": "network_slug"} {
		stdout, _, err := executeCommandCLI(t, "dex", "pairs", "--network", network, "--dex-slug", "uniswap-v3", "--dry-run")
		require.NoError(t, err)
		var out dryRunOutput
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out))
		assert.Equal(t, network, out.Params[wantKey])
		assert.Equal(t, "uniswap-v3", out.Params["dex_slug"])
		assert.Equal(t, "getSpotPairsLatest", out.OASOperationID)
	}
}

func TestDEX_PairsAppliesLimitLocally(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// The live API ignores limit and always returns a full page.
		_, _ = w.Write([]byte(`{"data":[{"contract_address":"0x1"},{"contract_address":"0x2"},{"contract_address":"0x3"}],"status":{"error_code":"0"}}`))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommandCLI(t, "dex", "pairs", "--network", "ethereum", "--dex-slug", "uniswap-v3", "--limit", "2", "-o", "json")
	require.NoError(t, err)
	var pairs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &pairs))
	assert.Len(t, pairs, 2)
}
