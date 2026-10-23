package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/openCMC/CoinMarketCap-CLI/internal/api"
	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/display"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"
	"github.com/spf13/cobra"
)

const apiTableMaxColumns = 10

var apiCmd = &cobra.Command{
	Use:   "api [<path> | <operationId>]",
	Short: "Call any endpoint in the published OpenAPI spec",
	Long: `Call any endpoint documented in the published OpenAPI spec and print the
response unchanged, including fields the CLI does not model.

The endpoint is a path (/v3/fear-and-greed/latest) or an operationId
(getV3FearAndGreedLatest). Pass query parameters with -p name=value. With no
API key, only endpoints the spec marks as keyless can be called.

Spec: ` + spec.SourceURL,
	Example: `  cmc api /v3/fear-and-greed/latest
  cmc api getV1AltcoinSeasonIndexHistorical -p timeframe=30d
  cmc api /v3/index/cmc20-latest -o table
  cmc api /v1/dex/holders/list --body '{"platform":"ethereum","tokenAddress":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","tag":"tag_all"}'
  cmc api --list --keyless -o table
  cmc api --list real-world-assets`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAPI,
}

func init() {
	apiCmd.Flags().StringArrayP("param", "p", nil, "Query parameter as name=value (repeatable)")
	apiCmd.Flags().String("body", "", "JSON request body for POST endpoints, or @file to read it from a file")
	apiCmd.Flags().Bool("list", false, "List endpoints instead of calling one; an argument filters by path, operationId or summary")
	apiCmd.Flags().Bool("keyless", false, "With --list, show only endpoints that work without an API key")
	apiCmd.Flags().String("tag", "", "With --list, show only endpoints with this tag (for example \"CMC Index\")")
	apiCmd.Flags().Bool("include-deprecated", false, "With --list, include endpoints the spec files under Deprecated")
	addOutputFlag(apiCmd)
	addDryRunFlag(apiCmd)
	rootCmd.AddCommand(apiCmd)
}

func runAPI(cmd *cobra.Command, args []string) error {
	if list, _ := cmd.Flags().GetBool("list"); list {
		filter := ""
		if len(args) == 1 {
			filter = args[0]
		}
		return runAPIList(cmd, filter)
	}
	if len(args) == 0 {
		return fmt.Errorf("pass an endpoint path or operationId, or run `cmc api --list` to see them")
	}

	ref, query, err := parseAPIRef(args[0])
	if err != nil {
		return err
	}
	paramFlags, _ := cmd.Flags().GetStringArray("param")
	for _, p := range paramFlags {
		name, value, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return fmt.Errorf("--param must be name=value (got %q)", p)
		}
		query.Add(strings.TrimSpace(name), value)
	}

	op, known := spec.Find(ref)
	method, path := http.MethodGet, spec.NormalizePath(ref)
	switch {
	case known:
		method, path = op.Method, op.Path
		if err := checkAPIParams(op, query); err != nil {
			return err
		}
	case !strings.Contains(ref, "/"):
		return fmt.Errorf("unknown operationId %q; run `cmc api --list` to see the endpoints in the spec", ref)
	default:
		warnf("Warning: %s is not in the embedded spec; calling it anyway.\n", path)
	}

	body, err := readAPIBody(cmd)
	if err != nil {
		return err
	}
	if body != nil && method == http.MethodGet {
		return fmt.Errorf("--body is only for POST endpoints; %s is GET", path)
	}
	if body == nil && known && op.Body {
		return fmt.Errorf("%s %s needs a JSON body; pass it with --body", method, path)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.IsKeyless() && known && !op.Keyless {
		return api.KeyRequired(method + " " + path)
	}

	if isDryRun(cmd) {
		return printAPIDryRun(cfg, method, path, query, body, op)
	}

	full := path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	raw, err := newAPIClient(cfg).Raw(cmd.Context(), method, full, body)
	if err != nil {
		return err
	}
	if outputJSON(cmd) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
		compact.WriteByte('\n')
		_, err := os.Stdout.Write(compact.Bytes())
		return err
	}
	return renderAPITable(raw)
}

// parseAPIRef accepts a path, an operationId, a path with a query string, or a
// full URL copied from the docs (with or without /public-api).
func parseAPIRef(ref string) (string, url.Values, error) {
	ref = strings.TrimSpace(ref)
	if !strings.Contains(ref, "/") {
		return ref, url.Values{}, nil
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", nil, fmt.Errorf("invalid endpoint %q: %w", ref, err)
	}
	path := strings.TrimPrefix(spec.NormalizePath(u.Path), config.KeylessPathPrefix)
	return spec.NormalizePath(path), u.Query(), nil
}

