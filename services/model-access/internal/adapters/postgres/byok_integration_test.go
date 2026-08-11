package postgres

import (
	"context"
	"testing"

	"github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/protoutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func (test *modelAccessIntegrationTest) getCredential(
	ctx context.Context,
	t *testing.T,
	connectionID string,
) domain.BYOKCredential {
	t.Helper()

	credential, err := test.repository.GetBYOKCredential(
		ctx,
		test.userA,
		test.tenantA,
		connectionID,
	)
	require.NoError(t, err)

	return credential
}

func (test *modelAccessIntegrationTest) pendingSecretCleanupCount(
	ctx context.Context,
	t *testing.T,
	action string,
) int {
	t.Helper()

	var count int

	err := test.repository.(*Repository).pool.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM model_access_secret_cleanup
			WHERE tenant_id = $1::uuid
			  AND action = $2
			  AND completed_at IS NULL
		`,
		test.tenantA,
		action,
	).Scan(&count)
	require.NoError(t, err)

	return count
}

func (test *modelAccessIntegrationTest) outboxEnvelopes(
	ctx context.Context,
	t *testing.T,
	connectionID string,
) [][]byte {
	t.Helper()

	rows, err := test.repository.(*Repository).pool.Query(
		ctx,
		`
			SELECT envelope
			FROM model_access_outbox
			WHERE partition_key = $1
			ORDER BY outbox_id
		`,
		connectionID,
	)
	require.NoError(t, err)
	defer rows.Close()

	var result [][]byte

	for rows.Next() {
		var envelope []byte

		require.NoError(t, rows.Scan(&envelope))

		result = append(result, envelope)
	}

	return result
}

func TestCreateBYOKConnectionActivatesAfterVerification(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	key := "test-provider-key-do-not-log"

	result, err := test.service.CreateBYOKConnection(
		context.Background(),
		application.CreateBYOKConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			DisplayName:    "Customer OpenAI",
			APIKey:         key,
		},
	)
	require.NoError(t, err)

	require.Equal(t, domain.ConnectionTypeBYOK, result.ConnectionType)
	require.Equal(t, domain.ConnectionStatusActive, result.Status)
	require.NotEmpty(t, result.CredentialFingerprint)
	require.NotContains(t, result.CredentialFingerprint, key)
	require.EqualValues(t, 2, result.Version)

	ctx := context.Background()

	require.EqualValues(t, 2, test.outboxCountForAggregate(ctx, t, result.ID))
	require.EqualValues(t, 1, test.connectionCount(ctx, t, test.tenantA))
	require.EqualValues(t, 2, test.revisionCount(ctx, t, test.tenantA, result.ID))
}

func TestBYOKSecretNeverReachesPostgres(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	raw := "distinctive-secret-value-123456789"

	connection := test.createBYOK(t, raw)

	var databaseText string

	test.scopedLookup(
		context.Background(),
		t,
		test.tenantA,
		&databaseText,
		`
			SELECT
				coalesce(
					string_agg(
						value,
						' '
					),
					''
				)
			FROM (
				SELECT row_to_json(c)::text AS value
				FROM model_access_connections c

				UNION ALL

				SELECT row_to_json(c)::text
				FROM model_access_connection_credentials c

				UNION ALL

				SELECT row_to_json(v)::text
				FROM model_access_connection_verification_events v

				UNION ALL

				SELECT row_to_json(i)::text
				FROM model_access_idempotency i

				UNION ALL

				SELECT encode(
					envelope,
					'escape'
				)
				FROM model_access_outbox
			) AS all_values
		`,
	)

	require.NotContains(t, databaseText, raw)
	require.NotEmpty(t, connection.CredentialFingerprint)
}

func TestBYOKKafkaEnvelopeCarriesNoSecret(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	raw := "envelope-secret-canary-987654321"

	connection := test.createBYOK(t, raw)

	for _, envelope := range test.outboxEnvelopes(
		context.Background(),
		t,
		connection.ID,
	) {
		require.NotContains(t, string(envelope), raw)
		require.NotContains(t, string(envelope), "vault://")
	}
}

func TestBYOKRejectedCredentialFailsClosed(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	pool := test.repository.(*Repository).pool

	service := newModelAccessServiceWithVerifier(
		t,
		pool,
		test.secretStore,
		rejectingIntegrationVerifier{},
	)

	ctx := context.Background()

	_, err := service.CreateBYOKConnection(
		ctx,
		application.CreateBYOKConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			DisplayName:    "Rejected OpenAI",
			APIKey:         "rejected-key-do-not-log",
		},
	)
	require.ErrorIs(t, err, domain.ErrCredentialRejected)

	var connectionID string

	test.scopedLookup(
		ctx,
		t,
		test.tenantA,
		&connectionID,
		`
			SELECT connection_id::text
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
			  AND connection_type = 'byok'
			ORDER BY created_at DESC
			LIMIT 1
		`,
		test.tenantA,
	)

	connection, err := service.GetConnection(
		ctx,
		test.userA,
		test.tenantA,
		connectionID,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ConnectionStatusVerificationFailed, connection.Status)
	require.EqualValues(t, 2, connection.Version)

	credential, err := test.repository.GetBYOKCredential(
		ctx,
		test.userA,
		test.tenantA,
		connectionID,
	)
	require.NoError(t, err)
	require.Zero(t, credential.ActiveVaultVersion)
	require.Equal(t, domain.CredentialStateVerificationFailed, credential.State)
	require.EqualValues(t, 1, test.pendingSecretCleanupCount(ctx, t, "destroy_version"))
}

func TestBYOKTransientFailureStaysPending(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	pool := test.repository.(*Repository).pool

	service := newModelAccessServiceWithVerifier(
		t,
		pool,
		test.secretStore,
		transientIntegrationVerifier{},
	)

	ctx := context.Background()

	_, err := service.CreateBYOKConnection(
		ctx,
		application.CreateBYOKConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			DisplayName:    "Transient OpenAI",
			APIKey:         "transient-key-do-not-log",
		},
	)
	require.ErrorIs(t, err, domain.ErrCredentialVerificationUnavailable)

	var connectionID string

	test.scopedLookup(
		ctx,
		t,
		test.tenantA,
		&connectionID,
		`
			SELECT connection_id::text
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
			  AND connection_type = 'byok'
			ORDER BY created_at DESC
			LIMIT 1
		`,
		test.tenantA,
	)

	connection, err := service.GetConnection(
		ctx,
		test.userA,
		test.tenantA,
		connectionID,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ConnectionStatusPendingVerification, connection.Status)
	require.EqualValues(t, 1, connection.Version)
}

func TestBYOKCreateIsIdempotent(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	key := uuid.NewString()

	input := application.CreateBYOKConnectionInput{
		ActorUserID:    test.userA,
		TenantID:       test.tenantA,
		IdempotencyKey: key,
		ProviderKey:    "openai",
		DisplayName:    "Idempotent OpenAI",
		APIKey:         "idempotent-key-do-not-log",
	}

	ctx := context.Background()

	first, err := test.service.CreateBYOKConnection(ctx, input)
	require.NoError(t, err)

	second, err := test.service.CreateBYOKConnection(ctx, input)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.ProviderPoolKey, second.ProviderPoolKey)
	require.Equal(t, domain.ConnectionStatusActive, second.Status)
	require.EqualValues(t, 2, test.outboxCountForAggregate(ctx, t, first.ID))
}

func TestBYOKMetadataOnlyCreateRejected(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	_, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypeBYOK,
			DisplayName:    "No credential",
		},
	)
	require.ErrorIs(t, err, domain.ErrCredentialRequired)
}

func TestRotateBYOKCredentialKeepsConnectionIdentity(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	original := test.createBYOK(t, "old-valid-key-1234567890")
	originalFingerprint := original.CredentialFingerprint

	rotated, err := test.service.RotateBYOKCredential(
		context.Background(),
		application.RotateBYOKCredentialInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    original.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: original.Version,
			APIKey:          "new-valid-key-1234567890",
		},
	)
	require.NoError(t, err)

	require.Equal(t, original.ID, rotated.ID)
	require.NotEqual(t, originalFingerprint, rotated.CredentialFingerprint)
	require.Equal(t, original.Version+1, rotated.Version)
	require.Equal(t, domain.ConnectionStatusActive, rotated.Status)
}

func TestRejectedRotationPreservesOldKey(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	original := test.createBYOK(t, "keep-me-valid-1234567890")
	before := test.getCredential(ctx, t, original.ID)

	service := newModelAccessServiceWithVerifier(
		t,
		test.repository.(*Repository).pool,
		test.secretStore,
		rejectingIntegrationVerifier{},
	)

	_, err := service.RotateBYOKCredential(
		ctx,
		application.RotateBYOKCredentialInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    original.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: original.Version,
			APIKey:          "bad-replacement-key-123",
		},
	)
	require.ErrorIs(t, err, domain.ErrCredentialRejected)

	after := test.getCredential(ctx, t, original.ID)

	require.Equal(t, before.ActiveVaultVersion, after.ActiveVaultVersion)
	require.Equal(t, before.FingerprintDisplay, after.FingerprintDisplay)
	require.Equal(t, domain.CredentialStateActive, after.State)
	require.EqualValues(t, 1, test.pendingSecretCleanupCount(ctx, t, "destroy_version"))
}

func TestArchiveBYOKQueuesSecretPurge(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	connection := test.createBYOK(t, "archive-me-valid-1234567890")

	archived, err := test.service.ArchiveConnection(
		ctx,
		application.ArchiveConnectionInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: connection.Version,
		},
	)
	require.NoError(t, err)
	require.Equal(t, domain.ConnectionStatusArchived, archived.Status)

	credential := test.getCredential(ctx, t, connection.ID)
	require.Equal(t, domain.CredentialStateDestroyed, credential.State)
	require.NotNil(t, credential.DestroyedAt)
	require.EqualValues(t, 1, test.pendingSecretCleanupCount(ctx, t, "purge_secret"))
}

func TestPublicConnectionDoesNotExposeSecretReference(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	connection := test.createBYOK(t, "no-leak-valid-1234567890")

	message := protoutil.Connection(connection)

	encoded, err := protojson.Marshal(message)
	require.NoError(t, err)

	require.NotContains(t, string(encoded), "vault://")
	require.NotContains(t, string(encoded), "secretRef")
	require.Contains(t, string(encoded), "credentialFingerprint")
}
