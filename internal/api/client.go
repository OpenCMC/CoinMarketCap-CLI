package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openCMC/CoinMarketCap-CLI/internal/config"
	"github.com/openCMC/CoinMarketCap-CLI/internal/spec"
)

const (
	maxErrorBodySize    = 1 << 20  // 1MB
	maxResponseBodySize = 50 << 20 // 50MB — guards against pathological upstream responses

	// keylessRetryBudget caps the total time spent waiting on 429s in keyless
	// mode: 1s + 2s + 4s + 8s, as the keyless docs recommend.
	keylessRetryBudget = 15 * time.Second
	keylessFirstRetry  = time.Second
)

var (
	ErrInvalidAPIKey     = fmt.Errorf("invalid API key — check your key with `cmc status` or set a new one with `cmc auth`")
	ErrPlanRestricted    = fmt.Errorf("this endpoint requires a higher-tier plan — visit https://coinmarketcap.com/api/pricing/")
	ErrRateLimited       = fmt.Errorf("rate limited — please wait and try again")
	ErrAssetNotFound     = fmt.Errorf("asset not found")
	ErrResolverAmbiguous = fmt.Errorf("multiple assets match symbol")
	ErrInvalidInput      = fmt.Errorf("invalid request parameters")
	ErrKeyRequired       = fmt.Errorf("needs a free API key — run `cmc auth` to add one (get a key at https://coinmarketcap.com/api/pricing/)")
)

// KeyRequired reports that subject (a command or an endpoint) is not served by
// the Keyless Public API.
func KeyRequired(subject string) error {
	return fmt.Errorf("%s %w", subject, ErrKeyRequired)
}

// RateLimitError carries rate-limit metadata from a 429 response.
type RateLimitError struct {
	RetryAfter int       // seconds from Retry-After header; 0 if absent
	ResetAt    time.Time // from x-ratelimit-reset header; zero if absent
	Keyless    bool      // the shared keyless pool was exhausted after retries
}

