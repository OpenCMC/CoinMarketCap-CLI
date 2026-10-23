package cmd

import (
	"fmt"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current configuration",
	RunE:  runStatus,
}

func init() {
	addOutputFlag(statusCmd)
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	runtimeCfg, err := loadRuntimeConfig()
	if err != nil {
		return err
	}
	effective := runtimeCfg.Effective
	if effective == nil {
		effective = &config.Config{}
	}
	baseURL := effective.BaseURL()
	mode := statusModeAPIKey
	if effective.IsKeyless() {
		mode = statusModeKeyless
	}

	if outputJSON(cmd) {
		return printJSONRaw(statusOutput{
			Mode:          mode,
			AuthSource:    string(runtimeCfg.AuthSource),
			Configured:    runtimeCfg.Configured,
			Tier:          stringPtrOrNil(runtimeCfg),
			APIKey:        maskedKeyPtrOrNil(runtimeCfg),
			BaseURL:       baseURL,
			ConfigPath:    runtimeCfg.ConfigPath,
			ConfigWarning: stringPtr(runtimeCfg.Warning),
		})
	}

	display.PrintBanner()
	fmt.Printf("Mode:        %s\n", mode)
	fmt.Printf("Auth Source: %s\n", runtimeCfg.AuthSource)
	fmt.Printf("Configured:  %t\n", runtimeCfg.Configured)
	fmt.Printf("Tier:        %s\n", stringOrNone(runtimeCfg))
	fmt.Printf("API Key:     %s\n", maskedKeyOrNone(runtimeCfg))
	fmt.Printf("Base URL:    %s\n", baseURL)
	fmt.Printf("Config Path: %s\n", runtimeCfg.ConfigPath)
	if runtimeCfg.Warning != "" {
		fmt.Printf("Warning:     %s\n", runtimeCfg.Warning)
	}
	if mode == statusModeKeyless {
		fmt.Println("Note:        no API key set; using the Keyless Public API. Run `cmc auth` to add a free key for higher limits and every endpoint.")
	}
	return nil
}

// Status modes: keyless calls /public-api with no key header.
const (
	statusModeKeyless = "keyless"
	statusModeAPIKey  = "api_key"
)

type statusOutput struct {
	Mode          string  `json:"mode"`
	AuthSource    string  `json:"auth_source"`
	Configured    bool    `json:"configured"`
	Tier          *string `json:"tier"`
	APIKey        *string `json:"api_key"`
	BaseURL       string  `json:"base_url"`
	ConfigPath    string  `json:"config_path"`
	ConfigWarning *string `json:"config_warning"`
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func stringPtrOrNil(rt *config.RuntimeConfig) *string {
	if rt == nil || rt.Effective == nil {
		return nil
	}
	if rt.AuthSource == config.AuthSourceNone {
		return nil
	}
	if rt.Effective.Tier == "" {
		return nil
	}
	return &rt.Effective.Tier
}

func maskedKeyPtrOrNil(rt *config.RuntimeConfig) *string {
	if rt == nil || rt.Effective == nil {
		return nil
	}
	if rt.AuthSource == config.AuthSourceNone {
		return nil
	}
	if rt.Effective.APIKey == "" {
		return nil
	}
	masked := rt.Effective.MaskedKey()
	return &masked
}

func stringOrNone(rt *config.RuntimeConfig) string {
	if v := stringPtrOrNil(rt); v != nil {
		return *v
	}
	return "<unconfigured>"
}

func maskedKeyOrNone(rt *config.RuntimeConfig) string {
	if v := maskedKeyPtrOrNil(rt); v != nil {
		return *v
	}
	return "<unconfigured>"
}
