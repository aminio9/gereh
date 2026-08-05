// Package auth provides authenticated-principal context helpers.
package auth

// Principal is the authenticated internal user identity.
//
// Tenant information is intentionally absent until tenant resolution is
// implemented in the next identity and tenancy stage.
type Principal struct {
	UserID        string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	PictureURL    string
}
