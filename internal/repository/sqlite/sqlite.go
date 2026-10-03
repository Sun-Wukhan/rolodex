// Package sqlite implements repository.UserRepository on SQLite using the
// pure-Go modernc.org/sqlite driver (no cgo). Intended for local development,
// tests and single-node deployments.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/migrations"
)

// Repository is a SQLite-backed UserRepository.
type Repository struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, enables foreign keys and
// WAL, and applies migrations.
func Open(ctx context.Context, path string) (*Repository, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY under load.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}
	if err := migrations.Up(ctx, db, "sqlite"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Repository{db: db}, nil
}

// CreateUser inserts a user, profile and credential in one transaction.
func (r *Repository) CreateUser(ctx context.Context, p domain.Profile, c domain.Credential) (domain.User, error) {
	now := time.Now().UTC()
	u := domain.User{ID: uuid.New(), CreatedAt: now, UpdatedAt: now}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("sqlite: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, created_at, updated_at) VALUES (?, ?, ?)`,
		u.ID.String(), u.CreatedAt, u.UpdatedAt); err != nil {
		return domain.User{}, mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, name, phone, street_address, locality, region, postal_code, country)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID.String(), p.Name, p.Phone, p.Address.StreetAddress, p.Address.Locality,
		p.Address.Region, p.Address.PostalCode, p.Address.Country); err != nil {
		return domain.User{}, mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_credentials (id, user_id, method, username, secret_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), u.ID.String(), string(c.Method), c.Username, nullString(c.SecretHash), now); err != nil {
		return domain.User{}, mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, fmt.Errorf("sqlite: commit: %w", err)
	}
	return u, nil
}

// GetProfile returns a single profile by user ID.
func (r *Repository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, name, phone, street_address, locality, region, postal_code, country
		 FROM user_profiles WHERE user_id = ?`, userID.String())
	p, err := scanProfile(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return p, nil
}

// SearchProfiles builds a parameterised query from the non-empty filters.
func (r *Repository) SearchProfiles(ctx context.Context, q domain.SearchQuery) ([]domain.Profile, error) {
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
	if q.Username != "" {
		where = append(where, `EXISTS (SELECT 1 FROM user_credentials c WHERE c.user_id = p.user_id AND lower(c.username) LIKE ? ESCAPE '\')`)
		args = append(args, "%"+escapeLike(strings.ToLower(q.Username))+"%")
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

	rows, err := r.db.QueryContext(ctx, query, args...)
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

// AddCredential inserts a credential for an existing user.
func (r *Repository) AddCredential(ctx context.Context, c domain.Credential) (domain.Credential, error) {
	c.ID = uuid.New()
	c.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_credentials (id, user_id, method, username, secret_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID.String(), c.UserID.String(), string(c.Method), c.Username, nullString(c.SecretHash), c.CreatedAt)
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return domain.Credential{}, domain.ErrNotFound
		}
		return domain.Credential{}, mapErr(err)
	}
	return c, nil
}

// GetCredential looks up a credential by method and case-insensitive username.
func (r *Repository) GetCredential(ctx context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, method, username, secret_hash, created_at, last_used_at
		 FROM user_credentials WHERE method = ? AND lower(username) = lower(?)`, string(method), username)
	c, err := scanCredential(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return c, nil
}

// ListCredentials returns every credential owned by a user.
func (r *Repository) ListCredentials(ctx context.Context, userID uuid.UUID) ([]domain.Credential, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, method, username, secret_hash, created_at, last_used_at
		 FROM user_credentials WHERE user_id = ? ORDER BY created_at`, userID.String())
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	out := []domain.Credential{}
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// TouchCredential sets last_used_at to now.
func (r *Repository) TouchCredential(ctx context.Context, credentialID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE user_credentials SET last_used_at = ? WHERE id = ?`, time.Now().UTC(), credentialID.String())
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Ping checks connectivity.
func (r *Repository) Ping(ctx context.Context) error { return r.db.PingContext(ctx) }

// Close closes the database.
func (r *Repository) Close() error { return r.db.Close() }

type scanner interface{ Scan(dest ...any) error }

func scanProfile(s scanner) (*domain.Profile, error) {
	var p domain.Profile
	if err := s.Scan(&p.UserID, &p.Name, &p.Phone, &p.Address.StreetAddress, &p.Address.Locality,
		&p.Address.Region, &p.Address.PostalCode, &p.Address.Country); err != nil {
		return nil, err
	}
	return &p, nil
}

func scanCredential(s scanner) (*domain.Credential, error) {
	var (
		c        domain.Credential
		method   string
		secret   sql.NullString
		lastUsed sql.NullTime
	)
	if err := s.Scan(&c.ID, &c.UserID, &method, &c.Username, &secret, &c.CreatedAt, &lastUsed); err != nil {
		return nil, err
	}
	c.Method = domain.CredentialMethod(method)
	c.SecretHash = secret.String
	if lastUsed.Valid {
		t := lastUsed.Time
		c.LastUsedAt = &t
	}
	return &c, nil
}

func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return domain.ErrConflict
		}
	}
	return fmt.Errorf("sqlite: %w", err)
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
