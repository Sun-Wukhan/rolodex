package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/migrations"
)

// ProfileStore is a PostgreSQL-backed repository.ProfileStore.
type ProfileStore struct {
	db *sql.DB
}

var _ repository.ProfileStore = (*ProfileStore)(nil)

// OpenProfiles connects to the profiles database and applies its migrations.
func OpenProfiles(ctx context.Context, dsn string) (*ProfileStore, error) {
	db, err := openDB(ctx, dsn, migrations.SchemaProfiles)
	if err != nil {
		return nil, err
	}
	return &ProfileStore{db: db}, nil
}

// DB exposes the underlying pool (used by tests for cleanup).
func (s *ProfileStore) DB() *sql.DB { return s.db }

// CreateUser inserts a user and profile in one transaction.
func (s *ProfileStore) CreateUser(ctx context.Context, u domain.User, p domain.Profile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, created_at, updated_at) VALUES ($1, $2, $3)`,
		u.ID, u.CreatedAt, u.UpdatedAt); err != nil {
		return mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, name, phone, street_address, locality, region, postal_code, country)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, p.Name, p.Phone, p.Address.StreetAddress, p.Address.Locality,
		p.Address.Region, p.Address.PostalCode, p.Address.Country); err != nil {
		return mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

// DeleteUser removes a user; the profile goes with it via ON DELETE CASCADE.
func (s *ProfileStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return mapErr(err)
	}
	return nil
}

// GetProfile returns a single profile by user ID.
func (s *ProfileStore) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT user_id, name, phone, street_address, locality, region, postal_code, country
		 FROM user_profiles WHERE user_id = $1`, userID)
	p, err := scanProfile(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return p, nil
}

// SearchProfiles builds a parameterised query from the non-empty filters.
func (s *ProfileStore) SearchProfiles(ctx context.Context, q domain.SearchQuery, userIDs []uuid.UUID) ([]domain.Profile, error) {
	q = q.Normalize()
	var (
		where []string
		args  []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if q.Name != "" {
		where = append(where, "lower(p.name) LIKE "+arg("%"+escapeLike(strings.ToLower(q.Name))+"%")+` ESCAPE '\'`)
	}
	if q.Phone != "" {
		where = append(where, "p.phone = "+arg(q.Phone))
	}
	if userIDs != nil {
		ids := make([]string, len(userIDs))
		for i, id := range userIDs {
			ids[i] = id.String()
		}
		where = append(where, "p.user_id = ANY("+arg(ids)+"::uuid[])")
	}
	query := `SELECT p.user_id, p.name, p.phone, p.street_address, p.locality, p.region, p.postal_code, p.country
	          FROM user_profiles p`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// Only constant SQL fragments and $n placeholders are concatenated; all
	// user-supplied values are bound parameters.
	query += " ORDER BY p.name, p.user_id LIMIT " + arg(q.Limit) + " OFFSET " + arg(q.Offset) //nolint:gosec // see above

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []domain.Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// Ping checks connectivity.
func (s *ProfileStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close closes the connection pool.
func (s *ProfileStore) Close() error { return s.db.Close() }

func scanProfile(sc scanner) (*domain.Profile, error) {
	var p domain.Profile
	if err := sc.Scan(&p.UserID, &p.Name, &p.Phone, &p.Address.StreetAddress, &p.Address.Locality,
		&p.Address.Region, &p.Address.PostalCode, &p.Address.Country); err != nil {
		return nil, err
	}
	return &p, nil
}
