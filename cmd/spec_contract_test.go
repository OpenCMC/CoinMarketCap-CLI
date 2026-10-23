package cmd

import (
	"sort"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every endpoint a command calls must be documented in the published spec, so
// operation IDs and requires_auth can be derived from it.
func TestSpecContract_CommandEndpointsAreInSpec(t *testing.T) {
	for name, meta := range commandMeta {
		for _, ep := range meta.endpoints() {
			op, ok := spec.Lookup(ep)
			require.True(t, ok, "%s calls %s, which is not in the embedded spec", name, ep)
			assert.NotEmpty(t, op.OperationID)
		}
	}
}

// Commands still on endpoints the spec files under Deprecated. Moving a
// command off one means deleting it here; a new entry fails the test.
var knownDeprecatedCommandEndpoints = []string{
	"/v1/cryptocurrency/market-pairs/latest", // pairs
	"/v1/cryptocurrency/quotes/historical",   // history
}

func TestSpecContract_DeprecatedEndpointsAreTracked(t *testing.T) {
	seen := map[string]bool{}
	for _, meta := range commandMeta {
		for _, ep := range meta.endpoints() {
			if op, ok := spec.Lookup(ep); ok && op.Deprecated {
				seen[ep] = true
			}
		}
	}
	var got []string
	for ep := range seen {
		got = append(got, ep)
	}
	sort.Strings(got)
	want := append([]string(nil), knownDeprecatedCommandEndpoints...)
	sort.Strings(want)
	assert.Equal(t, want, got)
}
