package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/migrations"
)

// ProfileStore is a SQLite-backed repository.ProfileStore.
type ProfileStore struct {
	db *sql.DB
}

var _ repository.ProfileStore = (*ProfileStore)(nil)

// OpenProfiles opens the profiles database file and applies its migrations.
func OpenProfiles(ctx context.Context, path string) (*ProfileStore, error) {
	db, err := openDB(ctx, path, migrations.SchemaProfiles)
	if err != nil {
		return nil, err
	}
	return &ProfileStore{db: db}, nil
}

// CreateUser inserts a user and profile in one transaction.
func (s *ProfileStore) CreateUser(ctx context.Context, u domain.User, p domain.Profile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, created_at, updated_at) VALUES (?, ?, ?)`,
		u.ID.String(), u.CreatedAt, u.UpdatedAt); err != nil {
		return mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, name, phone, street_address, locality, region, postal_code, country)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID.String(), p.Name, p.Phone, p.Address.StreetAddress, p.Address.Locality,
		p.Address.Region, p.Address.PostalCode, p.Address.Country); err != nil {
		return mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

// DeleteUser removes a user; the profile goes with it via ON DELETE CASCADE.
func (s *ProfileStore) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID.String()); err != nil {
		return mapErr(err)
	}
	return nil
}

// GetProfile returns a single profile by user ID.
func (s *ProfileStore) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT user_id, name, phone, street_address, locality, region, postal_code, country
		 FROM user_profiles WHERE user_id = ?`, userID.String())
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
	if q.Name != "" {
		where = append(where, `lower(p.name) LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(strings.ToLower(q.Name))+"%")
	}
	if q.Phone != "" {
		where = append(where, "p.phone = ?")
		args = append(args, q.Phone)
	}
	if userIDs != nil {
		// A single JSON-array parameter avoids SQLite's bound-variable limit.
		ids := make([]string, len(userIDs))
		for i, id := range userIDs {
			ids[i] = id.String()
		}
		encoded, err := json.Marshal(ids)
		if err != nil {
			return nil, fmt.Errorf("sqlite: encode ids: %w", err)
		}
		where = append(where, "p.user_id IN (SELECT value FROM json_each(?))")
		args = append(args, string(encoded))
	}
	query := `SELECT p.user_id, p.name, p.phone, p.street_address, p.locality, p.region, p.postal_code, p.country
	          FROM user_profiles p`
	if len(where) > 0 {
		// Only constant SQL fragments with ? placeholders are concatenated; all
		// user-supplied values are bound parameters.
		query += " WHERE " + strings.Join(where, " AND ") //nolint:gosec // see above
	}
	query += " ORDER BY p.name, p.user_id LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)

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

// Close closes the database.
func (s *ProfileStore) Close() error { return s.db.Close() }

func scanProfile(sc scanner) (*domain.Profile, error) {
	var p domain.Profile
	if err := sc.Scan(&p.UserID, &p.Name, &p.Phone, &p.Address.StreetAddress, &p.Address.Locality,
		&p.Address.Region, &p.Address.PostalCode, &p.Address.Country); err != nil {
		return nil, err
	}
	return &p, nil
}
