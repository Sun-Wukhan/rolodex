// Package mockvendor implements fake ABC and XYC identity vendors for local
// development and demos. Each vendor exposes POST /auth and POST /identity as
// described in the exercise, with configurable latency and failure injection.
package mockvendor

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
)

// Format selects the vendor's wire format.
type Format string

// Supported vendor formats.
const (
	FormatABC Format = "abc"
	FormatXYC Format = "xyc"
)

// Config configures a mock vendor.
type Config struct {
	Format      Format
	Username    string
	Password    string
	TokenTTL    time.Duration
	FailureRate float64
	Latency     time.Duration
}

// Server is a mock identity vendor.
type Server struct {
	cfg     Config
	records []Record
	log     *slog.Logger
	rand    func() float64

	mu     sync.Mutex
	tokens map[string]time.Time
}

// New creates a mock vendor serving the given records.
func New(cfg Config, records []Record, log *slog.Logger) *Server {
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 5 * time.Minute
	}
	return &Server{cfg: cfg, records: records, log: log, rand: mrand.Float64, tokens: map[string]time.Time{}}
}

// Handler returns the vendor's HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", s.auth)
	mux.HandleFunc("POST /identity", s.identity)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}

func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if req.Username != s.cfg.Username || req.Password != s.cfg.Password {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	tok := hex.EncodeToString(buf)

	s.mu.Lock()
	s.tokens[tok] = time.Now().Add(s.cfg.TokenTTL)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok, "token_type": "Bearer", "expires_in": int(s.cfg.TokenTTL.Seconds()),
	})
}

func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Latency > 0 {
		time.Sleep(s.cfg.Latency)
	}
	if !s.validToken(r.Header.Get("Authorization")) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
		return
	}
	if s.cfg.FailureRate > 0 && s.rand() < s.cfg.FailureRate {
		s.log.Info("injected failure", "vendor", s.cfg.Format)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}

	var q domain.IdentityQuery
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&q); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	rec, ok := s.find(q)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no_match"})
		return
	}
	writeJSON(w, http.StatusOK, s.render(rec))
}

func (s *Server) validToken(header string) bool {
	tok, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, found := s.tokens[tok]
	return found && time.Now().Before(exp)
}

// find matches on normalised phone first, then on case-insensitive name.
func (s *Server) find(q domain.IdentityQuery) (Record, bool) {
	if phone, err := domain.NormalizePhone(q.Phone); err == nil {
		for _, rec := range s.records {
			if p, _ := domain.NormalizePhone(rec.Phone); p == phone {
				return rec, true
			}
		}
	}
	name := strings.ToLower(strings.TrimSpace(q.Name))
	for _, rec := range s.records {
		if name != "" && strings.ToLower(rec.Name) == name {
			return rec, true
		}
	}
	return Record{}, false
}

func (s *Server) render(rec Record) any {
	if s.cfg.Format == FormatXYC {
		return map[string]any{"person": map[string]any{
			"full_name": rec.Name,
			"msisdn":    rec.Phone,
			"addr": map[string]string{
				"line1": rec.Street, "city": rec.City, "state": rec.Region,
				"zip": rec.Postal, "country_code": strings.ToLower(rec.Country),
			},
		}}
	}
	return map[string]any{
		"name":  rec.Name,
		"phone": rec.Phone,
		"address": map[string]string{
			"street_address": rec.Street, "locality": rec.City, "region": rec.Region,
			"postal_code": rec.Postal, "country": rec.Country,
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
