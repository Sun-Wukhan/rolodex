package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/internal/service"
)

// ErrorBody is the standard error envelope returned by every endpoint.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes a single API error.
type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string, fields map[string]string) {
	writeJSON(w, status, ErrorBody{Error: ErrorDetail{
		Code: code, Message: msg, Fields: fields, RequestID: middleware.GetReqID(r.Context()),
	}})
}

// handleError maps domain errors to HTTP responses. Unexpected errors are
// logged with the request ID and reported generically so internals never leak.
func handleError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "request validation failed", ve.Fields)
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "request validation failed", nil)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "resource not found", nil)
	case errors.Is(err, domain.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", "resource already exists", nil)
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
	default:
		log.ErrorContext(r.Context(), "unhandled error", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "internal", "internal server error", nil)
	}
}
