package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// withTestClient overrides newAPIClient so that command handlers use a client
// pointed at the given httptest.Server. Restored via t.Cleanup.
func withTestClient(t *testing.T, srv *httptest.Server, tier string) {
	t.Helper()

	// Override loadConfig to return a test config without touching real config files.
	origLoad := loadConfig
	loadConfig = func() (*config.Config, error) {
		return &config.Config{APIKey: "test-key", Tier: tier}, nil
	}
	t.Cleanup(func() { loadConfig = origLoad })

	// Override newAPIClient to point at the test server.
	origClient := newAPIClient
	newAPIClient = func(cfg *config.Config) *api.Client {
		c := api.NewClient(cfg)
		c.SetBaseURL(srv.URL)
		return c
	}
	t.Cleanup(func() { newAPIClient = origClient })
}

// withTestClientDemo is shorthand for withTestClient with demo tier.
func withTestClientDemo(t *testing.T, srv *httptest.Server) {
	t.Helper()
	withTestClient(t, srv, config.TierBasic)
}

// withTestClientPaid is shorthand for withTestClient with paid tier.
func withTestClientPaid(t *testing.T, srv *httptest.Server) {
	t.Helper()
	withTestClient(t, srv, config.TierHobbyist)
}

// executeCommand runs rootCmd with the given args and captures stdout/stderr.
// It resets rootCmd args via t.Cleanup.
func executeCommand(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	// Capture stdout.
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	// Capture stderr.
	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	// Reset all flags to defaults before each run to prevent state leakage.
	resetAllFlags(rootCmd)
	if !containsOutputFlag(args) && commandSupportsFlag(args, "output") {
		args = append(args, "-o", "table")
	}

	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SilenceUsage = false
		rootCmd.SilenceErrors = false
	})

	// Drain pipes concurrently to avoid deadlock if output exceeds pipe buffer.
	var bufOut, bufErr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&bufOut, rOut) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&bufErr, rErr) }()

	// Run the command — Cobra writes to os.Stdout/os.Stderr.
	cmdErr := rootCmd.Execute()

	// Close write ends so readers finish, then wait.
	_ = wOut.Close()
	_ = wErr.Close()
	wg.Wait()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return bufOut.String(), bufErr.String(), cmdErr
}

// executeCommandCLI runs rootCmd with args only — no implicit -o table injection.
// Use for assertions about real CLI defaults on commands that own their output flag.
func executeCommandCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	resetAllFlags(rootCmd)

	rootCmd.SetArgs(args)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SilenceUsage = false
		rootCmd.SilenceErrors = false
	})

	var bufOut, bufErr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&bufOut, rOut) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&bufErr, rErr) }()

	cmdErr := rootCmd.Execute()

	_ = wOut.Close()
	_ = wErr.Close()
	wg.Wait()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return bufOut.String(), bufErr.String(), cmdErr
}

// newTestServer creates an httptest.Server from a handler func.
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// resetAllFlags resets all flags on a command and its children to their default values.
func resetAllFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Set on a slice flag appends, so "[]" would become an element.
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetAllFlags(c)
	}
}

func containsOutputFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-o" || arg == "--output" {
			return true
		}
	}
	return false
}

func commandSupportsFlag(args []string, name string) bool {
	cmd, _, err := rootCmd.Find(args)
	if err != nil {
		return false
	}
	return cmd.Flags().Lookup(name) != nil
}
