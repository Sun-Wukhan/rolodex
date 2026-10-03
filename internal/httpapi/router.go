package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

// Deps are the collaborators the HTTP layer needs.
type Deps struct {
	Auth               Authenticator
	Profiles           ProfileManager
	Identity           Enricher
	Ready              Pinger
	Tokens             TokenVerifier
	Log                *slog.Logger
	CORSAllowedOrigins []string
	LoginRatePerMinute int
}

// NewRouter builds the HTTP handler with all routes and middleware.
func NewRouter(d Deps) http.Handler {
	h := &handlers{auth: d.Auth, profiles: d.Profiles, identity: d.Identity, ready: d.Ready, log: d.Log}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(d.Log))
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: d.CORSAllowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		ExposedHeaders: []string{"X-Request-Id"},
		MaxAge:         300,
	}))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", h.healthz)
	r.Get("/readyz", h.readyz)

	r.Route("/api/v1", func(r chi.Router) {
		r.With(httprate.LimitByIP(d.LoginRatePerMinute, time.Minute)).Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth(d.Tokens))
			r.Get("/me", h.me)
			r.Get("/providers", h.listProviders)
			r.Get("/users", h.searchUsers)
			r.Post("/users", h.createUser)
			r.Get("/users/{id}", h.getUser)
			r.Post("/users/{id}/enrich", h.enrichUser)
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "route not found", nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})
	return r
}
