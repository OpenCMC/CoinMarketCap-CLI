package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedIndexLoads(t *testing.T) {
	info := Info()
	assert.Equal(t, SourceURL, info.Source)
	assert.Len(t, info.SHA256, 64)
	require.NotEmpty(t, info.Operations)

	seen := map[string]bool{}
	for _, op := range info.Operations {
		require.NotEmpty(t, op.OperationID, "%s has no operationId", op.Path)
		require.False(t, seen[op.Path], "duplicate path %s", op.Path)
		seen[op.Path] = true
	}
}

func TestLookupAndFind(t *testing.T) {
	op, ok := Lookup("/v3/cryptocurrency/quotes/latest?id=1")
	require.True(t, ok)
	assert.Equal(t, "getV3CryptocurrencyQuotesLatest", op.OperationID)
	assert.True(t, op.Keyless)
	_, hasID := op.Param("id")
	assert.True(t, hasID)

	byID, ok := Find("getv3cryptocurrencyquoteslatest")
	require.True(t, ok, "operationId lookup is case-insensitive")
	assert.Equal(t, op.Path, byID.Path)

	noSlash, ok := Find("v3/cryptocurrency/quotes/latest")
	require.True(t, ok)
	assert.Equal(t, op.Path, noSlash.Path)

	_, ok = Find("/v9/not/a/path")
	assert.False(t, ok)
}

func TestIsKeyless(t *testing.T) {
	assert.True(t, IsKeyless("/v1/global-metrics/quotes/latest"))
	assert.True(t, IsKeyless("/v3/fear-and-greed/latest"))
	assert.False(t, IsKeyless("/v1/content/latest"))
	assert.False(t, IsKeyless("/v9/unknown"), "unknown paths are not keyless")
}

func TestKeylessCountMatchesPublishedKeylessSpec(t *testing.T) {
	// The generator cross-checks x-keyless against the keyless spec; this keeps
	// a regeneration from silently dropping every flag.
	n := 0
	for _, op := range Operations() {
		if op.Keyless {
			n++
		}
	}
	assert.Greater(t, n, 20)
}

func TestNormalizePath(t *testing.T) {
	assert.Equal(t, "/v1/a", NormalizePath("v1/a"))
	assert.Equal(t, "/v1/a", NormalizePath("//v1/a?x=1"))
	assert.Equal(t, "/v1/a", NormalizePath(" /v1/a "))
}
