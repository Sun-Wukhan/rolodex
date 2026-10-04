package split

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/repository"
)

// fakeProfiles implements only what these tests exercise; calling anything
// else panics on the nil embedded interface.
type fakeProfiles struct {
	repository.ProfileStore
	createErr  error
	pingErr    error
	searchCall int
}

func (f *fakeProfiles) CreateUser(context.Context, domain.User, domain.Profile) error {
	return f.createErr
}

func (f *fakeProfiles) SearchProfiles(context.Context, domain.SearchQuery, []uuid.UUID) ([]domain.Profile, error) {
	f.searchCall++
	return []domain.Profile{}, nil
}

func (f *fakeProfiles) Ping(context.Context) error { return f.pingErr }

type fakeCredentials struct {
	repository.CredentialStore
	added     []domain.Credential
	deleted   []uuid.UUID
	deleteErr error
	ids       []uuid.UUID
}

func (f *fakeCredentials) AddCredential(_ context.Context, c domain.Credential) (domain.Credential, error) {
	f.added = append(f.added, c)
	return c, nil
}

func (f *fakeCredentials) DeleteCredentials(ctx context.Context, userID uuid.UUID) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.deleted = append(f.deleted, userID)
	return f.deleteErr
}

func (f *fakeCredentials) UserIDsByUsername(context.Context, string) ([]uuid.UUID, error) {
	return f.ids, nil
}

func (f *fakeCredentials) Ping(context.Context) error { return nil }

func TestCreateUserLinksCredentialToNewUser(t *testing.T) {
	creds := &fakeCredentials{}
	r := New(&fakeProfiles{}, creds)

	u, err := r.CreateUser(context.Background(), domain.Profile{Name: "Ada"}, domain.Credential{Username: "ada"})
	if err != nil {
		t.Fatal(err)
	}
	if len(creds.added) != 1 || creds.added[0].UserID != u.ID {
		t.Fatalf("credential not linked to user %s: %+v", u.ID, creds.added)
	}
}

func TestCreateUserRemovesCredentialWhenProfileFails(t *testing.T) {
	boom := errors.New("profiles down")
	creds := &fakeCredentials{}
	r := New(&fakeProfiles{createErr: boom}, creds)

	// A cancelled request must not stop the cleanup.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.CreateUser(ctx, domain.Profile{}, domain.Credential{Username: "ada"})
	if !errors.Is(err, boom) {
		t.Fatalf("want profile error, got %v", err)
	}
	if len(creds.deleted) != 1 || creds.deleted[0] != creds.added[0].UserID {
		t.Fatalf("orphaned credential not removed: added %+v deleted %v", creds.added, creds.deleted)
	}
}

func TestCreateUserReportsFailedCompensation(t *testing.T) {
	boom := errors.New("profiles down")
	r := New(&fakeProfiles{createErr: boom}, &fakeCredentials{deleteErr: errors.New("credentials down")})

	_, err := r.CreateUser(context.Background(), domain.Profile{}, domain.Credential{Username: "ada"})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "orphaned credential") {
		t.Fatalf("want both errors, got %v", err)
	}
}

func TestSearchByUnknownUsernameSkipsProfiles(t *testing.T) {
	profiles := &fakeProfiles{}
	r := New(profiles, &fakeCredentials{})

	got, err := r.SearchProfiles(context.Background(), domain.SearchQuery{Username: "nobody"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
	if profiles.searchCall != 0 {
		t.Fatal("profiles database queried although no username matched")
	}
}

func TestPingNamesFailingDatabase(t *testing.T) {
	r := New(&fakeProfiles{pingErr: errors.New("refused")}, &fakeCredentials{})
	if err := r.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "profiles database") {
		t.Fatalf("got %v", err)
	}
	if err := New(&fakeProfiles{}, &fakeCredentials{}).Ping(context.Background()); err != nil {
		t.Fatalf("healthy ping failed: %v", err)
	}
}
