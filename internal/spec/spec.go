// Package spec exposes the operation index generated from the published
// CoinMarketCap OpenAPI spec. The index is embedded at build time, so the CLI
// knows every endpoint, its parameters and whether it works without an API key.
//
// Regenerate it with `make spec` (or `go generate ./internal/spec`); never edit
// operations.json by hand.
package spec

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:generate go run ./gen -out operations.json

const (
	// SourceURL is the published Pro API spec the index is generated from.
	SourceURL = "https://coinmarketcap.com/api/documentation/openapi.json"
	// KeylessSourceURL is the published Keyless Public API spec, used to cross-check keyless flags.
	KeylessSourceURL = "https://coinmarketcap.com/api/documentation/openapi-keyless.json"
)

// Index is the compact, CLI-facing view of the OpenAPI spec.
type Index struct {
	Source      string      `json:"source"`
	SpecVersion string      `json:"spec_version"`
	SHA256      string      `json:"sha256"`
	Operations  []Operation `json:"operations"`
}

// Operation is one documented endpoint.
type Operation struct {
	OperationID string  `json:"operation_id"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Summary     string  `json:"summary"`
	Tag         string  `json:"tag"`
	Keyless     bool    `json:"keyless"`
	Deprecated  bool    `json:"deprecated"`
	Body        bool    `json:"body,omitempty"`
	Params      []Param `json:"params,omitempty"`
}

// Param is one query parameter of an operation.
type Param struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Param returns the named parameter, if the operation documents it.
func (o Operation) Param(name string) (Param, bool) {
	for _, p := range o.Params {
		if p.Name == name {
			return p, true
		}
	}
	return Param{}, false
}

//go:embed operations.json
var indexJSON []byte

var (
	loadOnce sync.Once
	index    Index
	byPath   map[string]int
	byID     map[string]int
)

func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(indexJSON, &index); err != nil {
			panic(fmt.Sprintf("spec: embedded operations.json is invalid: %v", err))
		}
		byPath = make(map[string]int, len(index.Operations))
		byID = make(map[string]int, len(index.Operations))
		for i, op := range index.Operations {
			byPath[op.Path] = i
			byID[strings.ToLower(op.OperationID)] = i
		}
	})
}

// Info returns the index metadata and all operations.
func Info() Index {
	load()
	return index
}

// Operations returns every operation in the index, sorted by path.
func Operations() []Operation {
	load()
	return index.Operations
}

// Lookup finds an operation by request path. A query string and a missing
// leading slash are ignored.
func Lookup(path string) (Operation, bool) {
	load()
	i, ok := byPath[NormalizePath(path)]
	if !ok {
		return Operation{}, false
	}
	return index.Operations[i], true
}

// Find resolves a user reference: a path ("/v3/fear-and-greed/latest" or
// "v3/fear-and-greed/latest") or an operationId (case-insensitive).
func Find(ref string) (Operation, bool) {
	load()
	ref = strings.TrimSpace(ref)
	if op, ok := Lookup(ref); ok {
		return op, true
	}
	if i, ok := byID[strings.ToLower(ref)]; ok {
		return index.Operations[i], true
	}
	return Operation{}, false
}

// IsKeyless reports whether the spec documents the path as callable with no API key.
// Unknown paths report false.
func IsKeyless(path string) bool {
	op, ok := Lookup(path)
	return ok && op.Keyless
}

// NormalizePath strips the query string and ensures a single leading slash.
func NormalizePath(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
}
