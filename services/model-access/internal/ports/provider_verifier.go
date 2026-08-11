package ports

import "context"

// ProviderVerification is the sanitized result of a provider credential check.
type ProviderVerification struct {
	HTTPStatus int

	// Evidence only; phase 19 owns persistent model catalog data.
	DiscoveredModelCount int
}

// CredentialVerifier verifies a credential against fixed Gereh-owned
// provider endpoints. It never follows redirects.
type CredentialVerifier interface {
	Verify(
		ctx context.Context,
		providerKey string,
		credential []byte,
	) (ProviderVerification, error)
}
