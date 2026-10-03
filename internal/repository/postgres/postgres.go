// Package postgres implements repository.UserRepository on PostgreSQL. Because
// CockroachDB speaks the PostgreSQL wire protocol and supports this SQL subset,
// the same implementation can be pointed at CockroachDB.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/navid/rolodex/internal/domain"
	"github.com/navid/rolodex/migrations"
)

const uniqueViolation = "23505"

// Repository is a PostgreSQL-backed UserRepository.
type Repository struct {
	db *sql.DB
}

// Open connects to PostgreSQL, configures the connection pool and applies
// migrations.
func Open(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	if err := migrations.Up(ctx, db, "postgres"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Repository{db: db}, nil
}

// DB exposes the underlying pool (used by tests for cleanup).
func (r *Repository) DB() *sql.DB { return r.db }

// CreateUser inserts a user, profile and credential in one transaction.
func (r *Repository) CreateUser(ctx context.Context, p domain.Profile, c domain.Credential) (domain.User, error) {
	now := time.Now().UTC()
	u := domain.User{ID: uuid.New(), CreatedAt: now, UpdatedAt: now}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, created_at, updated_at) VALUES ($1, $2, $3)`,
		u.ID, u.CreatedAt, u.UpdatedAt); err != nil {
		return domain.User{}, mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, name, phone, street_address, locality, region, postal_code, country)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, p.Name, p.Phone, p.Address.StreetAddress, p.Address.Locality,
		p.Address.Region, p.Address.PostalCode, p.Address.Country); err != nil {
		return domain.User{}, mapErr(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_credentials (id, user_id, method, username, secret_hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), u.ID, c.Method, c.Username, nullString(c.SecretHash), now); err != nil {
		return domain.User{}, mapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, fmt.Errorf("postgres: commit: %w", err)
	}
	return u, nil
}

// GetProfile returns a single profile by user ID.
func (r *Repository) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, name, phone, street_address, locality, region, postal_code, country
		 FROM user_profiles WHERE user_id = $1`, userID)
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
	if q.Username != "" {
		where = append(where, "EXISTS (SELECT 1 FROM user_credentials c WHERE c.user_id = p.user_id AND lower(c.username) LIKE "+
			arg("%"+escapeLike(strings.ToLower(q.Username))+"%")+` ESCAPE '\')`)
	}
	query := `SELECT p.user_id, p.name, p.phone, p.street_address, p.locality, p.region, p.postal_code, p.country
	          FROM user_profiles p`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY p.name, p.user_id LIMIT " + arg(q.Limit) + " OFFSET " + arg(q.Offset)

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
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.UserID, c.Method, c.Username, nullString(c.SecretHash), c.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
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
		 FROM user_credentials WHERE method = $1 AND lower(username) = lower($2)`, method, username)
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
		 FROM user_credentials WHERE user_id = $1 ORDER BY created_at`, userID)
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
		`UPDATE user_credentials SET last_used_at = $1 WHERE id = $2`, time.Now().UTC(), credentialID)
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

// Close closes the connection pool.
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
		secret   sql.NullString
		lastUsed sql.NullTime
	)
	if err := s.Scan(&c.ID, &c.UserID, &c.Method, &c.Username, &secret, &c.CreatedAt, &lastUsed); err != nil {
		return nil, err
	}
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.ErrConflict
	}
	return fmt.Errorf("postgres: %w", err)
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
