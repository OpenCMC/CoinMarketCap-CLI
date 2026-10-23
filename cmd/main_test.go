package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps the tests away from the developer's real config file and
// CMC_API_KEY, so a local run sees what CI sees: no key, keyless mode.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "cmc-cli-test-home")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	_ = os.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	_ = os.Unsetenv("CMC_API_KEY")
	_ = os.Unsetenv("CMC_API_TIER")

	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
