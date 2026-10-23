// Command gen builds internal/spec/operations.json from the published
// CoinMarketCap OpenAPI spec.
//
//	go run ./internal/spec/gen -out internal/spec/operations.json
//
// -spec and -keyless-spec accept a URL or a local file. The keyless spec is
// only used to check that the x-keyless flags in the main spec agree with it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"
)

var methods = []string{"get", "post", "put", "patch", "delete"}

type openAPI struct {
	OpenAPI string `json:"openapi"`
	Info    struct {
		Version string `json:"version"`
	} `json:"info"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

type operation struct {
	OperationID string      `json:"operationId"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags"`
	Deprecated  bool        `json:"deprecated"`
	Keyless     bool        `json:"x-keyless"`
	Parameters  []parameter `json:"parameters"`
	RequestBody *struct{}   `json:"requestBody"`
}

type parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Schema      struct {
		Type    json.RawMessage `json:"type"`
		Enum    []any           `json:"enum"`
		Default any             `json:"default"`
	} `json:"schema"`
}

func main() {
	specRef := flag.String("spec", spec.SourceURL, "OpenAPI spec URL or file")
	keylessRef := flag.String("keyless-spec", spec.KeylessSourceURL, "Keyless OpenAPI spec URL or file (empty to skip the cross-check)")
	out := flag.String("out", "operations.json", "Output file")
	flag.Parse()

	if err := run(*specRef, *keylessRef, *out); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(specRef, keylessRef, out string) error {
	raw, doc, err := readSpec(specRef)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)

	index := spec.Index{
		Source:      specRef,
		SpecVersion: doc.Info.Version,
		SHA256:      hex.EncodeToString(sum[:]),
	}
	for path, item := range doc.Paths {
		for _, method := range methods {
			rawOp, ok := item[method]
			if !ok {
				continue
			}
			var op operation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return fmt.Errorf("%s %s: %w", strings.ToUpper(method), path, err)
			}
			index.Operations = append(index.Operations, convert(method, path, op))
		}
	}
	sort.Slice(index.Operations, func(i, j int) bool {
		a, b := index.Operations[i], index.Operations[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Method < b.Method
	})

	if keylessRef != "" {
		if err := checkKeyless(keylessRef, index.Operations); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), 0o644)
}

func convert(method, path string, op operation) spec.Operation {
	tag := ""
	if len(op.Tags) > 0 {
		tag = op.Tags[0]
	}
	summary := strings.TrimSpace(op.Summary)
	if summary == "" {
		summary = firstSentence(op.Description)
	}
	out := spec.Operation{
		OperationID: op.OperationID,
		Method:      strings.ToUpper(method),
		Path:        path,
		Summary:     summary,
		Tag:         tag,
		Keyless:     op.Keyless,
		Deprecated:  op.Deprecated || strings.EqualFold(tag, "Deprecated"),
		Body:        op.RequestBody != nil,
	}
	for _, p := range op.Parameters {
		if p.In != "query" {
			continue
		}
		param := spec.Param{
			Name:        p.Name,
			Type:        schemaType(p.Schema.Type),
			Required:    p.Required,
			Description: firstSentence(p.Description),
		}
		for _, e := range p.Schema.Enum {
			param.Enum = append(param.Enum, fmt.Sprint(e))
		}
		if p.Schema.Default != nil {
			param.Default = fmt.Sprint(p.Schema.Default)
		}
		out.Params = append(out.Params, param)
	}
	return out
}

// schemaType returns the first non-null type from a "type" that may be a
// string or an OpenAPI 3.1 type array.
func schemaType(raw json.RawMessage) string {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, t := range many {
			if t != "null" {
				return t
			}
		}
	}
	return "string"
}

// firstSentence keeps parameter help to one line: the first sentence of the
// first paragraph, with whitespace collapsed.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), " ")
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i+1]
	}
	return s
}

func checkKeyless(ref string, ops []spec.Operation) error {
	_, doc, err := readSpec(ref)
	if err != nil {
		return fmt.Errorf("keyless spec: %w", err)
	}
	want := map[string]bool{}
	for path, item := range doc.Paths {
		for _, method := range methods {
			if _, ok := item[method]; ok {
				want[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	var mismatches []string
	for _, op := range ops {
		key := op.Method + " " + op.Path
		if op.Keyless != want[key] {
			mismatches = append(mismatches, fmt.Sprintf("%s (x-keyless=%v, in keyless spec=%v)", key, op.Keyless, want[key]))
		}
		delete(want, key)
	}
	for key := range want {
		mismatches = append(mismatches, key+" (in keyless spec only)")
	}
	if len(mismatches) > 0 {
		sort.Strings(mismatches)
		return fmt.Errorf("x-keyless flags disagree with the keyless spec:\n  %s", strings.Join(mismatches, "\n  "))
	}
	return nil
}

func readSpec(ref string) ([]byte, *openAPI, error) {
	var raw []byte
	var err error
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		raw, err = fetch(ref)
	} else {
		raw, err = os.ReadFile(ref)
	}
	if err != nil {
		return nil, nil, err
	}
	var doc openAPI
	// The docs site answers unknown paths with an HTML page and status 200,
	// so check the content rather than trusting the status.
	if err := json.Unmarshal(raw, &doc); err != nil || doc.OpenAPI == "" || len(doc.Paths) == 0 {
		return nil, nil, fmt.Errorf("%s is not an OpenAPI document", ref)
	}
	return raw, &doc, nil
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
