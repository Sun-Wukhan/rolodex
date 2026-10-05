package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/migrations"
)

// CredentialStore is a PostgreSQL-backed repository.CredentialStore.
type CredentialStore struct {
	db *sql.DB
}

var _ repository.CredentialStore = (*CredentialStore)(nil)

// OpenCredentials connects to the credentials database and applies its
// migrations.
func OpenCredentials(ctx context.Context, dsn string) (*CredentialStore, error) {
	db, err := openDB(ctx, dsn, migrations.SchemaCredentials)
	if err != nil {
		return nil, err
	}
	return &CredentialStore{db: db}, nil
}

// DB exposes the underlying pool (used by tests for cleanup).
func (s *CredentialStore) DB() *sql.DB { return s.db }

// AddCredential inserts a credential.
func (s *CredentialStore) AddCredential(ctx context.Context, c domain.Credential) (domain.Credential, error) {
	c.ID = uuid.New()
	c.CreatedAt = time.Now().UTC()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO user_credentials (id, user_id, method, username, secret_hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.UserID, c.Method, c.Username, nullString(c.SecretHash), c.CreatedAt); err != nil {
		return domain.Credential{}, mapErr(err)
	}
	return c, nil
}

// DeleteCredentials removes all credentials of a user.
func (s *CredentialStore) DeleteCredentials(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_credentials WHERE user_id = $1`, userID); err != nil {
		return mapErr(err)
	}
	return nil
}

// GetCredential looks up a credential by method and case-insensitive username.
func (s *CredentialStore) GetCredential(ctx context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, method, username, secret_hash, created_at, last_used_at
		 FROM user_credentials WHERE method = $1 AND lower(username) = lower($2)`, method, username)
	c, err := scanCredential(row)
	if err != nil {
		return nil, mapErr(err)
	}
	return c, nil
}

// ListCredentials returns every credential owned by a user.
func (s *CredentialStore) ListCredentials(ctx context.Context, userID uuid.UUID) ([]domain.Credential, error) {
	rows, err := s.db.QueryContext(ctx,
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
func (s *CredentialStore) TouchCredential(ctx context.Context, credentialID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE user_credentials SET last_used_at = $1 WHERE id = $2`, time.Now().UTC(), credentialID)
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UserIDsByUsername returns users with a credential username containing substr.
func (s *CredentialStore) UserIDsByUsername(ctx context.Context, substr string) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT user_id FROM user_credentials WHERE lower(username) LIKE $1 ESCAPE '\'`,
		"%"+escapeLike(strings.ToLower(substr))+"%")
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapErr(err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Ping checks connectivity.
func (s *CredentialStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close closes the connection pool.
func (s *CredentialStore) Close() error { return s.db.Close() }

func scanCredential(sc scanner) (*domain.Credential, error) {
	var (
		c        domain.Credential
		secret   sql.NullString
		lastUsed sql.NullTime
	)
	if err := sc.Scan(&c.ID, &c.UserID, &c.Method, &c.Username, &secret, &c.CreatedAt, &lastUsed); err != nil {
		return nil, err
	}
	c.SecretHash = secret.String
	if lastUsed.Valid {
		t := lastUsed.Time
		c.LastUsedAt = &t
	}
	return &c, nil
}
