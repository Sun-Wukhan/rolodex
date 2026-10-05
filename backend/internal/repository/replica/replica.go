// Package replica splits a ProfileStore's workload between a primary and a
// read-only streaming replica (command/query separation at the database
// level): writes always go to the primary, reads go to the replica.
//
// A replica lags the primary slightly, so reads fall back to the primary when
// the replica cannot answer: a profile that is not on the replica yet (for
// example, read straight after it was created) or a replica that is down.
// Search results may briefly miss a user created a moment ago; that is the
// accepted cost of offloading reads.
package replica

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
)

// ProfileStore routes profile reads to a replica and writes to the primary.
type ProfileStore struct {
	primary repository.ProfileStore
	replica repository.ProfileStore
	log     *slog.Logger
}

var _ repository.ProfileStore = (*ProfileStore)(nil)

// New combines a primary and a replica. The ProfileStore takes ownership of
// both and closes them in Close.
func New(primary, replica repository.ProfileStore, log *slog.Logger) *ProfileStore {
	return &ProfileStore{primary: primary, replica: replica, log: log}
}

// CreateUser writes to the primary.
func (s *ProfileStore) CreateUser(ctx context.Context, u domain.User, p domain.Profile) error {
	return s.primary.CreateUser(ctx, u, p)
}

// DeleteUser writes to the primary.
func (s *ProfileStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return s.primary.DeleteUser(ctx, userID)
}

// GetProfile reads from the replica. A miss is retried on the primary so a
// user can always read a profile they have just created.
func (s *ProfileStore) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	p, err := s.replica.GetProfile(ctx, userID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		s.fellBack(ctx, "get_profile", err)
	}
	return s.primary.GetProfile(ctx, userID)
}

// SearchProfiles reads from the replica, falling back to the primary only if
// the replica fails.
func (s *ProfileStore) SearchProfiles(ctx context.Context, q domain.SearchQuery, userIDs []uuid.UUID) ([]domain.Profile, error) {
	out, err := s.replica.SearchProfiles(ctx, q, userIDs)
	if err == nil {
		return out, nil
	}
	s.fellBack(ctx, "search_profiles", err)
	return s.primary.SearchProfiles(ctx, q, userIDs)
}

// Ping checks only the primary: reads survive a replica outage by falling
// back, so a lost replica must not take the service out of rotation.
func (s *ProfileStore) Ping(ctx context.Context) error {
	if err := s.replica.Ping(ctx); err != nil {
		s.log.WarnContext(ctx, "profiles replica unreachable; reads are served by the primary", "error", err)
	}
	return s.primary.Ping(ctx)
}

// Close closes both connections.
func (s *ProfileStore) Close() error {
	return errors.Join(s.primary.Close(), s.replica.Close())
}

func (s *ProfileStore) fellBack(ctx context.Context, op string, err error) {
	if ctx.Err() != nil {
		return
	}
	s.log.WarnContext(ctx, "profiles replica read failed; using primary", "op", op, "error", err)
}
