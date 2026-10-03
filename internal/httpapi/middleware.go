package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/navid/rolodex/internal/security"
)

type ctxKey int

const claimsKey ctxKey = iota

// TokenVerifier validates bearer tokens.
type TokenVerifier interface {
	Verify(token string) (*security.Claims, error)
}

// requestLogger logs one structured line per request. It logs the matched
// route pattern rather than the raw URL so search terms (names, phone
// numbers) in query strings never reach the logs.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			level := slog.LevelInfo
			if ww.Status() >= 500 {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "http request",
				"method", r.Method,
				"route", route,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// securityHeaders sets conservative defaults for a JSON API.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// requireAuth rejects requests without a valid bearer token and stores the
// verified claims in the request context.
func requireAuth(v TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="rolodex"`)
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing bearer token", nil)
				return
			}
			claims, err := v.Verify(strings.TrimSpace(raw))
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer realm="rolodex", error="invalid_token"`)
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or expired token", nil)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
		})
	}
}

// claimsFrom returns the authenticated caller's claims, if any.
func claimsFrom(ctx context.Context) *security.Claims {
	c, _ := ctx.Value(claimsKey).(*security.Claims)
	return c
}
