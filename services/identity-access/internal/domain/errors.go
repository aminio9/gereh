package domain

import "errors"

var (
	// ErrInvalidRequest indicates malformed or unsafe input.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrAuthenticationFailed indicates an invalid OIDC response or identity.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrSessionNotFound indicates a missing or expired session.
	ErrSessionNotFound = errors.New("session not found")

	// ErrCSRFValidation indicates a rejected CSRF token.
	ErrCSRFValidation = errors.New("csrf validation failed")
)
