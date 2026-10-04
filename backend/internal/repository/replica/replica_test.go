package replica

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
)

// fakeStore records which operations reached it and returns canned errors.
type fakeStore struct {
	name      string
	calls     []string
	getErr    error
	searchErr error
	pingErr   error
}

func (f *fakeStore) CreateUser(context.Context, domain.User, domain.Profile) error {
	f.calls = append(f.calls, "create")
	return nil
}

func (f *fakeStore) DeleteUser(context.Context, uuid.UUID) error {
	f.calls = append(f.calls, "delete")
	return nil
}

func (f *fakeStore) GetProfile(_ context.Context, id uuid.UUID) (*domain.Profile, error) {
	f.calls = append(f.calls, "get")
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &domain.Profile{UserID: id, Name: f.name}, nil
}

func (f *fakeStore) SearchProfiles(context.Context, domain.SearchQuery, []uuid.UUID) ([]domain.Profile, error) {
	f.calls = append(f.calls, "search")
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return []domain.Profile{{Name: f.name}}, nil
}

func (f *fakeStore) Ping(context.Context) error {
	f.calls = append(f.calls, "ping")
	return f.pingErr
}

func (f *fakeStore) Close() error { return nil }

func newStore(primary, rep *fakeStore) (*ProfileStore, *bytes.Buffer) {
	var logs bytes.Buffer
	return New(primary, rep, slog.New(slog.NewTextHandler(&logs, nil))), &logs
}

func TestWritesGoToPrimaryReadsToReplica(t *testing.T) {
	primary, rep := &fakeStore{name: "primary"}, &fakeStore{name: "replica"}
	s, _ := newStore(primary, rep)
	ctx := context.Background()

	_ = s.CreateUser(ctx, domain.User{}, domain.Profile{})
	_ = s.DeleteUser(ctx, uuid.New())
	p, _ := s.GetProfile(ctx, uuid.New())
	found, _ := s.SearchProfiles(ctx, domain.SearchQuery{Name: "a"}, nil)

	if p.Name != "replica" || found[0].Name != "replica" {
		t.Fatalf("reads not served by replica: %+v %+v", p, found)
	}
	if strings.Join(primary.calls, ",") != "create,delete" || strings.Join(rep.calls, ",") != "get,search" {
		t.Fatalf("primary %v, replica %v", primary.calls, rep.calls)
	}
}

func TestGetFallsBackWhenReplicaHasNotCaughtUp(t *testing.T) {
	primary, rep := &fakeStore{name: "primary"}, &fakeStore{getErr: domain.ErrNotFound}
	s, logs := newStore(primary, rep)

	p, err := s.GetProfile(context.Background(), uuid.New())
	if err != nil || p.Name != "primary" {
		t.Fatalf("got %+v %v", p, err)
	}
	if logs.Len() != 0 {
		t.Fatalf("replica lag is expected and should not be logged: %s", logs)
	}

	primary.getErr = domain.ErrNotFound
	if _, err := s.GetProfile(context.Background(), uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound from primary, got %v", err)
	}
}

func TestReadsFallBackWhenReplicaIsDown(t *testing.T) {
	down := errors.New("connection refused")
	primary, rep := &fakeStore{name: "primary"}, &fakeStore{getErr: down, searchErr: down}
	s, logs := newStore(primary, rep)

	p, err := s.GetProfile(context.Background(), uuid.New())
	if err != nil || p.Name != "primary" {
		t.Fatalf("get: %+v %v", p, err)
	}
	found, err := s.SearchProfiles(context.Background(), domain.SearchQuery{Name: "a"}, nil)
	if err != nil || found[0].Name != "primary" {
		t.Fatalf("search: %+v %v", found, err)
	}
	if !strings.Contains(logs.String(), "using primary") {
		t.Fatalf("fallback not logged: %s", logs)
	}
}

func TestPingIgnoresReplicaOutage(t *testing.T) {
	primary, rep := &fakeStore{}, &fakeStore{pingErr: errors.New("down")}
	s, logs := newStore(primary, rep)

	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("replica outage failed readiness: %v", err)
	}
	if !strings.Contains(logs.String(), "replica unreachable") {
		t.Fatalf("outage not logged: %s", logs)
	}
	primary.pingErr = errors.New("primary down")
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("primary outage must fail readiness")
	}
}