func (e *RateLimitError) Error() string {
	if e.Keyless {
		return "rate limited — keyless requests share an IP-based limit; a free API key gives you your own, higher limits (run `cmc auth`)"
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited — retry after %d seconds", e.RetryAfter)
	}
	if !e.ResetAt.IsZero() {
		return fmt.Sprintf("rate limited — resets at %s", e.ResetAt.UTC().Format("15:04:05 UTC"))
	}
	return "rate limited — please wait and try again"
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// apiErrorResponse covers CMC's error JSON format:
//
//	{"status": {"error_code": N, "error_message": "..."}}
type apiErrorResponse struct {
	Status *struct {
		ErrorCode    errorCode `json:"error_code"`
		ErrorMessage string    `json:"error_message"`
	} `json:"status"`
	Data json.RawMessage `json:"data"`
}

// errorCode accepts both encodings the API uses for status.error_code:
// 1001 (Node endpoints) and "1001" (Java and DEX endpoints).
type errorCode int

func (e *errorCode) UnmarshalJSON(b []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	if err != nil {
		*e = 0
		return nil
	}
	*e = errorCode(n)
	return nil
}

// errorCodeKeyRequired is returned by the keyless API for endpoints it does not serve.
const errorCodeKeyRequired = 1005

// extractMessage returns the error message from the response body.
func (e *apiErrorResponse) extractMessage() string {
	if e.Status != nil && e.Status.ErrorMessage != "" {
		return e.Status.ErrorMessage
	}
	return ""
}

// classify401 determines whether a 401 response is an auth failure.
// CMC returns 401 for invalid/missing API keys (error codes 1001, 1002).
func classify401(apiErr apiErrorResponse, msg string) error {
	code := 0
	if apiErr.Status != nil {
		code = int(apiErr.Status.ErrorCode)
	}
	lower := strings.ToLower(msg)

	// CMC auth failures: error codes 1001 (invalid), 1002 (missing)
	if code == 1001 || code == 1002 || isAuthFailure(lower) {
		if msg != "" {
			return fmt.Errorf("%s (%w)", msg, ErrInvalidAPIKey)
		}
		return ErrInvalidAPIKey
	}

	// Unknown 401 — return generic error or assume auth
	if msg != "" {
		return fmt.Errorf("API error 401: %s", msg)
	}
	return ErrInvalidAPIKey
}

func isAuthFailure(lowerMsg string) bool {
	for _, phrase := range []string{
		"invalid api key",
		"api key missing",
		"api key is invalid",
	} {
		if strings.Contains(lowerMsg, phrase) {
			return true
		}
	}
	return false
}

func isAssetNotFound(lowerMsg string) bool {
	for _, phrase := range []string{
		"no data found",
		"not found",
		"could not find",
	} {
		if strings.Contains(lowerMsg, phrase) {
			return true
		}
	}
	return false
}

func isInvalidInput(lowerMsg string) bool {
	for _, phrase := range []string{
		"invalid",
		"required",
	} {
		if strings.Contains(lowerMsg, phrase) {
			return true
		}
	}
	return false
}

type Client struct {
	http       *http.Client
	baseURLVal string // override; empty = use cfg.BaseURL()
	cfg        *config.Config
	UserAgent  string // sent with every request; set by cmd layer

	retryBudget time.Duration                              // keyless 429 retry budget
	sleep       func(context.Context, time.Duration) error // overridden in tests
}

func NewClient(cfg *config.Config) *Client {
	return NewClientWithHTTP(cfg, &http.Client{Timeout: 30 * time.Second})
}

func NewClientWithHTTP(cfg *config.Config, httpClient *http.Client) *Client {
	return &Client{
		http:        httpClient,
		cfg:         cfg,
		retryBudget: keylessRetryBudget,
		sleep:       sleepContext,
	}
}

// SetBaseURL overrides the API host. In keyless mode the /public-api prefix is
// still added, so tests see the same paths as production.
func (c *Client) SetBaseURL(url string) {
	c.baseURLVal = url
}

func (c *Client) baseURL() string {
	if c.baseURLVal != "" {
		if c.cfg.IsKeyless() {
			return c.baseURLVal + config.KeylessPathPrefix
		}
		return c.baseURLVal
	}
	return c.cfg.BaseURL()
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	raw, err := c.Raw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}

// Raw sends a request and returns the response body unchanged, so callers can
// pass through fields the CLI does not model. path may include a query string.
//
// In keyless mode, endpoints the spec does not mark as keyless fail with
// ErrKeyRequired before any request is sent, and 429s are retried with
// exponential backoff.
func (c *Client) Raw(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.cfg.IsKeyless() {
		if op, ok := spec.Lookup(path); ok && !op.Keyless {
			return nil, KeyRequired(op.Path)
		}
	}

	var waited time.Duration
	for attempt := 0; ; attempt++ {
		raw, err := c.send(ctx, method, path, body)
		var rle *RateLimitError
		if !c.cfg.IsKeyless() || !errors.As(err, &rle) {
			return raw, err
		}
		wait := keylessFirstRetry << attempt
		if after := time.Duration(rle.RetryAfter) * time.Second; after > wait {
			wait = after
		}
		if waited+wait > c.retryBudget {
			rle.Keyless = true
			return nil, rle
		}
		if err := c.sleep(ctx, wait); err != nil {
			return nil, err
		}
		waited += wait
	}
}

func (c *Client) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.cfg.ApplyAuth(req)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if len(raw) > maxResponseBodySize {
		return nil, fmt.Errorf("response larger than %d MB", maxResponseBodySize>>20)
	}
	if err := envelopeError(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// envelopeError catches errors the API reports with HTTP 200, such as an
// unknown path: a non-zero status.error_code and no data.
func envelopeError(raw []byte) error {
	var env apiErrorResponse
	if json.Unmarshal(raw, &env) != nil || env.Status == nil || env.Status.ErrorCode == 0 {
		return nil
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		return nil
	}
	msg := env.Status.ErrorMessage
	if msg == "" {
		msg = "unknown error"
	}
	return fmt.Errorf("API error %d: %s", env.Status.ErrorCode, msg)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))

	// Parse CMC error body.
	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)
	msg := apiErr.extractMessage()
	lower := strings.ToLower(msg)

	switch resp.StatusCode {
	case http.StatusBadRequest:
		switch {
		case isAssetNotFound(lower):
			if msg != "" {
				return fmt.Errorf("%s (%w)", msg, ErrAssetNotFound)
			}
			return ErrAssetNotFound
		case isInvalidInput(lower):
			if msg != "" {
				return fmt.Errorf("%s (%w)", msg, ErrInvalidInput)
			}
			return ErrInvalidInput
		default:
			if msg != "" {
				return fmt.Errorf("API error 400: %s", msg)
			}
			return fmt.Errorf("API error 400: %s", string(body))
		}

	case http.StatusUnauthorized:
		return classify401(apiErr, msg)

	case http.StatusForbidden:
		// The keyless API returns 403 / 1005 for endpoints it does not serve.
		if apiErr.Status != nil && apiErr.Status.ErrorCode == errorCodeKeyRequired {
			return KeyRequired(resp.Request.URL.Path)
		}
		// CMC returns 403 for plan restrictions (error code 1003)
		if msg != "" {
			return fmt.Errorf("%s (%w)", msg, ErrPlanRestricted)
		}
		return ErrPlanRestricted

	case http.StatusNotFound:
		if isAssetNotFound(lower) {
			if msg != "" {
				return fmt.Errorf("%s (%w)", msg, ErrAssetNotFound)
			}
			return ErrAssetNotFound
		}
		if msg != "" {
			return fmt.Errorf("API error 404: %s", msg)
		}
		return fmt.Errorf("API error 404: %s", string(body))

	case http.StatusTooManyRequests:
		rle := &RateLimitError{}
		if retry := resp.Header.Get("Retry-After"); retry != "" {
			if secs, err := strconv.Atoi(retry); err == nil && secs > 0 {
				rle.RetryAfter = secs
			}
		}
		if reset := resp.Header.Get("x-ratelimit-reset"); reset != "" {
			// CMC may send timestamp format
			if t, err := time.Parse("2006-01-02 15:04:05 -0700", reset); err == nil {
				rle.ResetAt = t
			}
		}
		return rle

	default:
		if msg != "" {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
}

func (c *Client) requirePaid() error {
	if !c.cfg.IsPaid() {
		return ErrPlanRestricted
	}
	return nil
}
