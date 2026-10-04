// Package httpapi is the HTTP transport layer: routing, middleware, request
// decoding and error mapping. It contains no business logic.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/service"
)

const maxBodyBytes = 1 << 20

// Authenticator logs users in.
type Authenticator interface {
	Login(ctx context.Context, username, password string) (service.AccessToken, error)
}

// ProfileManager creates, reads and searches profiles.
type ProfileManager interface {
	CreateUser(ctx context.Context, in service.CreateUserInput) (domain.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (*service.UserDetails, error)
	Search(ctx context.Context, q domain.SearchQuery) ([]domain.Profile, error)
}

// Enricher enriches profiles from identity providers.
type Enricher interface {
	Enrich(ctx context.Context, userID uuid.UUID, providers []string) (*domain.EnrichedProfile, error)
	ProviderNames() []string
}

// Pinger reports datastore readiness.
type Pinger interface {
	Ping(ctx context.Context) error
}

type handlers struct {
	auth     Authenticator
	profiles ProfileManager
	identity Enricher
	ready    Pinger
	log      *slog.Logger
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SearchResponse is the paginated search result envelope.
type SearchResponse struct {
	Data   []domain.Profile `json:"data"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decode(w, r, &req) {
		return
	}
	tok, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		handleError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": c.Subject, "username": c.Username, "expires_at": c.ExpiresAt.Time,
	})
}

func (h *handlers) createUser(w http.ResponseWriter, r *http.Request) {
	var in service.CreateUserInput
	if !decode(w, r, &in) {
		return
	}
	u, err := h.profiles.CreateUser(r.Context(), in)
	if err != nil {
		handleError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+u.ID.String())
	writeJSON(w, http.StatusCreated, u)
}

func (h *handlers) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	d, err := h.profiles.GetUser(r.Context(), id)
	if err != nil {
		handleError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *handlers) searchUsers(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	limit, err1 := intParam(qs.Get("limit"))
	offset, err2 := intParam(qs.Get("offset"))
	if err1 != nil || err2 != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "limit and offset must be integers", nil)
		return
	}
	q := domain.SearchQuery{
		Name: qs.Get("name"), Phone: qs.Get("phone"), Username: qs.Get("username"),
		Limit: limit, Offset: offset,
	}
	res, err := h.profiles.Search(r.Context(), q)
	if err != nil {
		handleError(w, r, h.log, err)
		return
	}
	n := q.Normalize()
	writeJSON(w, http.StatusOK, SearchResponse{Data: res, Limit: n.Limit, Offset: n.Offset})
}

func (h *handlers) enrichUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var providers []string
	if p := r.URL.Query().Get("provider"); p != "" {
		providers = strings.Split(p, ",")
	}
	res, err := h.identity.Enrich(r.Context(), id, providers)
	if err != nil {
		handleError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *handlers) listProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"providers": h.identity.ProviderNames()})
}

func (h *handlers) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.ready.Ping(ctx); err != nil {
		h.log.WarnContext(ctx, "readiness check failed", "error", err)
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "datastore unavailable", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// decode reads a size-limited JSON body, rejecting unknown fields and
// trailing data. It writes the error response itself and returns false on
// failure.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json", nil)
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large", nil)
			return false
		}
		writeError(w, r, http.StatusBadRequest, "invalid_json", "malformed JSON body", nil)
		return false
	}
	if dec.More() {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "unexpected data after JSON body", nil)
		return false
	}
	return true
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "id must be a UUID", nil)
		return uuid.Nil, false
	}
	return id, true
}

func intParam(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
