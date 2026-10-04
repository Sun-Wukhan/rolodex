// Package split implements repository.UserRepository on top of two separate
// databases: a ProfileStore for user and profile data and a CredentialStore for
// login credentials. No transaction spans both, so writes are ordered and
// compensated to keep them consistent.
package split

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
)

// compensateTimeout bounds cleanup after a failed cross-database write. Cleanup
// runs even if the request context has been cancelled.
const compensateTimeout = 5 * time.Second

// Repository is a UserRepository spanning a profiles and a credentials database.
type Repository struct {
	profiles    repository.ProfileStore
	credentials repository.CredentialStore
}

var _ repository.UserRepository = (*Repository)(nil)

// New combines a profile store and a credential store. The Repository takes
// ownership of both and closes them in Close.
func New(profiles repository.ProfileStore, credentials repository.CredentialStore) *Repository {
	return &Repository{profiles: profiles, credentials: credentials}
}

// CreateUser stores the credential first, because its unique username is the
// constraint most likely to fail, then the profile. If the profile write fails
// the credential is deleted again so no login exists without a user.
func (r *Repository) CreateUser(ctx context.Context, p domain.Profile, c domain.Credential) (domain.User, error) {
	now := time.Now().UTC()
	u := domain.User{ID: uuid.New(), CreatedAt: now, UpdatedAt: now}
	c.UserID = u.ID

	if _, err := r.credentials.AddCredential(ctx, c); err != nil {
		return domain.User{}, err
	}
	if err := r.profiles.CreateUser(ctx, u, p); err != nil {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), compensateTimeout)
		defer cancel()
		if cerr := r.credentials.DeleteCredentials(cctx, u.ID); cerr != nil {
			return domain.User{}, errors.Join(err, fmt.Errorf("split: remove orphaned credential for user %s: %w", u.ID, cerr))
		}
		return domain.User{}, err
	}
	return u, nil
}

// GetProfile reads from the profiles database.
func (r *Repository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	return r.profiles.GetProfile(ctx, userID)
}

// SearchProfiles resolves a username filter against the credentials database
// first and then restricts the profile search to the matching users.
func (r *Repository) SearchProfiles(ctx context.Context, q domain.SearchQuery) ([]domain.Profile, error) {
	q = q.Normalize()
	var ids []uuid.UUID
	if q.Username != "" {
		var err error
		ids, err = r.credentials.UserIDsByUsername(ctx, q.Username)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []domain.Profile{}, nil
		}
	}
	return r.profiles.SearchProfiles(ctx, q, ids)
}

// AddCredential checks that the user exists, since no foreign key can enforce
// it across databases, then stores the credential.
func (r *Repository) AddCredential(ctx context.Context, c domain.Credential) (domain.Credential, error) {
	if _, err := r.profiles.GetProfile(ctx, c.UserID); err != nil {
		return domain.Credential{}, err
	}
	return r.credentials.AddCredential(ctx, c)
}

// GetCredential reads from the credentials database.
func (r *Repository) GetCredential(ctx context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error) {
	return r.credentials.GetCredential(ctx, method, username)
}

// ListCredentials reads from the credentials database.
func (r *Repository) ListCredentials(ctx context.Context, userID uuid.UUID) ([]domain.Credential, error) {
	return r.credentials.ListCredentials(ctx, userID)
}

// TouchCredential writes to the credentials database.
func (r *Repository) TouchCredential(ctx context.Context, credentialID uuid.UUID) error {
	return r.credentials.TouchCredential(ctx, credentialID)
}

// Ping reports ready only when both databases are reachable.
func (r *Repository) Ping(ctx context.Context) error {
	return errors.Join(labelled("profiles", r.profiles.Ping(ctx)), labelled("credentials", r.credentials.Ping(ctx)))
}

// Close closes both stores.
func (r *Repository) Close() error {
	return errors.Join(labelled("profiles", r.profiles.Close()), labelled("credentials", r.credentials.Close()))
}

// labelled prefixes err with the database it came from, preserving nil.
func labelled(db string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s database: %w", db, err)
}
