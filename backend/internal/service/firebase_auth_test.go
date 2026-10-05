package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/security"
)

type fakeVerifier struct {
	id  security.FirebaseIdentity
	err error
}

func (f fakeVerifier) Verify(context.Context, string) (security.FirebaseIdentity, error) {
	return f.id, f.err
}

func googleUser(email string) security.FirebaseIdentity {
	return security.FirebaseIdentity{UID: "uid-" + email, Email: email, EmailVerified: true, Name: "Grace Hopper"}
}

func TestEmailAllowlist(t *testing.T) {
	a := NewEmailAllowlist([]string{" Ada@Example.com "}, []string{"@navy.mil", "Corp.example"})
	for email, want := range map[string]bool{
		"ada@example.com":        true,
		"grace@navy.mil":         true,
		"x@corp.example":         true,
		"bob@example.com":        false,
		"grace@sub.navy.mil":     false,
		"evil@navy.mil.attacker": false,
		"navy.mil":               false,
		"@navy.mil":              false,
	} {
		if got := a.Allows(email); got != want {
			t.Errorf("Allows(%q) = %v, want %v", email, got, want)
		}
	}
	if NewEmailAllowlist(nil, nil).Allows("ada@example.com") {
		t.Fatal("empty allowlist must allow nobody")
	}
	if a.AllowsAny() {
		t.Fatal("explicit entries must not admit everyone")
	}
}

func TestEmailAllowlistAnyDomain(t *testing.T) {
	a := NewEmailAllowlist(nil, []string{" * "})
	if !a.AllowsAny() {
		t.Fatal("AllowsAny() = false with the wildcard listed")
	}
	for _, email := range []string{"ada@example.com", "grace@navy.mil", "x@sub.corp.example"} {
		if !a.Allows(email) {
			t.Errorf("Allows(%q) = false with the wildcard listed", email)
		}
	}
}

func TestFirebaseLoginAnyDomainStillRequiresVerifiedEmail(t *testing.T) {
	unverified := googleUser("ada@example.com")
	unverified.EmailVerified = false
	repo := newMemRepo()
	svc := NewFirebaseAuthService(repo, fakeVerifier{id: unverified}, NewEmailAllowlist(nil, []string{AnyDomain}), fakeIssuer{}, discard)
	if _, err := svc.Login(context.Background(), "t"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
	if len(repo.profiles) != 0 {
		t.Fatal("refused sign-in provisioned a user")
	}
}

func TestFirebaseLoginProvisionsThenReusesUser(t *testing.T) {
	repo := newMemRepo()
	allow := NewEmailAllowlist(nil, []string{"navy.mil"})
	svc := NewFirebaseAuthService(repo, fakeVerifier{id: googleUser("Grace@Navy.mil")}, allow, fakeIssuer{}, discard)

	first, err := svc.Login(context.Background(), "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.profiles) != 1 || len(repo.creds) != 1 {
		t.Fatalf("want one provisioned user, got %d profiles %d creds", len(repo.profiles), len(repo.creds))
	}
	c := repo.creds[0]
	if c.Method != domain.MethodOAuth || c.Username != "grace@navy.mil" || c.SecretHash != "" {
		t.Fatalf("credential: %+v", c)
	}
	if p := repo.profiles[c.UserID]; p.Name != "Grace Hopper" || p.Phone != "" {
		t.Fatalf("profile: %+v", p)
	}
	if first.AccessToken != "token-for-"+c.UserID.String() || first.TokenType != "Bearer" {
		t.Fatalf("token: %+v", first)
	}

	second, err := svc.Login(context.Background(), "id-token")
	if err != nil || second.AccessToken != first.AccessToken {
		t.Fatalf("second login: %+v %v", second, err)
	}
	if len(repo.profiles) != 1 || repo.touched != 2 {
		t.Fatalf("second login must reuse the user: %d profiles, %d touches", len(repo.profiles), repo.touched)
	}
}

func TestFirebaseLoginRefusals(t *testing.T) {
	allow := NewEmailAllowlist([]string{"ada@example.com"}, nil)
	unverified := googleUser("ada@example.com")
	unverified.EmailVerified = false

	cases := map[string]struct {
		verifier fakeVerifier
		token    string
		want     error
	}{
		"empty token":       {fakeVerifier{id: googleUser("ada@example.com")}, " ", domain.ErrUnauthorized},
		"invalid token":     {fakeVerifier{err: security.ErrInvalidToken}, "t", domain.ErrUnauthorized},
		"not on allowlist":  {fakeVerifier{id: googleUser("bob@example.com")}, "t", domain.ErrForbidden},
		"unverified email":  {fakeVerifier{id: unverified}, "t", domain.ErrForbidden},
		"no email":          {fakeVerifier{id: googleUser("")}, "t", domain.ErrForbidden},
		"control character": {fakeVerifier{id: googleUser("ada@example.com\x00")}, "t", domain.ErrForbidden},
	}
	for name, tc := range cases {
		repo := newMemRepo()
		svc := NewFirebaseAuthService(repo, tc.verifier, allow, fakeIssuer{}, discard)
		if _, err := svc.Login(context.Background(), tc.token); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
		if len(repo.profiles) != 0 {
			t.Errorf("%s: refused sign-in provisioned a user", name)
		}
	}

	keysDown := NewFirebaseAuthService(newMemRepo(), fakeVerifier{err: security.ErrKeysUnavailable}, allow, fakeIssuer{}, discard)
	_, err := keysDown.Login(context.Background(), "t")
	if err == nil || errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("key outage must be an internal error, got %v", err)
	}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"  Ada Lovelace ": "Ada Lovelace",
		"":                "ada@example.com",
		"Ada\x00":         "ada@example.com",
	}
	for in, want := range cases {
		if got := displayName(in, "ada@example.com"); got != want {
			t.Errorf("displayName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := displayName(strings.Repeat("é", maxNameLen+10), "x@y"); utf8.RuneCountInString(got) != maxNameLen {
		t.Errorf("long name not truncated: %d runes", utf8.RuneCountInString(got))
	}
}
