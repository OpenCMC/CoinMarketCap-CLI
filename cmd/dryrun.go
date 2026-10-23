package cmd

import (
	"encoding/json"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"
	"github.com/spf13/cobra"
)

func addDryRunFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "Preview API request without executing (JSON output)")
}

func isDryRun(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool("dry-run")
	return err == nil && v
}

type dryRunOutput struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Params         map[string]string `json:"params"`
	Headers        map[string]string `json:"headers"`
	Body           json.RawMessage   `json:"body,omitempty"`
	OASOperationID string            `json:"oas_operation_id,omitempty"`
	OASSpec        string            `json:"oas_spec,omitempty"`
	Note           string            `json:"note,omitempty"`
	Pagination     *paginationInfo   `json:"pagination"`
}

type paginationInfo struct {
	TotalRequested int `json:"total_requested"`
	PerPage        int `json:"per_page"`
	Pages          int `json:"pages"`
}

func printDryRun(cfg *config.Config, cmdName, endpoint string, params map[string]string, pagination *paginationInfo) error {
	return printDryRunWithOp(cfg, cmdName, "", endpoint, params, pagination)
}

func printDryRunWithOp(cfg *config.Config, cmdName, opKey, endpoint string, params map[string]string, pagination *paginationInfo) error {
	return printDryRunFull(cfg, cmdName, opKey, endpoint, params, pagination, "")
}

func printDryRunFull(cfg *config.Config, cmdName, opKey, endpoint string, params map[string]string, pagination *paginationInfo, note string) error {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}
	headers["Accept"] = "application/json"
	headers["User-Agent"] = userAgent

	out := dryRunOutput{
		Method:     "GET",
		URL:        cfg.BaseURL() + endpoint,
		Params:     params,
		Headers:    headers,
		Note:       note,
		Pagination: pagination,
	}

	if _, ok := commandMeta[cmdName]; ok {
		out.OASSpec = spec.SourceURL
		out.OASOperationID = operationID(endpoint)
	}

	return printJSONRaw(out)
}

// printDryRunMulti prints multiple dry-run requests as an array.
func printDryRunMulti(cfg *config.Config, cmdName string, requests []struct {
	opKey    string
	endpoint string
	params   map[string]string
}) error {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}
	headers["Accept"] = "application/json"
	headers["User-Agent"] = userAgent

	outs := make([]dryRunOutput, len(requests))
	for i, req := range requests {
		outs[i] = dryRunOutput{
			Method:  "GET",
			URL:     cfg.BaseURL() + req.endpoint,
			Params:  req.params,
			Headers: headers,
		}
		if _, ok := commandMeta[cmdName]; ok {
			outs[i].OASSpec = spec.SourceURL
			outs[i].OASOperationID = operationID(req.endpoint)
		}
	}

	return printJSONRaw(outs)
}
