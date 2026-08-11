package domain

// SecretCleanup is a durable queued secret-store operation.
type SecretCleanup struct {
	ID int64

	TenantID string

	SecretRef string

	Version *int64

	Action string

	Attempts int
}