// checkAPIParams rejects missing required parameters. Unknown names and values
// outside the documented enum only warn, so parameters added to the API after
// this build still work.
func checkAPIParams(op spec.Operation, query url.Values) error {
	var missing []string
	for _, p := range op.Params {
		if p.Required && query.Get(p.Name) == "" {
			missing = append(missing, p.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s needs %s; pass them with -p name=value", op.Path, strings.Join(missing, ", "))
	}
	for name, values := range query {
		p, ok := op.Param(name)
		if !ok {
			warnf("Warning: %s does not document parameter %q; sending it anyway.\n", op.Path, name)
			continue
		}
		if len(p.Enum) == 0 {
			continue
		}
		for _, v := range values {
			if !containsString(p.Enum, v) {
				warnf("Warning: %s=%q is not one of %s; sending it anyway.\n", name, v, strings.Join(p.Enum, ", "))
			}
		}
	}
	return nil
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func readAPIBody(cmd *cobra.Command) ([]byte, error) {
	value, _ := cmd.Flags().GetString("body")
	if value == "" {
		return nil, nil
	}
	body := []byte(value)
	if strings.HasPrefix(value, "@") {
		b, err := os.ReadFile(strings.TrimPrefix(value, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading --body: %w", err)
		}
		body = b
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("--body must be valid JSON")
	}
	return body, nil
}

func printAPIDryRun(cfg *config.Config, method, path string, query url.Values, body []byte, op spec.Operation) error {
	params := make(map[string]string, len(query))
	for name := range query {
		params[name] = strings.Join(query[name], ",")
	}
	out := dryRunOutput{
		Method:         method,
		URL:            cfg.BaseURL() + path,
		Params:         params,
		Headers:        buildDryRunHeaders(cfg),
		Body:           body,
		OASOperationID: op.OperationID,
		OASSpec:        spec.SourceURL,
	}
	if body != nil {
		out.Headers["Content-Type"] = "application/json"
	}
	return printJSONRaw(out)
}

func runAPIList(cmd *cobra.Command, filter string) error {
	keylessOnly, _ := cmd.Flags().GetBool("keyless")
	tag, _ := cmd.Flags().GetString("tag")
	includeDeprecated, _ := cmd.Flags().GetBool("include-deprecated")
	filter = strings.ToLower(strings.TrimSpace(filter))

	var ops []spec.Operation
	for _, op := range spec.Operations() {
		switch {
		case op.Deprecated && !includeDeprecated:
			continue
		case keylessOnly && !op.Keyless:
			continue
		case tag != "" && !strings.EqualFold(op.Tag, tag):
			continue
		case filter != "" &&
			!strings.Contains(strings.ToLower(op.Path), filter) &&
			!strings.Contains(strings.ToLower(op.OperationID), filter) &&
			!strings.Contains(strings.ToLower(op.Summary), filter):
			continue
		}
		ops = append(ops, op)
	}

	if outputJSON(cmd) {
		if ops == nil {
			ops = []spec.Operation{}
		}
		return printJSONRaw(ops)
	}

	rows := make([][]string, 0, len(ops))
	for _, op := range ops {
		keyless := ""
		if op.Keyless {
			keyless = "yes"
		}
		rows = append(rows, []string{op.Method, op.Path, keyless, op.Tag, display.SanitizeCell(op.Summary)})
	}
	display.PrintTable([]string{"Method", "Path", "Keyless", "Tag", "Summary"}, rows)
	return nil
}

// renderAPITable prints the response's data as a table: one row per item for
// an array of objects, or field/value rows for an object. Nested values are
// summarized; use -o json for the full response.
func renderAPITable(raw []byte) error {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	data, ok := doc["data"]
	if !ok {
		data = raw
	}

	var items []map[string]json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		if len(items) == 0 {
			fmt.Println("No results.")
			return nil
		}
		headers := apiTableColumns(items[0])
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			row := make([]string, len(headers))
			for i, h := range headers {
				row[i] = apiCell(item[h])
			}
			rows = append(rows, row)
		}
		display.PrintTable(headers, rows)
		return nil
	}

	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([][]string, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, []string{k, apiCell(obj[k])})
		}
		display.PrintTable([]string{"Field", "Value"}, rows)
		return nil
	}

	fmt.Println(apiCell(data))
	return nil
}

// apiTableColumns picks the scalar fields of the first item, identity fields first.
func apiTableColumns(item map[string]json.RawMessage) []string {
	preferred := []string{"id", "name", "symbol", "slug"}
	var cols []string
	for _, k := range preferred {
		if v, ok := item[k]; ok && isScalarJSON(v) {
			cols = append(cols, k)
		}
	}
	var rest []string
	for k, v := range item {
		if !containsString(preferred, k) && isScalarJSON(v) {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	cols = append(cols, rest...)
	if len(cols) > apiTableMaxColumns {
		cols = cols[:apiTableMaxColumns]
	}
	return cols
}

func isScalarJSON(v json.RawMessage) bool {
	v = bytes.TrimSpace(v)
	return len(v) > 0 && v[0] != '{' && v[0] != '['
}

func apiCell(v json.RawMessage) string {
	v = bytes.TrimSpace(v)
	if len(v) == 0 || string(v) == "null" {
		return ""
	}
	switch v[0] {
	case '"':
		var s string
		_ = json.Unmarshal(v, &s)
		return display.SanitizeCell(s)
	case '[':
		var items []json.RawMessage
		_ = json.Unmarshal(v, &items)
		return fmt.Sprintf("[%d items]", len(items))
	case '{':
		return "{…}"
	default:
		return string(v)
	}
}
