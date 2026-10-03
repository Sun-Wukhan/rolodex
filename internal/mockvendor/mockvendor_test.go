package mockvendor

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(cfg Config) *httptest.Server {
	s := New(cfg, ABCRecords(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.rand = func() float64 { return 0.1 }
	return httptest.NewServer(s.Handler())
}

func post(t *testing.T, url, token, body string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestIdentityRequiresValidToken(t *testing.T) {
	srv := newServer(Config{Format: FormatABC, Username: "u", Password: "p"})
	defer srv.Close()
	if got := post(t, srv.URL+"/identity", "", `{"name":"Ada Lovelace"}`); got != http.StatusUnauthorized {
		t.Fatalf("no token: %d", got)
	}
	if got := post(t, srv.URL+"/identity", "forged", `{"name":"Ada Lovelace"}`); got != http.StatusUnauthorized {
		t.Fatalf("forged token: %d", got)
	}
	if got := post(t, srv.URL+"/auth", "", `{"username":"u","password":"bad"}`); got != http.StatusUnauthorized {
		t.Fatalf("bad creds: %d", got)
	}
	if got := post(t, srv.URL+"/auth", "", `not json`); got != http.StatusBadRequest {
		t.Fatalf("bad body: %d", got)
	}
}

func TestFailureInjection(t *testing.T) {
	srv := newServer(Config{Format: FormatABC, Username: "u", Password: "p", FailureRate: 0.5})
	defer srv.Close()
	tok := issue(t, srv.URL)
	if got := post(t, srv.URL+"/identity", tok, `{"name":"Ada Lovelace"}`); got != http.StatusServiceUnavailable {
		t.Fatalf("expected injected 503, got %d", got)
	}
}

func issue(t *testing.T, base string) string {
	t.Helper()
	resp, err := http.Post(base+"/auth", "application/json", strings.NewReader(`{"username":"u","password":"p"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.AccessToken == "" {
		t.Fatalf("auth response: %v", err)
	}
	return out.AccessToken
}

func TestRendersBothFormats(t *testing.T) {
	for _, f := range []Format{FormatABC, FormatXYC} {
		srv := newServer(Config{Format: f, Username: "u", Password: "p"})
		tok := issue(t, srv.URL)
		if got := post(t, srv.URL+"/identity", tok, `{"phone":"416 555 0101"}`); got != http.StatusOK {
			t.Errorf("%s: phone match status %d", f, got)
		}
		if got := post(t, srv.URL+"/identity", tok, `{"name":"Nobody"}`); got != http.StatusNotFound {
			t.Errorf("%s: no match status %d", f, got)
		}
		srv.Close()
	}
}
