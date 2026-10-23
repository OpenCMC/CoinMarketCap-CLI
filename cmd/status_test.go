package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/stretchr/testify/require"
)

func writeIsolatedConfig(t *testing.T, apiKey, tier string) string {
	t.Helper()
	withIsolatedUserConfigDir(t)

	cfg := &config.Config{APIKey: apiKey, Tier: tier}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, err := config.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	return path
}

func TestStatus_JSONOutputContainsTruthfulContractFields(t *testing.T) {
	const wantKey = "status-json-key-abcdefghij"
	wantTier := config.TierStandard
	configPath := writeIsolatedConfig(t, wantKey, wantTier)

	wantMasked := (&config.Config{APIKey: wantKey}).MaskedKey()
	wantBase := (&config.Config{APIKey: wantKey}).BaseURL()

	stdout, stderr, err := executeCommand(t, "status", "-o", "json")
	if err != nil {
		t.Fatalf("status -o json: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}

	var got map[string]any
	if decErr := json.NewDecoder(strings.NewReader(stdout)).Decode(&got); decErr != nil {
		t.Fatalf("decode JSON: %v\nraw: %q", decErr, stdout)
	}

	for _, key := range []string{"mode", "auth_source", "configured", "tier", "api_key", "base_url", "config_path", "config_warning"} {
		if _, ok := got[key]; !ok {
			t.Errorf("JSON missing key %q; got %#v", key, got)
		}
	}
	require.Equal(t, "api_key", got["mode"])
	require.Equal(t, "config", got["auth_source"])
	require.Equal(t, true, got["configured"])
	require.Equal(t, wantTier, got["tier"])
	require.Equal(t, wantMasked, got["api_key"])
	require.Equal(t, wantBase, got["base_url"])
	require.Equal(t, configPath, got["config_path"])
	require.Nil(t, got["config_warning"])
}

func TestStatus_EnvOnlyIsTruthful(t *testing.T) {
	withIsolatedUserConfigDir(t)

	const wantKey = "env-only-status-key-abcdefgh"
	t.Setenv("CMC_API_KEY", wantKey)

	stdout, stderr, err := executeCommand(t, "status", "-o", "json")
	if err != nil {
		t.Fatalf("status -o json: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}

	var got map[string]any
	require.NoError(t, json.NewDecoder(strings.NewReader(stdout)).Decode(&got))
	require.Equal(t, "env", got["auth_source"])
	require.Equal(t, false, got["configured"])
	require.Nil(t, got["tier"])
	require.Equal(t, (&config.Config{APIKey: wantKey}).MaskedKey(), got["api_key"])
	require.Nil(t, got["config_warning"])
}

func TestStatus_EmptyConfigIsTruthful(t *testing.T) {
	withIsolatedUserConfigDir(t)

	stdout, stderr, err := executeCommand(t, "status", "-o", "json")
	if err != nil {
		t.Fatalf("status -o json: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}

	var got map[string]any
	require.NoError(t, json.NewDecoder(strings.NewReader(stdout)).Decode(&got))
	require.Equal(t, "none", got["auth_source"])
	require.Equal(t, false, got["configured"])
	require.Nil(t, got["tier"])
	require.Nil(t, got["api_key"])
	require.Nil(t, got["config_warning"])
}

func TestStatus_InvalidPersistedTierSurfacesWarning(t *testing.T) {
	withIsolatedUserConfigDir(t)

	configDir := filepath.Dir(mustConfigPath(t))
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	require.NoError(t, os.WriteFile(mustConfigPath(t), []byte("api_key: invalid-tier-key\ntier: nope\n"), 0o600))

	stdout, stderr, err := executeCommand(t, "status", "-o", "json")
	if err != nil {
		t.Fatalf("status -o json: %v", err)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr:\n%s", stderr)
	}

	var got map[string]any
	require.NoError(t, json.NewDecoder(strings.NewReader(stdout)).Decode(&got))
	require.Equal(t, "api_key", got["mode"])
	require.Equal(t, "config", got["auth_source"])
	require.Equal(t, true, got["configured"])
	require.Nil(t, got["tier"])
	require.Equal(t, (&config.Config{APIKey: "invalid-tier-key"}).MaskedKey(), got["api_key"])
	require.NotNil(t, got["config_warning"])
	require.Contains(t, strings.ToLower(got["config_warning"].(string)), "tier")
}

func mustConfigPath(t *testing.T) string {
	t.Helper()
	path, err := config.ConfigPath()
	require.NoError(t, err)
	return path
}
