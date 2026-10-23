package config

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidTier(t *testing.T) {
	tests := []struct {
		tier  string
		valid bool
	}{
		{"basic", true},
		{"hobbyist", true},
		{"startup", true},
		{"standard", true},
		{"professional", true},
		{"enterprise", true},
		{"Basic", true},
		{"HOBBYIST", true},
		{"demo", false},
		{"paid", false},
		{"free", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.valid, IsValidTier(tt.tier), "tier=%q", tt.tier)
	}
}

func TestIsPaid(t *testing.T) {
	tests := []struct {
		tier string
		paid bool
	}{
		{"basic", false},
		{"hobbyist", true},
		{"startup", true},
		{"standard", true},
		{"professional", true},
		{"enterprise", true},
		{"Hobbyist", true},
	}
	for _, tt := range tests {
		cfg := &Config{Tier: tt.tier}
		assert.Equal(t, tt.paid, cfg.IsPaid(), "tier=%q", tt.tier)
	}
}

func TestBaseURL(t *testing.T) {
	basic := &Config{APIKey: "basic-key", Tier: "basic"}
	assert.Equal(t, cmcBaseURL, basic.BaseURL())

	hobbyist := &Config{APIKey: "hobbyist-key", Tier: "hobbyist"}
	assert.Equal(t, cmcBaseURL, hobbyist.BaseURL())
}

func TestKeylessMode(t *testing.T) {
	keyless := &Config{}
	assert.True(t, keyless.IsKeyless())
	assert.Equal(t, "https://pro-api.coinmarketcap.com/public-api", keyless.BaseURL())

	req, _ := http.NewRequest(http.MethodGet, keyless.BaseURL()+"/v1/global-metrics/quotes/latest", nil)
	keyless.ApplyAuth(req)
	assert.Empty(t, req.Header.Get("X-CMC_PRO_API_KEY"), "keyless requests must not send the key header")

	keyed := &Config{APIKey: "k"}
	assert.False(t, keyed.IsKeyless())
}

func TestAuthHeader(t *testing.T) {
	basic := &Config{APIKey: "basic-key-123", Tier: "basic"}
	key, val := basic.AuthHeader()
	assert.Equal(t, cmcHeaderKey, key)
	assert.Equal(t, "basic-key-123", val)

	hobbyist := &Config{APIKey: "hobbyist-key-456", Tier: "hobbyist"}
	key, val = hobbyist.AuthHeader()
	assert.Equal(t, cmcHeaderKey, key)
	assert.Equal(t, "hobbyist-key-456", val)
}

func TestApplyAuth(t *testing.T) {
	cfg := &Config{APIKey: "test-key", Tier: "basic"}
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	cfg.ApplyAuth(req)
	assert.Equal(t, "test-key", req.Header.Get(cmcHeaderKey))

	// No key set — should not add header
	cfg2 := &Config{Tier: "basic"}
	req2, _ := http.NewRequest("GET", "https://example.com", nil)
	cfg2.ApplyAuth(req2)
	assert.Empty(t, req2.Header.Get(cmcHeaderKey))
}

func TestLoadMissingConfigReturnsEmptyConfig(t *testing.T) {
	// Point HOME to a temp dir so os.UserConfigDir() finds no config
	t.Setenv("HOME", t.TempDir())
	cfg, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, cfg.Tier)
	assert.Empty(t, cfg.APIKey)
}

func TestMaskedKey(t *testing.T) {
	tests := []struct {
		key    string
		expect string
	}{
		{"", ""},
		{"abcd", "****"},
		{"abcdefgh", "********"},
		{"abcdefghij", "abcd**ghij"},
		{"CG-abc123def456ghi", "CG-a**********6ghi"},
	}
	for _, tt := range tests {
		cfg := &Config{APIKey: tt.key}
		assert.Equal(t, tt.expect, cfg.MaskedKey(), "key=%q", tt.key)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := ConfigPath()
	assert.NoError(t, err)
	assert.Contains(t, path, "cmc-cli/config.yaml")
}
