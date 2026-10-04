package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/httpapi"
	"github.com/Sun-Wukhan/rolodex/internal/provider"
	"github.com/Sun-Wukhan/rolodex/internal/repository/factory"
	"github.com/Sun-Wukhan/rolodex/internal/security"
	"github.com/Sun-Wukhan/rolodex/internal/service"
)

type stubProvider struct{}

func (stubProvider) Name() string { return "abc" }
func (stubProvider) Lookup(context.Context, domain.IdentityQuery) (*domain.Identity, error) {
	return &domain.Identity{Provider: "abc", Name: "Ada Lovelace", Address: domain.Address{PostalCode: "M5V 2T6"}}, nil
}

type downPinger struct{}

func (downPinger) Ping(context.Context) error { return errors.New("down") }

type env struct {
	srv    *httptest.Server
	userID string
}

func setup(t *testing.T, ready httpapi.Pinger) *env {
	t.Helper()
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	repo, err := factory.Open(ctx, factory.DriverSQLite, filepath.Join(dir, "profiles.db"), filepath.Join(dir, "credentials.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	hasher := security.NewArgon2Hasher(security.Argon2Params{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	tokens, _ := security.NewTokenManager([]byte(strings.Repeat("k", 32)), "rolodex", time.Minute)
	auth, _ := service.NewAuthService(repo, hasher, tokens, log)
	profiles := service.NewProfileService(repo, hasher)
	identity := service.NewIdentityService(repo, []provider.IdentityProvider{stubProvider{}}, time.Second, log)

	u, err := profiles.CreateUser(ctx, service.CreateUserInput{
		Name: "Ada Lovelace", Phone: "4165550100", Username: "ada", Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ready == nil {
		ready = repo
	}
	h := httpapi.NewRouter(httpapi.Deps{
		Auth: auth, Profiles: profiles, Identity: identity, Ready: ready, Tokens: tokens, Log: log,
		CORSAllowedOrigins: []string{"http://localhost:5173"}, LoginRatePerMinute: 5,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &env{srv: srv, userID: u.ID.String()}
}

func (e *env) do(t *testing.T, method, path, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func (e *env) login(t *testing.T) string {
	t.Helper()
	resp, body := e.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "ada", "password": "correct-horse-battery"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status %d: %v", resp.StatusCode, body)
	}
	return body["access_token"].(string)
}

func TestLoginAndAuthenticatedFlow(t *testing.T) {
	e := setup(t, nil)
	tok := e.login(t)

	resp, body := e.do(t, http.MethodGet, "/api/v1/me", tok, nil)
	if resp.StatusCode != 200 || body["username"] != "ada" {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}

	resp, body = e.do(t, http.MethodGet, "/api/v1/users?name=ada", tok, nil)
	if resp.StatusCode != 200 || len(body["data"].([]any)) != 1 {
		t.Fatalf("search: %d %v", resp.StatusCode, body)
	}

	resp, body = e.do(t, http.MethodGet, "/api/v1/users/"+e.userID, tok, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get: %d %v", resp.StatusCode, body)
	}
	creds := body["credentials"].([]any)
	if _, leaked := creds[0].(map[string]any)["secret_hash"]; leaked {
		t.Fatal("secret hash leaked in response")
	}

	resp, body = e.do(t, http.MethodPost, "/api/v1/users/"+e.userID+"/enrich?provider=abc", tok, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("enrich: %d %v", resp.StatusCode, body)
	}
	postal := body["fields"].(map[string]any)["postal_code"].(map[string]any)
	if postal["source"] != "abc" {
		t.Fatalf("enrich provenance: %v", postal)
	}

	resp, body = e.do(t, http.MethodGet, "/api/v1/providers", tok, nil)
	if resp.StatusCode != 200 || len(body["providers"].([]any)) != 1 {
		t.Fatalf("providers: %d %v", resp.StatusCode, body)
	}
}

func TestCreateUser(t *testing.T) {
	e := setup(t, nil)
	tok := e.login(t)
	in := map[string]any{"name": "Grace Hopper", "phone": "+1 212 555 0199", "username": "grace", "password": "a-very-long-password"}

	resp, body := e.do(t, http.MethodPost, "/api/v1/users", tok, in)
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Location") == "" {
		t.Fatalf("create: %d %v", resp.StatusCode, body)
	}
	resp, _ = e.do(t, http.MethodPost, "/api/v1/users", tok, in)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate: want 409, got %d", resp.StatusCode)
	}
	in["username"], in["password"] = "x", "short"
	resp, body = e.do(t, http.MethodPost, "/api/v1/users", tok, in)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("validation: %d", resp.StatusCode)
	}
	fields := body["error"].(map[string]any)["fields"].(map[string]any)
	if fields["password"] == nil || fields["username"] == nil {
		t.Fatalf("field errors: %v", fields)
	}
	resp, _ = e.do(t, http.MethodPost, "/api/v1/users", tok, map[string]any{"name": "x", "is_admin": true})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown fields must be rejected, got %d", resp.StatusCode)
	}
}

func TestAuthFailures(t *testing.T) {
	e := setup(t, nil)
	cases := []struct {
		name, token string
	}{{"missing", ""}, {"garbage", "not-a-jwt"}}
	for _, c := range cases {
		resp, body := e.do(t, http.MethodGet, "/api/v1/users?name=a", c.token, nil)
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") == "" {
			t.Errorf("%s: %d %v", c.name, resp.StatusCode, body)
		}
	}
	resp, body := e.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "ada", "password": "nope"})
	if resp.StatusCode != http.StatusUnauthorized || body["error"].(map[string]any)["request_id"] == "" {
		t.Fatalf("bad password: %d %v", resp.StatusCode, body)
	}
}

