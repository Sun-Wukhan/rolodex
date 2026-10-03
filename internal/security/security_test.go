package security

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func fastHasher() *Argon2Hasher {
	return NewArgon2Hasher(Argon2Params{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
}

func TestArgon2HashVerify(t *testing.T) {
	h := fastHasher()
	enc, err := h.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoding %q", enc)
	}
	if err := h.Verify("correct horse", enc); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := h.Verify("wrong", enc); !errors.Is(err, ErrMismatch) {
		t.Fatalf("want ErrMismatch, got %v", err)
	}
	other, _ := h.Hash("correct horse")
	if other == enc {
		t.Fatal("salts must differ")
	}
	if err := h.Verify("x", "$bcrypt$nope"); err == nil {
		t.Fatal("expected format error")
	}
	if err := NewArgon2Hasher(DefaultArgon2Params()).Verify("correct horse", enc); err != nil {
		t.Fatalf("params must be read from hash: %v", err)
	}
}

func TestTokenIssueVerify(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	m, err := NewTokenManager(secret, "rolodex", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tok, exp, err := m.Issue("user-1", "ada")
	if err != nil || exp.IsZero() {
		t.Fatal(err)
	}
	c, err := m.Verify(tok)
	if err != nil || c.Subject != "user-1" || c.Username != "ada" {
		t.Fatalf("verify: %+v %v", c, err)
	}

	m.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if _, err := m.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("expired token accepted")
	}
}

func TestTokenRejectsTamperingAndAlgNone(t *testing.T) {
	m, _ := NewTokenManager([]byte(strings.Repeat("s", 32)), "rolodex", time.Minute)
	other, _ := NewTokenManager([]byte(strings.Repeat("x", 32)), "rolodex", time.Minute)
	tok, _, _ := other.Issue("u", "n")
	if _, err := m.Verify(tok); err == nil {
		t.Fatal("token signed with other key accepted")
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": "u", "iss": "rolodex", "exp": time.Now().Add(time.Hour).Unix()}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := m.Verify(none); err == nil {
		t.Fatal("alg=none accepted")
	}
	if _, err := NewTokenManager([]byte("short"), "x", time.Minute); err == nil {
		t.Fatal("short secret accepted")
	}
}
