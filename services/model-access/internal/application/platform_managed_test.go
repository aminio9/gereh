package application

import (
	"context"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type platformManagedAuthorizer struct {
	enabled bool
}

func (authorizer platformManagedAuthorizer) Require(
	_ context.Context,
	_ string,
	_ string,
	_ tenantv1.Permission,
) error {
	return nil
}

func (authorizer platformManagedAuthorizer) RequireWithContext(
	_ context.Context,
	_ string,
	_ string,
	_ tenantv1.Permission,
) (ports.TenantAccessContext, error) {
	return ports.TenantAccessContext{
		Region:  "eu-central",
		PlanKey: "test",
		Features: map[string]bool{
			"platform_managed_models": authorizer.enabled,
		},
		Limits: map[string]int64{},
	}, nil
}

// repositoryFake is a minimal repository stub for application-layer tests.
type repositoryFake struct {
	createCalls int
}

func (fake *repositoryFake) ListProviders(
	context.Context,
	string,
	string,
) ([]domain.Provider, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) CreateConnection(
	context.Context,
	ports.CreateConnectionParams,
) (domain.Connection, error) {
	fake.createCalls++
	panic("repository must not be called when entitlement is denied")
}

func (fake *repositoryFake) GetConnection(
	context.Context,
	string,
	string,
	string,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ListConnections(
	context.Context,
	string,
	string,
	int,
	*ports.ConnectionCursor,
	bool,
) ([]domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) UpdateConnection(
	context.Context,
	ports.UpdateConnectionParams,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ArchiveConnection(
	context.Context,
	ports.ArchiveConnectionParams,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ClaimOutbox(
	context.Context,
	int,
	time.Duration,
) ([]domain.OutboxRecord, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) MarkOutboxPublished(
	context.Context,
	int64,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ReleaseOutbox(
	context.Context,
	int64,
	time.Time,
	string,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) EnsureBYOKCredential(
	context.Context,
	ports.EnsureBYOKCredentialParams,
) (domain.BYOKCredential, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) GetBYOKCredential(
	context.Context,
	string,
	string,
	string,
) (domain.BYOKCredential, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) MarkBYOKSecretStored(
	context.Context,
	string,
	string,
	string,
	int64,
	time.Time,
) (domain.BYOKCredential, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ActivateBYOK(
	context.Context,
	ports.ActivateBYOKParams,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) FailInitialBYOKVerification(
	context.Context,
	ports.FailInitialBYOKParams,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) RecordTransientVerification(
	context.Context,
	domain.CredentialVerification,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) PrepareBYOKRotation(
	context.Context,
	ports.PrepareRotationParams,
) (ports.RotationPreparation, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) MarkBYOKRotationSecretStored(
	context.Context,
	string,
	string,
	string,
	string,
	int64,
	time.Time,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) CompleteBYOKRotation(
	context.Context,
	ports.CompleteRotationParams,
) (domain.Connection, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) RejectBYOKRotation(
	context.Context,
	string,
	string,
	string,
	string,
	int64,
	domain.CredentialVerification,
	time.Time,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ClaimSecretCleanup(
	context.Context,
	int,
	time.Duration,
) ([]domain.SecretCleanup, error) {
	panic("unexpected repository call")
}

func (fake *repositoryFake) CompleteSecretCleanup(
	context.Context,
	int64,
) error {
	panic("unexpected repository call")
}

func (fake *repositoryFake) ReleaseSecretCleanup(
	context.Context,
	int64,
	time.Time,
	string,
) error {
	panic("unexpected repository call")
}

func TestPlatformManagedRequiresEntitlement(t *testing.T) {
	t.Parallel()

	repository := &repositoryFake{}

	service, err := New(
		repository,
		platformManagedAuthorizer{enabled: false},
		newFakeSecretStore(),
		stubVerifier{},
		newTestFingerprinter(t),
		Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
	)
	require.NoError(t, err)

	_, err = service.CreateConnection(
		context.Background(),
		CreateConnectionInput{
			ActorUserID:    uuid.NewString(),
			TenantID:       uuid.NewString(),
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "Gereh OpenAI",
		},
	)

	require.ErrorIs(t, err, domain.ErrPlatformManagedEntitlementRequired)
	require.Zero(t, repository.createCalls)
}