func TestLoginRateLimited(t *testing.T) {
	e := setup(t, nil)
	var last int
	for i := 0; i < 7; i++ {
		// Rotating spoofed forwarding headers must not create fresh buckets.
		req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/api/v1/auth/login",
			strings.NewReader(`{"username":"ada","password":"nope"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i))
		req.Header.Set("X-Real-IP", fmt.Sprintf("198.51.100.%d", i))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		last = resp.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("want 429 after limit despite spoofed headers, got %d", last)
	}
}

func TestRequestValidation(t *testing.T) {
	e := setup(t, nil)
	tok := e.login(t)
	checks := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/v1/users", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/users?name=a&limit=abc", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/users?name=%00", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/users/not-a-uuid", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/users/00000000-0000-0000-0000-000000000000", http.StatusNotFound},
		{http.MethodPost, "/api/v1/users/" + e.userID + "/enrich?provider=nope", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/nope", http.StatusNotFound},
		{http.MethodDelete, "/api/v1/users", http.StatusMethodNotAllowed},
	}
	for _, c := range checks {
		resp, body := e.do(t, c.method, c.path, tok, nil)
		if resp.StatusCode != c.want {
			t.Errorf("%s %s: got %d want %d (%v)", c.method, c.path, resp.StatusCode, c.want, body)
		}
	}

	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/api/v1/auth/login", strings.NewReader(`{"username":"a"}`))
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("missing content-type: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, e.srv.URL+"/api/v1/auth/login", strings.NewReader(`{"username":`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed json: %d", resp.StatusCode)
	}
}

func TestHealthAndSecurityHeaders(t *testing.T) {
	e := setup(t, nil)
	resp, _ := e.do(t, http.MethodGet, "/healthz", "", nil)
	if resp.StatusCode != 200 || resp.Header.Get("X-Content-Type-Options") != "nosniff" ||
		resp.Header.Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Fatalf("healthz: %d %v", resp.StatusCode, resp.Header)
	}
	resp, _ = e.do(t, http.MethodGet, "/readyz", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("readyz: %d", resp.StatusCode)
	}
	down := setup(t, downPinger{})
	resp, _ = down.do(t, http.MethodGet, "/readyz", "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz down: %d", resp.StatusCode)
	}
}
