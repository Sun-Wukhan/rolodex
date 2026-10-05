package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testProject = "rolodex-test"

type keyServer struct {
	srv   *httptest.Server
	key   *rsa.PrivateKey
	hits  atomic.Int32
	fails atomic.Bool
}

func newKeyServer(t *testing.T) *keyServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ks := &keyServer{key: key}
	ks.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ks.hits.Add(1)
		if ks.fails.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(ks.srv.Close)
	return ks
}

func validClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "https://securetoken.google.com/" + testProject, "aud": testProject,
		"sub": "firebase-uid-1", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "auth_time": now.Unix(),
		"email": "ada@example.com", "email_verified": true, "name": "Ada Lovelace",
	}
}

func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newVerifier(t *testing.T, ks *keyServer) *FirebaseVerifier {
	t.Helper()
	v, err := NewFirebaseVerifier(testProject, ks.srv.URL, ks.srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFirebaseVerifyValidTokenAndCachesKeys(t *testing.T) {
	ks := newKeyServer(t)
	v := newVerifier(t, ks)
	tok := sign(t, ks.key, "k1", validClaims(time.Now()))

	for i := 0; i < 3; i++ {
		id, err := v.Verify(context.Background(), tok)
		if err != nil {
			t.Fatal(err)
		}
		want := FirebaseIdentity{UID: "firebase-uid-1", Email: "ada@example.com", EmailVerified: true, Name: "Ada Lovelace"}
		if id != want {
			t.Fatalf("identity = %+v, want %+v", id, want)
		}
	}
	if n := ks.hits.Load(); n != 1 {
		t.Fatalf("keys fetched %d times, want 1 (cached)", n)
	}
}

func TestFirebaseVerifyRejectsBadTokens(t *testing.T) {
	ks := newKeyServer(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Now()
	with := func(key string, value any) jwt.MapClaims {
		c := validClaims(now)
		if value == nil {
			delete(c, key)
		} else {
			c[key] = value
		}
		return c
	}
	hs256, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(now)).SignedString([]byte("secret"))

	cases := map[string]string{
		"other project audience": sign(t, ks.key, "k1", with("aud", "someone-else")),
		"other issuer":           sign(t, ks.key, "k1", with("iss", "https://securetoken.google.com/someone-else")),
		"expired":                sign(t, ks.key, "k1", with("exp", now.Add(-time.Hour).Unix())),
		"issued in the future":   sign(t, ks.key, "k1", with("iat", now.Add(time.Hour).Unix())),
		"auth_time in future":    sign(t, ks.key, "k1", with("auth_time", now.Add(time.Hour).Unix())),
		"missing auth_time":      sign(t, ks.key, "k1", with("auth_time", nil)),
		"missing subject":        sign(t, ks.key, "k1", with("sub", nil)),
		"missing kid":            sign(t, ks.key, "", validClaims(now)),
		"unknown kid":            sign(t, ks.key, "k2", validClaims(now)),
		"signed by another key":  sign(t, other, "k1", validClaims(now)),
		"hs256 algorithm":        hs256,
		"garbage":                "not-a-jwt",
	}
	v := newVerifier(t, ks)
	for name, tok := range cases {
		if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: got %v, want ErrInvalidToken", name, err)
		}
	}
	if n := ks.hits.Load(); n > 2 {
		t.Fatalf("forged tokens triggered %d key fetches; refreshes must be rate limited", n)
	}
}

func TestFirebaseVerifyKeysUnavailable(t *testing.T) {
	ks := newKeyServer(t)
	ks.fails.Store(true)
	v := newVerifier(t, ks)
	tok := sign(t, ks.key, "k1", validClaims(time.Now()))
	for i := 0; i < 3; i++ {
		if _, err := v.Verify(context.Background(), tok); !errors.Is(err, ErrKeysUnavailable) {
			t.Fatalf("got %v, want ErrKeysUnavailable", err)
		}
	}
	if n := ks.hits.Load(); n != 1 {
		t.Fatalf("outage triggered %d key fetches, want 1 within the backoff window", n)
	}

	ks.fails.Store(false)
	v.now = func() time.Time { return time.Now().Add(2 * minKeysRefreshBackoff) }
	if _, err := v.Verify(context.Background(), sign(t, ks.key, "k1", validClaims(v.now()))); err != nil {
		t.Fatalf("after recovery: %v", err)
	}
}

func TestFirebaseVerifierConfig(t *testing.T) {
	if _, err := NewFirebaseVerifier("", FirebaseKeysURL, nil); err == nil {
		t.Fatal("empty project ID accepted")
	}
	for header, want := range map[string]time.Duration{
		"public, max-age=19800, must-revalidate": 19800 * time.Second,
		"no-cache":                               defaultKeysMaxAge,
		"max-age=abc":                            defaultKeysMaxAge,
	} {
		if got := maxAge(header); got != want {
			t.Errorf("maxAge(%q) = %v, want %v", header, got, want)
		}
	}
}
