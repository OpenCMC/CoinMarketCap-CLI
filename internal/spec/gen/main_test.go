package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureSpec = `{
  "openapi": "3.1.0",
  "info": {"version": "9.9.9"},
  "paths": {
    "/v2/b": {
      "post": {
        "operationId": "postB",
        "tags": ["Deprecated"],
        "summary": "",
        "description": "Posts B. More detail follows.\n\nSecond paragraph.",
        "requestBody": {"content": {}}
      }
    },
    "/v1/a": {
      "parameters": [],
      "get": {
        "operationId": "getA",
        "tags": ["Cryptocurrency"],
        "summary": "Get A",
        "x-keyless": true,
        "parameters": [
          {"name": "limit", "in": "query", "required": true, "description": "Max rows. Defaults to 10.", "schema": {"type": "integer", "default": 10}},
          {"name": "sort", "in": "query", "schema": {"type": ["string", "null"], "enum": ["asc", "desc"]}},
          {"name": "X-Trace", "in": "header", "schema": {"type": "string"}}
        ]
      }
    }
  }
}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRunBuildsSortedIndex(t *testing.T) {
	dir := t.TempDir()
	specPath := writeFile(t, dir, "openapi.json", fixtureSpec)
	keylessPath := writeFile(t, dir, "keyless.json", `{"openapi":"3.1.0","paths":{"/v1/a":{"get":{}}}}`)
	out := filepath.Join(dir, "operations.json")

	require.NoError(t, run(specPath, keylessPath, out))

	var index spec.Index
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &index))

	assert.Equal(t, "9.9.9", index.SpecVersion)
	assert.Len(t, index.SHA256, 64)
	require.Len(t, index.Operations, 2)

	a := index.Operations[0]
	assert.Equal(t, "/v1/a", a.Path, "operations are sorted by path")
	assert.Equal(t, "GET", a.Method)
	assert.True(t, a.Keyless)
	assert.False(t, a.Deprecated)
	require.Len(t, a.Params, 2, "only query parameters are kept")
	assert.Equal(t, spec.Param{Name: "limit", Type: "integer", Required: true, Default: "10", Description: "Max rows."}, a.Params[0])
	assert.Equal(t, "string", a.Params[1].Type, "3.1 type arrays resolve to the non-null type")
	assert.Equal(t, []string{"asc", "desc"}, a.Params[1].Enum)

	b2 := index.Operations[1]
	assert.Equal(t, "POST", b2.Method)
	assert.True(t, b2.Deprecated, "the Deprecated tag marks an operation deprecated")
	assert.True(t, b2.Body)
	assert.Equal(t, "Posts B.", b2.Summary, "summary falls back to the description's first sentence")
}

func TestRunRejectsKeylessMismatch(t *testing.T) {
	dir := t.TempDir()
	specPath := writeFile(t, dir, "openapi.json", fixtureSpec)
	keylessPath := writeFile(t, dir, "keyless.json", `{"openapi":"3.1.0","paths":{"/v2/b":{"post":{}}}}`)

	err := run(specPath, keylessPath, filepath.Join(dir, "operations.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GET /v1/a (x-keyless=true, in keyless spec=false)")
	assert.Contains(t, err.Error(), "POST /v2/b (x-keyless=false, in keyless spec=true)")
}

func TestRunRejectsNonSpecContent(t *testing.T) {
	dir := t.TempDir()
	htmlPath := writeFile(t, dir, "openapi.json", "<!doctype html><html></html>")

	err := run(htmlPath, "", filepath.Join(dir, "operations.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an OpenAPI document")
}

func TestEmbeddedIndexIsUpToDateWithGenerator(t *testing.T) {
	// Regenerating from the spec the index records must reproduce the
	// committed file byte for byte. Skipped offline; run `make spec` to update.
	if testing.Short() || os.Getenv("CMC_SPEC_CHECK") == "" {
		t.Skip("set CMC_SPEC_CHECK=1 to compare operations.json with the live spec")
	}
	out := filepath.Join(t.TempDir(), "operations.json")
	require.NoError(t, run(spec.SourceURL, spec.KeylessSourceURL, out))
	want, err := os.ReadFile(out)
	require.NoError(t, err)
	got, err := os.ReadFile("../operations.json")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got), "operations.json is stale; run `make spec`")
}
