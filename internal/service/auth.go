// Package service contains the business logic. Services depend only on
// interfaces (repository, provider, security) so they can be unit tested with
// fakes and are unaware of HTTP or the underlying database.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/internal/repository"
	"github.com/navid/rolodex/internal/security"
)

// TokenIssuer issues signed access tokens.
type TokenIssuer interface {
	Issue(subject, username string) (string, time.Time, error)
}

// AccessToken is the result of a successful login.
type AccessToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// AuthService authenticates users with username/password credentials.
type AuthService struct {
	repo   repository.UserRepository
	hasher security.PasswordHasher
	tokens TokenIssuer
	log    *slog.Logger
	// dummyHash is verified when the username does not exist so that response
	// timing does not reveal which usernames are registered.
	dummyHash string
}

// NewAuthService wires an AuthService.
func NewAuthService(repo repository.UserRepository, hasher security.PasswordHasher, tokens TokenIssuer, log *slog.Logger) (*AuthService, error) {
	dummy, err := hasher.Hash("timing-equaliser")
	if err != nil {
		return nil, fmt.Errorf("auth: init: %w", err)
	}
	return &AuthService{repo: repo, hasher: hasher, tokens: tokens, log: log, dummyHash: dummy}, nil
}

// Login verifies a username/password pair and returns an access token. Every
// failure mode returns domain.ErrUnauthorized so callers cannot enumerate users.
func (s *AuthService) Login(ctx context.Context, username, password string) (AccessToken, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || password == "" || len(password) > maxPasswordLen {
		return AccessToken{}, domain.ErrUnauthorized
	}

	cred, err := s.repo.GetCredential(ctx, domain.MethodPassword, username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			_ = s.hasher.Verify(password, s.dummyHash)
			return AccessToken{}, domain.ErrUnauthorized
		}
		return AccessToken{}, fmt.Errorf("auth: lookup: %w", err)
	}
	if err := s.hasher.Verify(password, cred.SecretHash); err != nil {
		return AccessToken{}, domain.ErrUnauthorized
	}

	if err := s.repo.TouchCredential(ctx, cred.ID); err != nil {
		s.log.WarnContext(ctx, "failed to record credential use", "credential_id", cred.ID, "error", err)
	}

	tok, exp, err := s.tokens.Issue(cred.UserID.String(), cred.Username)
	if err != nil {
		return AccessToken{}, fmt.Errorf("auth: issue: %w", err)
	}
	return AccessToken{AccessToken: tok, TokenType: "Bearer", ExpiresAt: exp}, nil
}
