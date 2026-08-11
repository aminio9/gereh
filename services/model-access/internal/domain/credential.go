package domain

import "time"

// CredentialState describes the lifecycle of a BYOK credential reference.
type CredentialState string

const (
	// CredentialStatePendingStore indicates metadata exists but the secret
	// has not yet been persisted to the secret store.
	CredentialStatePendingStore CredentialState = "pending_store"

	// CredentialStatePendingVerification indicates the secret is stored and
	// awaiting provider verification.
	CredentialStatePendingVerification CredentialState = "pending_verification"

	// CredentialStateActive indicates the credential is verified and active.
	CredentialStateActive CredentialState = "active"

	// CredentialStateVerificationFailed indicates the provider rejected it.
	CredentialStateVerificationFailed CredentialState = "verification_failed"

	// CredentialStateDestroyed indicates the secret was purged.
	CredentialStateDestroyed CredentialState = "destroyed"
)

// BYOKCredential is the PostgreSQL-resident metadata for a BYOK secret.
//
// It stores a secret reference and keyed fingerprints only. It never
// contains the raw provider credential.
type BYOKCredential struct {
	TenantID     string
	ConnectionID string

	SecretRef string

	Fingerprint []byte

	FingerprintDisplay string
	FingerprintKeyID   string

	State CredentialState

	ActiveVaultVersion  int64
	VaultLatestVersion  int64
	PendingVaultVersion int64

	PendingFingerprint        []byte
	PendingFingerprintDisplay string

	CredentialVersion int64

	VerifiedAt  *time.Time
	DestroyedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// VerificationOutcome categorizes a provider verification result.
type VerificationOutcome string

// Verification outcomes.
const (
	// VerificationSucceeded indicates the provider accepted the credential.
	VerificationSucceeded VerificationOutcome = "succeeded"

	// VerificationRejected indicates the provider rejected the credential.
	VerificationRejected VerificationOutcome = "rejected"

	// VerificationTransientFailure indicates a temporary provider failure.
	VerificationTransientFailure VerificationOutcome = "transient_failure"
)

// CredentialVerification is an immutable audit event for a credential
// verification attempt. It contains no secret material.
type CredentialVerification struct {
	EventID string

	TenantID     string
	ConnectionID string
	ActorUserID  string

	Operation string

	Outcome VerificationOutcome

	ReasonCode string

	ProviderHTTPStatus *int

	FingerprintDisplay string

	RequestID     string
	CorrelationID string

	OccurredAt time.Time
}

// CredentialOperationState describes rotation idempotency progress.
type CredentialOperationState string

// Credential operation states.
const (
	// CredentialOperationPrepared indicates the rotation was prepared.
	CredentialOperationPrepared CredentialOperationState = "prepared"

	// CredentialOperationSecretStored indicates the new secret is stored.
	CredentialOperationSecretStored CredentialOperationState = "secret_stored"

	// CredentialOperationSucceeded indicates rotation completed.
	CredentialOperationSucceeded CredentialOperationState = "succeeded"

	// CredentialOperationRejected indicates rotation was rejected.
	CredentialOperationRejected CredentialOperationState = "rejected"
)

// CredentialOperation tracks a credential rotation across PostgreSQL, the
// secret store and the provider.
type CredentialOperation struct {
	TenantID    string
	ActorUserID string

	Operation string

	IdempotencyKey string
	RequestHash    []byte

	ConnectionID string

	State CredentialOperationState

	ResponseConnectionVersion *int64

	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
}
