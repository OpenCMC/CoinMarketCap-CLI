package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	TierBasic        = "basic"
	TierHobbyist     = "hobbyist"
	TierStartup      = "startup"
	TierStandard     = "standard"
	TierProfessional = "professional"
	TierEnterprise   = "enterprise"

	cmcBaseURL   = "https://pro-api.coinmarketcap.com"
	cmcHeaderKey = "X-CMC_PRO_API_KEY"

	// KeylessPathPrefix is prepended to every path in keyless mode.
	KeylessPathPrefix = "/public-api"

	// Compatibility aliases used by untouched command/tests in later batches.
	TierDemo = TierBasic
	TierPaid = TierHobbyist
)

var ValidTiers = []string{
	TierBasic,
	TierHobbyist,
	TierStartup,
	TierStandard,
	TierProfessional,
	TierEnterprise,
}

type Config struct {
	APIKey string `mapstructure:"api_key"`
	Tier   string `mapstructure:"tier"`
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	return filepath.Join(dir, "cmc-cli"), nil
}

func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(dir, "config.yaml"))

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	v := viper.New()
	v.Set("api_key", cfg.APIKey)
	v.Set("tier", cfg.Tier)

	path := filepath.Join(dir, "config.yaml")
	if err := v.WriteConfigAs(path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// IsKeyless reports whether no API key is configured. Requests then go to the
// Keyless Public API and are limited to the endpoints it serves.
func (c *Config) IsKeyless() bool {
	return strings.TrimSpace(c.APIKey) == ""
}

func (c *Config) BaseURL() string {
	if c.IsKeyless() {
		return cmcBaseURL + KeylessPathPrefix
	}
	return cmcBaseURL
}

func (c *Config) AuthHeader() (string, string) {
	return cmcHeaderKey, c.APIKey
}

func (c *Config) ApplyAuth(req *http.Request) {
	if c.APIKey != "" {
		key, val := c.AuthHeader()
		req.Header.Set(key, val)
	}
}

func (c *Config) IsPaid() bool {
	t := strings.ToLower(c.Tier)
	return t != "" && t != TierBasic && t != "demo"
}

func IsValidTier(tier string) bool {
	t := strings.ToLower(tier)
	for _, v := range ValidTiers {
		if t == v {
			return true
		}
	}
	return false
}

func (c *Config) MaskedKey() string {
	if len(c.APIKey) <= 8 {
		return strings.Repeat("*", len(c.APIKey))
	}
	return c.APIKey[:4] + strings.Repeat("*", len(c.APIKey)-8) + c.APIKey[len(c.APIKey)-4:]
}
