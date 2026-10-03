package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/navid/rolodex/internal/domain"
)

// memRepo is a minimal in-memory repository.UserRepository for unit tests.
type memRepo struct {
	mu       sync.Mutex
	profiles map[uuid.UUID]domain.Profile
	creds    []domain.Credential
	failWith error
	touched  int
}

func newMemRepo() *memRepo { return &memRepo{profiles: map[uuid.UUID]domain.Profile{}} }

func (m *memRepo) CreateUser(_ context.Context, p domain.Profile, c domain.Credential) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return domain.User{}, m.failWith
	}
	for _, e := range m.creds {
		if e.Method == c.Method && e.Username == c.Username {
			return domain.User{}, domain.ErrConflict
		}
	}
	u := domain.User{ID: uuid.New(), CreatedAt: time.Now()}
	p.UserID = u.ID
	m.profiles[u.ID] = p
	c.ID, c.UserID = uuid.New(), u.ID
	m.creds = append(m.creds, c)
	return u, nil
}

func (m *memRepo) GetProfile(_ context.Context, id uuid.UUID) (*domain.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return nil, m.failWith
	}
	p, ok := m.profiles[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &p, nil
}

func (m *memRepo) SearchProfiles(_ context.Context, q domain.SearchQuery) ([]domain.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Profile{}
	for _, p := range m.profiles {
		if q.Name != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(q.Name)) {
			continue
		}
		if q.Phone != "" && p.Phone != q.Phone {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (m *memRepo) AddCredential(_ context.Context, c domain.Credential) (domain.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.ID = uuid.New()
	m.creds = append(m.creds, c)
	return c, nil
}

func (m *memRepo) GetCredential(_ context.Context, method domain.CredentialMethod, username string) (*domain.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failWith != nil {
		return nil, m.failWith
	}
	for _, c := range m.creds {
		if c.Method == method && strings.EqualFold(c.Username, username) {
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *memRepo) ListCredentials(_ context.Context, id uuid.UUID) ([]domain.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Credential{}
	for _, c := range m.creds {
		if c.UserID == id {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memRepo) TouchCredential(context.Context, uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.touched++
	return nil
}

func (m *memRepo) Ping(context.Context) error { return nil }
func (m *memRepo) Close() error               { return nil }

// plainHasher is a deliberately trivial hasher so tests run fast.
type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Verify(p, enc string) error {
	if enc != "h:"+p {
		return errors.New("mismatch")
	}
	return nil
}

type fakeIssuer struct{}

func (fakeIssuer) Issue(sub, _ string) (string, time.Time, error) {
	return "token-for-" + sub, time.Now().Add(time.Minute), nil
}

// fakeProvider returns a canned identity or error, optionally after a delay.
type fakeProvider struct {
	name  string
	id    *domain.Identity
	err   error
	delay time.Duration
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Lookup(ctx context.Context, _ domain.IdentityQuery) (*domain.Identity, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.id, f.err
}
