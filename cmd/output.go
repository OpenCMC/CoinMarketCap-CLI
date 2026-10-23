package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/export"

	"github.com/spf13/cobra"
)

// CLIError is the structured JSON error format emitted to stderr when -o json is set.
type CLIError struct {
	Error      string              `json:"error"`
	Message    string              `json:"message"`
	RetryAfter *int                `json:"retry_after,omitempty"`
	Candidates []api.ResolvedAsset `json:"candidates,omitempty"`
}

type renderedError struct {
	err error
}

func (e *renderedError) Error() string {
	return e.err.Error()
}

func (e *renderedError) Unwrap() error {
	return e.err
}

func markErrorRendered(err error) error {
	if err == nil {
		return nil
	}
	return &renderedError{err: err}
}

func printJSONRaw(v any) error {
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func exportCSV(path string, headers []string, rows [][]string) error {
	if err := export.ExportCSV(path, headers, rows); err != nil {
		return err
	}
	warnf("Exported to %s\n", path)
	return nil
}

// formatError writes a structured JSON error to stderr when -o json is active,
// otherwise returns the error unchanged for Cobra's default plain text handling.
func formatError(cmd *cobra.Command, err error) error {
	if !outputJSON(cmd) {
		return err
	}
	var rendered *renderedError
	if errors.As(err, &rendered) {
		return rendered.Unwrap()
	}

	cliErr := classifyError(err)
	enc := json.NewEncoder(os.Stderr)
	_ = enc.Encode(cliErr)
	return err
}

func classifyError(err error) CLIError {
	var rle *api.RateLimitError
	if errors.As(err, &rle) {
		ce := CLIError{Error: "rate_limited", Message: rle.Error()}
		if rle.RetryAfter > 0 {
			ce.RetryAfter = &rle.RetryAfter
		}
		return ce
	}
	if errors.Is(err, api.ErrInvalidAPIKey) {
		return CLIError{Error: "invalid_api_key", Message: err.Error()}
	}
	if errors.Is(err, api.ErrKeyRequired) {
		return CLIError{Error: "api_key_required", Message: err.Error()}
	}
	if errors.Is(err, api.ErrPlanRestricted) {
		return CLIError{Error: "plan_restricted", Message: err.Error()}
	}
	if errors.Is(err, api.ErrAssetNotFound) {
		return CLIError{Error: "asset_not_found", Message: err.Error()}
	}
	var ambig *api.ResolverAmbiguityError
	if errors.As(err, &ambig) {
		return CLIError{
			Error:      "resolver_ambiguous",
			Message:    err.Error(),
			Candidates: ambig.Candidates,
		}
	}
	if errors.Is(err, api.ErrResolverAmbiguous) {
		return CLIError{Error: "resolver_ambiguous", Message: err.Error()}
	}
	if errors.Is(err, api.ErrInvalidInput) {
		return CLIError{Error: "invalid_input", Message: err.Error()}
	}
	return CLIError{Error: "error", Message: err.Error()}
}
