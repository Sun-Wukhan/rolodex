package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/navid/rolodex/internal/domain"
)

const (
	maxResponseBytes = 1 << 20
	// tokenRefreshSkew refreshes tokens slightly before they expire so an
	// in-flight request never carries a token that expires mid-call.
	tokenRefreshSkew = 30 * time.Second
	defaultTokenTTL  = 5 * time.Minute
)

// Credentials are the service credentials used against a vendor's /auth
// endpoint. They are never logged or persisted.
type Credentials struct {
	Username string
	Password string
}

// String redacts the password if Credentials is ever formatted.
func (c Credentials) String() string {
	return fmt.Sprintf("{Username:%s Password:REDACTED}", c.Username)
}

// RetryPolicy bounds retries for transient failures.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryPolicy is three attempts with 200ms, 400ms backoff (plus jitter).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: 2 * time.Second}
}

// ClientConfig configures a vendor Client.
type ClientConfig struct {
	BaseURL     string
	Credentials Credentials
	Timeout     time.Duration
	Retry       RetryPolicy
	HTTPClient  *http.Client
}

// Client is a vendor HTTP client implementing the documented contract:
// POST /auth {username,password} -> {access_token, expires_in}, then
// authenticated JSON calls with Authorization: Bearer <token>.
type Client struct {
	baseURL string
	creds   Credentials
	http    *http.Client
	retry   RetryPolicy
	now     func() time.Time

	mu     sync.Mutex
	token  string
	expiry time.Time
}

// NewClient builds a Client, applying sane defaults.
func NewClient(cfg ClientConfig) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 3 * time.Second
		}
		hc = &http.Client{Timeout: timeout}
	}
	retry := cfg.Retry
	if retry.MaxAttempts <= 0 {
		retry = DefaultRetryPolicy()
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		creds:   cfg.Credentials,
		http:    hc,
		retry:   retry,
		now:     time.Now,
	}
}

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// PostJSON sends an authenticated JSON POST and decodes the response into out.
// A 401 triggers one token refresh and replay.
func (c *Client) PostJSON(ctx context.Context, path string, in, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := c.getToken(ctx)
		if err != nil {
			return err
		}
		status, body, err := c.send(ctx, path, in, tok)
		if err != nil {
			return err
		}
		switch {
		case status == http.StatusUnauthorized && attempt == 0:
			c.invalidate(tok)
			continue
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			return ErrAuth
		case status == http.StatusNotFound:
			return domain.ErrNotFound
		case status >= 200 && status < 300:
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("%w: %v", ErrBadResponse, err)
			}
			return nil
		default:
			return fmt.Errorf("%w: unexpected status %d", ErrBadResponse, status)
		}
	}
	return ErrAuth
}

// getToken returns a cached token or authenticates. The mutex also coalesces
// concurrent refreshes into a single /auth call.
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.expiry.Add(-tokenRefreshSkew)) {
		return c.token, nil
	}

	status, body, err := c.send(ctx, "/auth", map[string]string{
		"username": c.creds.Username, "password": c.creds.Password,
	}, "")
	if err != nil {
		return "", err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return "", ErrAuth
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("%w: auth status %d", ErrBadResponse, status)
	}
	var ar authResponse
	if err := json.Unmarshal(body, &ar); err != nil || ar.AccessToken == "" {
		return "", fmt.Errorf("%w: invalid auth response", ErrBadResponse)
	}
	ttl := time.Duration(ar.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	c.token, c.expiry = ar.AccessToken, c.now().Add(ttl)
	return c.token, nil
}

func (c *Client) invalidate(tok string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == tok {
		c.token = ""
	}
}

// send performs a POST with bounded retries on transport errors, 429 and 5xx.
// It returns the final status and body; exhausting retries yields
// ErrUnavailable.
func (c *Client) send(ctx context.Context, path string, in any, token string) (int, []byte, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return 0, nil, fmt.Errorf("provider: encode: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.retry.MaxAttempts; attempt++ {
		if attempt > 1 {
			if err := sleep(ctx, c.backoff(attempt-1)); err != nil {
				return 0, nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return 0, nil, fmt.Errorf("provider: request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, nil, fmt.Errorf("%w: %v", ErrUnavailable, ctx.Err())
			}
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}
		return resp.StatusCode, body, nil
	}
	return 0, nil, fmt.Errorf("%w: after %d attempts: %v", ErrUnavailable, c.retry.MaxAttempts, lastErr)
}

// backoff returns exponential delay with full jitter, capped at MaxDelay.
func (c *Client) backoff(retry int) time.Duration {
	d := c.retry.BaseDelay << (retry - 1)
	if d > c.retry.MaxDelay || d <= 0 {
		d = c.retry.MaxDelay
	}
	return d/2 + rand.N(d/2+1)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
