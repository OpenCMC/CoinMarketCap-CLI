package cmd

import (
	"fmt"
	"os"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:               "cmc",
	Short:             "CoinMarketCap CLI for cryptocurrency market data",
	Long:              "A command-line tool for accessing CoinMarketCap cryptocurrency market data.",
	Version:           version,
	PersistentPreRunE: validateRootFlags,
	Run: func(cmd *cobra.Command, args []string) {
		display.PrintLogo()
		display.PrintWelcomeBox()
	},
}

func Execute() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		// Emit structured JSON error to stderr when -o json, otherwise plain text.
		cmd, _, _ := rootCmd.Find(os.Args[1:])
		if cmd != nil && outputJSON(cmd) {
			_ = formatError(cmd, err)
		} else {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

func addOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", "json", "Output format (json, table)")
}

func validateRootFlags(cmd *cobra.Command, _ []string) error {
	if err := validateOutputFormat(cmd); err != nil {
		return err
	}
	return requireKeyForCommand(cmd)
}

// requireKeyForCommand stops commands that only call key-only endpoints before
// any request (or dry-run) when no API key is configured.
func requireKeyForCommand(cmd *cobra.Command) error {
	name := catalogName(cmd)
	meta, ok := commandMeta[name]
	if !ok || !meta.requiresAuth() {
		return nil
	}
	cfg, err := loadConfig()
	if err != nil || !cfg.IsKeyless() {
		// Config errors are reported by the command itself.
		return nil
	}
	return api.KeyRequired("`cmc " + name + "`")
}

// catalogName is the command's name in `cmc commands`: "price", "dex pairs".
func catalogName(cmd *cobra.Command) string {
	if parent := cmd.Parent(); parent != nil && parent != cmd.Root() {
		return parent.Name() + " " + cmd.Name()
	}
	return cmd.Name()
}

func validateOutputFormat(cmd *cobra.Command) error {
	if cmd == nil || cmd.Flags().Lookup("output") == nil {
		return nil
	}

	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}
	switch output {
	case "json", "table":
		return nil
	default:
		return fmt.Errorf("--output must be one of: json, table (got %q)", output)
	}
}

func outputJSON(cmd *cobra.Command) bool {
	o, err := cmd.Flags().GetString("output")
	return err == nil && o == "json"
}
