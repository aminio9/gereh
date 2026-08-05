package domain

import (
	"encoding/json"
	"time"
)

// ExternalIdentity is a verified identity returned by an OpenID Provider.
type ExternalIdentity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	PictureURL    string
	RawClaims     json.RawMessage
}

// User is an internal Gereh user resolved from an external identity.
type User struct {
	ID            string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	PictureURL    string
}

// LoginTransaction is a one-time OIDC authorization transaction.
type LoginTransaction struct {
	State              string    `json:"state"`
	BrowserBindingHash string    `json:"browser_binding_hash"`
	Nonce              string    `json:"nonce"`
	PKCEVerifier       string    `json:"pkce_verifier"`
	ReturnTo           string    `json:"return_to"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

// Session is an opaque server-side browser session.
type Session struct {
	ID        string    `json:"id"`
	CSRFHash  string    `json:"csrf_hash"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
