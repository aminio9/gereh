// Package auth provides authenticated-principal context helpers.
package auth

// Principal is the authenticated internal user and validated tenant context.
type Principal struct {
	UserID        string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	PictureURL    string

	// TenantID and TenantRole are empty until a Tenant Service call validates
	// the requested tenant membership.
	TenantID   string
	TenantRole string
}
