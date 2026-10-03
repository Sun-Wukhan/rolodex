package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/internal/mockvendor"
	"github.com/navid/rolodex/internal/provider"
	"github.com/navid/rolodex/internal/provider/abc"
	"github.com/navid/rolodex/internal/provider/xyc"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func fastRetry() provider.RetryPolicy {
	return provider.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
}

func clientCfg(url string) provider.ClientConfig {
	return provider.ClientConfig{
		BaseURL: url, Credentials: provider.Credentials{Username: "svc", Password: "secret"},
		Timeout: time.Second, Retry: fastRetry(),
	}
}

func vendor(t *testing.T, f mockvendor.Format, recs []mockvendor.Record) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(mockvendor.New(mockvendor.Config{Format: f, Username: "svc", Password: "secret"}, recs, quiet).Handler())
	t.Cleanup(s.Close)
	return s
}

func TestABCAdapterMapsSpecFormat(t *testing.T) {
	srv := vendor(t, mockvendor.FormatABC, mockvendor.ABCRecords())
	p := abc.New(clientCfg(srv.URL))
	id, err := p.Lookup(context.Background(), domain.IdentityQuery{Phone: "4165550101"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Provider != "abc" || id.Name != "Ada Lovelace" || id.Phone != "+14165550101" || id.Address.PostalCode != "M5X 1A9" {
		t.Fatalf("unexpected identity %+v", id)
	}
	if _, err := p.Lookup(context.Background(), domain.IdentityQuery{Name: "Nobody"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestXYCAdapterNormalisesVendorFormat(t *testing.T) {
	srv := vendor(t, mockvendor.FormatXYC, mockvendor.XYCRecords())
	p := xyc.New(clientCfg(srv.URL))
	id, err := p.Lookup(context.Background(), domain.IdentityQuery{Name: "alan turing"})
	if err != nil {
		t.Fatal(err)
	}
	if id.Provider != "xyc" || id.Phone != "+442079460103" || id.Address.Locality != "Milton Keynes" || id.Address.Country != "GB" {
		t.Fatalf("unexpected identity %+v", id)
	}
}

func TestXYCMissingPersonIsBadResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 60})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := xyc.New(clientCfg(srv.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "x"})
	if !errors.Is(err, provider.ErrBadResponse) {
		t.Fatalf("want ErrBadResponse, got %v", err)
	}
}

func TestBadServiceCredentials(t *testing.T) {
	srv := vendor(t, mockvendor.FormatABC, nil)
	cfg := clientCfg(srv.URL)
	cfg.Credentials.Password = "wrong"
	_, err := abc.New(cfg).Lookup(context.Background(), domain.IdentityQuery{Name: "x"})
	if !errors.Is(err, provider.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

// scripted is a vendor whose /identity responses follow a fixed sequence.
type scripted struct {
	authCalls     atomic.Int32
	identityCalls atomic.Int32
	statuses      []int
	tokenTTL      int
}

func (s *scripted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth" {
		n := s.authCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-" + string(rune('0'+n)), "expires_in": s.tokenTTL})
		return
	}
	i := int(s.identityCalls.Add(1)) - 1
	status := http.StatusOK
	if i < len(s.statuses) {
		status = s.statuses[i]
	}
	w.WriteHeader(status)
	if status == http.StatusOK {
		_, _ = w.Write([]byte(`{"name":"Ada","phone":"4165550101","address":{}}`))
	}
}

func TestRetriesTransientFailures(t *testing.T) {
	s := &scripted{statuses: []int{503, 502}, tokenTTL: 300}
	srv := httptest.NewServer(s)
	defer srv.Close()

	id, err := abc.New(clientCfg(srv.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"})
	if err != nil || id.Name != "Ada" {
		t.Fatalf("expected success after retries: %v", err)
	}
	if got := s.identityCalls.Load(); got != 3 {
		t.Fatalf("identity calls = %d, want 3", got)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	s := &scripted{statuses: []int{503, 503, 503, 503}, tokenTTL: 300}
	srv := httptest.NewServer(s)
	defer srv.Close()

	_, err := abc.New(clientCfg(srv.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"})
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if got := s.identityCalls.Load(); got != 3 {
		t.Fatalf("identity calls = %d, want exactly 3 (bounded)", got)
	}
}

func TestDoesNotRetryClientErrors(t *testing.T) {
	s := &scripted{statuses: []int{400}, tokenTTL: 300}
	srv := httptest.NewServer(s)
	defer srv.Close()

	_, err := abc.New(clientCfg(srv.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"})
	if !errors.Is(err, provider.ErrBadResponse) || s.identityCalls.Load() != 1 {
		t.Fatalf("4xx must not be retried: err=%v calls=%d", err, s.identityCalls.Load())
	}
}

func TestReauthenticatesOnceOn401(t *testing.T) {
	s := &scripted{statuses: []int{401}, tokenTTL: 300}
	srv := httptest.NewServer(s)
	defer srv.Close()

	if _, err := abc.New(clientCfg(srv.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"}); err != nil {
		t.Fatal(err)
	}
	if s.authCalls.Load() != 2 || s.identityCalls.Load() != 2 {
		t.Fatalf("auth=%d identity=%d, want 2/2", s.authCalls.Load(), s.identityCalls.Load())
	}

	s2 := &scripted{statuses: []int{401, 401}, tokenTTL: 300}
	srv2 := httptest.NewServer(s2)
	defer srv2.Close()
	if _, err := abc.New(clientCfg(srv2.URL)).Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"}); !errors.Is(err, provider.ErrAuth) {
		t.Fatalf("persistent 401 should be ErrAuth, got %v", err)
	}
}

func TestTokenIsCachedAndRefreshedNearExpiry(t *testing.T) {
	cached := &scripted{tokenTTL: 300}
	srv := httptest.NewServer(cached)
	defer srv.Close()
	p := abc.New(clientCfg(srv.URL))
	for i := 0; i < 3; i++ {
		if _, err := p.Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"}); err != nil {
			t.Fatal(err)
		}
	}
	if cached.authCalls.Load() != 1 {
		t.Fatalf("token should be cached, auth calls = %d", cached.authCalls.Load())
	}

	// A 10s TTL is inside the 30s refresh skew, so every call re-authenticates.
	short := &scripted{tokenTTL: 10}
	srv2 := httptest.NewServer(short)
	defer srv2.Close()
	p2 := abc.New(clientCfg(srv2.URL))
	for i := 0; i < 2; i++ {
		_, _ = p2.Lookup(context.Background(), domain.IdentityQuery{Name: "Ada"})
	}
	if short.authCalls.Load() != 2 {
		t.Fatalf("near-expiry token should refresh, auth calls = %d", short.authCalls.Load())
	}
}

func TestContextCancellationStopsRetries(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 60})
			return
		}
		time.Sleep(200 * time.Millisecond)
	}))
	defer slow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := abc.New(clientCfg(slow.URL)).Lookup(ctx, domain.IdentityQuery{Name: "Ada"})
	if !errors.Is(err, provider.ErrUnavailable) || time.Since(start) > 150*time.Millisecond {
		t.Fatalf("want fast ErrUnavailable, got %v after %v", err, time.Since(start))
	}
}

func TestCredentialsRedacted(t *testing.T) {
	c := provider.Credentials{Username: "svc", Password: "hunter2"}
	if s := c.String(); s != "{Username:svc Password:REDACTED}" {
		t.Fatalf("got %q", s)
	}
}
