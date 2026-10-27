package apperr

import "net/http"

// Error is a safe, client-facing error.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func Validation(fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "Invalid input", Fields: fields}
}

var (
	ErrUnauthorized = New(http.StatusUnauthorized, "unauthorized", "Authentication required")
	ErrForbidden    = New(http.StatusForbidden, "forbidden", "You do not own this resource")
	ErrNotFound     = New(http.StatusNotFound, "not_found", "Resource not found")
	ErrEmailTaken   = New(http.StatusConflict, "email_taken", "Email already registered")
	ErrBadCreds     = New(http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
	ErrAlreadyVoted = New(http.StatusConflict, "already_voted", "You have already voted on this poll")
	ErrPollClosed   = New(http.StatusConflict, "poll_closed", "This poll is closed")
	ErrBadOption    = New(http.StatusUnprocessableEntity, "invalid_option", "Option does not belong to this poll")
	ErrBadID        = New(http.StatusBadRequest, "invalid_id", "Invalid identifier")
	ErrInternal     = New(http.StatusInternalServerError, "internal_error", "Something went wrong")
	ErrRateLimited  = New(http.StatusTooManyRequests, "rate_limited", "Too many requests, slow down")
)
