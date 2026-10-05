package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/security"
)

const (
	maxIDTokenLen = 8 << 10
	maxEmailLen   = 254
)

// IDTokenVerifier verifies a Firebase ID token and returns its identity.
type IDTokenVerifier interface {
	Verify(ctx context.Context, token string) (security.FirebaseIdentity, error)
}

// AnyDomain, listed as an allowed domain, admits every verified email.
const AnyDomain = "*"

// EmailAllowlist decides which verified email addresses may sign in with
// Firebase: exact addresses, any address at a listed domain, or every
// address when AnyDomain is listed.
type EmailAllowlist struct {
	emails  map[string]struct{}
	domains map[string]struct{}
	any     bool
}

// NewEmailAllowlist builds an allowlist. Entries are matched case-insensitively;
// domains may be written with or without a leading "@".
func NewEmailAllowlist(emails, domains []string) EmailAllowlist {
	a := EmailAllowlist{emails: map[string]struct{}{}, domains: map[string]struct{}{}}
	for _, e := range emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			a.emails[e] = struct{}{}
		}
	}
	for _, d := range domains {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
		switch d {
		case "":
		case AnyDomain:
			a.any = true
		default:
			a.domains[d] = struct{}{}
		}
	}
	return a
}

// AllowsAny reports whether every verified email is admitted.
func (a EmailAllowlist) AllowsAny() bool { return a.any }

// Allows reports whether email (already lower-cased) is on the allowlist.
func (a EmailAllowlist) Allows(email string) bool {
	if a.any {
		return true
	}
	if _, ok := a.emails[email]; ok {
		return true
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return false
	}
	_, ok := a.domains[email[at+1:]]
	return ok
}

// FirebaseAuthService exchanges a Firebase ID token (Google sign-in) for a
// Rolodex access token. The Google account's verified email must be on the
// allowlist; the first sign-in creates a Rolodex user linked to that email
// through an "oauth" credential.
type FirebaseAuthService struct {
	repo     repository.UserRepository
	verifier IDTokenVerifier
	allow    EmailAllowlist
	tokens   TokenIssuer
	log      *slog.Logger
}

// NewFirebaseAuthService wires a FirebaseAuthService.
func NewFirebaseAuthService(repo repository.UserRepository, verifier IDTokenVerifier, allow EmailAllowlist, tokens TokenIssuer, log *slog.Logger) *FirebaseAuthService {
	return &FirebaseAuthService{repo: repo, verifier: verifier, allow: allow, tokens: tokens, log: log}
}

// Login verifies idToken and returns an access token for the linked user.
// Invalid tokens yield domain.ErrUnauthorized; valid Google identities that
// are not allowed yield domain.ErrForbidden.
func (s *FirebaseAuthService) Login(ctx context.Context, idToken string) (AccessToken, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" || len(idToken) > maxIDTokenLen {
		return AccessToken{}, domain.ErrUnauthorized
	}
	id, err := s.verifier.Verify(ctx, idToken)
	if err != nil {
		if errors.Is(err, security.ErrInvalidToken) {
			return AccessToken{}, domain.ErrUnauthorized
		}
		return AccessToken{}, fmt.Errorf("firebase auth: verify: %w", err)
	}

	email := strings.ToLower(strings.TrimSpace(id.Email))
	if !id.EmailVerified || email == "" || len(email) > maxEmailLen || !isPlainText(email) || !s.allow.Allows(email) {
		s.log.InfoContext(ctx, "firebase sign-in refused", "firebase_uid", id.UID, "email_verified", id.EmailVerified)
		return AccessToken{}, domain.ErrForbidden
	}

	cred, err := s.credentialFor(ctx, email, id)
	if err != nil {
		return AccessToken{}, err
	}
	if err := s.repo.TouchCredential(ctx, cred.ID); err != nil {
		s.log.WarnContext(ctx, "failed to record credential use", "credential_id", cred.ID, "error", err)
	}
	tok, exp, err := s.tokens.Issue(cred.UserID.String(), cred.Username)
	if err != nil {
		return AccessToken{}, fmt.Errorf("firebase auth: issue: %w", err)
	}
	return AccessToken{AccessToken: tok, TokenType: "Bearer", ExpiresAt: exp}, nil
}

// credentialFor returns the oauth credential for email, creating the user on
// first sign-in. A concurrent first sign-in loses the insert race with
// ErrConflict and then reads the winner's credential.
func (s *FirebaseAuthService) credentialFor(ctx context.Context, email string, id security.FirebaseIdentity) (*domain.Credential, error) {
	cred, err := s.repo.GetCredential(ctx, domain.MethodOAuth, email)
	if err == nil {
		return cred, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("firebase auth: lookup: %w", err)
	}

	u, err := s.repo.CreateUser(ctx, domain.Profile{Name: displayName(id.Name, email)},
		domain.Credential{Method: domain.MethodOAuth, Username: email})
	switch {
	case err == nil:
		s.log.InfoContext(ctx, "user provisioned from firebase sign-in", "user_id", u.ID, "firebase_uid", id.UID)
	case !errors.Is(err, domain.ErrConflict):
		return nil, fmt.Errorf("firebase auth: provision: %w", err)
	}
	cred, err = s.repo.GetCredential(ctx, domain.MethodOAuth, email)
	if err != nil {
		return nil, fmt.Errorf("firebase auth: lookup after provisioning: %w", err)
	}
	return cred, nil
}

// displayName uses the Google profile name when it is usable plain text,
// falling back to the email address.
func displayName(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" || !isPlainText(name) {
		return email
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		name = string([]rune(name)[:maxNameLen])
	}
	return name
}
