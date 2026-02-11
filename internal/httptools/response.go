package httptools

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/grantsy/grantsy/internal/infra/tracing"
)

const Version = "1.0"

const (
	ErrTypeValidationFailed = "https://grantsy.example/errors/validation-failed"
	ErrTypeBadRequest       = "https://grantsy.example/errors/bad-request"
	ErrTypeUnauthorized     = "https://grantsy.example/errors/unauthorized"
	ErrTypeInternalError    = "https://grantsy.example/errors/internal-error"
)

type Response struct {
	Data any   `json:"data,omitempty"`
	Meta *Meta `json:"meta"`
}

type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type ErrorResponse struct {
	Error ProblemDetails `json:"error"`
}

type ProblemDetails struct {
	Type      string       `json:"type" enum:"https://grantsy.example/errors/validation-failed,https://grantsy.example/errors/bad-request,https://grantsy.example/errors/unauthorized,https://grantsy.example/errors/internal-error"`
	Title     string       `json:"title"`
	Detail    string       `json:"detail"`
	Status    int          `json:"status"`
	RequestID string       `json:"request_id"`
	Fields    []FieldError `json:"fields,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	resp := Response{
		Data: data,
		Meta: &Meta{
			RequestID: tracing.GetRequestID(r.Context()),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   Version,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func Error(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	errType, title, detail string,
) {
	resp := ErrorResponse{
		Error: ProblemDetails{
			Type:      errType,
			Title:     title,
			Detail:    detail,
			Status:    status,
			RequestID: tracing.GetRequestID(r.Context()),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func ValidationError(
	w http.ResponseWriter,
	r *http.Request,
	fields []FieldError,
) {
	resp := ErrorResponse{
		Error: ProblemDetails{
			Type:      ErrTypeValidationFailed,
			Title:     "Validation Failed",
			Detail:    "One or more fields failed validation",
			Status:    http.StatusUnprocessableEntity,
			RequestID: tracing.GetRequestID(r.Context()),
			Fields:    fields,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(resp)
}

func InternalError(w http.ResponseWriter, r *http.Request) {
	Error(w, r, http.StatusInternalServerError,
		ErrTypeInternalError,
		"Internal Server Error",
		"An unexpected error occurred",
	)
}

func BadRequest(w http.ResponseWriter, r *http.Request, detail string) {
	Error(w, r, http.StatusBadRequest,
		ErrTypeBadRequest,
		"Bad Request",
		detail,
	)
}

func WriteStatus(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}
