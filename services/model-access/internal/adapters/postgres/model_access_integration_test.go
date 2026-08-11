package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// allowAllAuthorizer permits every tenant permission in integration tests;
// authorization behavior itself is covered by Tenant Service tests.
type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Require(
	_ context.Context,
	_ string,
	_ string,
	_ tenantv1.Permission,
) error {
	return nil
}

// modelAccessIntegrationTest wires a repository-backed application service
// against the local model_access_db runtime role.
type modelAccessIntegrationTest struct {
	repository ports.Repository
	service    *application.Service

	tenantA string
	userA   string

	tenantB string
	userB   string
}

func newModelAccessIntegrationTest(t *testing.T) *modelAccessIntegrationTest {
	t.Helper()

	databaseURL := os.Getenv("MODEL_ACCESS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("MODEL_ACCESS_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	repository := New(pool)

	service, err := application.New(
		repository,
		allowAllAuthorizer{},
		application.Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
	)
	require.NoError(t, err)

	return &modelAccessIntegrationTest{
		repository: repository,
		service:    service,

		tenantA: uuid.NewString(),
		userA:   uuid.NewString(),

		tenantB: uuid.NewString(),
		userB:   uuid.NewString(),
	}
}

func (test *modelAccessIntegrationTest) createConnection(
	t *testing.T,
	tenantID string,
	userID string,
	displayName string,
) domain.Connection {
	t.Helper()

	value, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    userID,
			TenantID:       tenantID,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypeBYOK,
			DisplayName:    displayName,
		},
	)
	require.NoError(t, err)

	return value
}

func (test *modelAccessIntegrationTest) connectionCount(
	ctx context.Context,
	t *testing.T,
	tenantID string,
) int {
	t.Helper()

	var count int

	err := test.repository.(*Repository).database.Pool().QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	).Scan(&count)
	require.NoError(t, err)

	return count
}

func (test *modelAccessIntegrationTest) revisionCount(
	ctx context.Context,
	t *testing.T,
	tenantID string,
	connectionID string,
) int {
	t.Helper()

	var count int

	err := test.repository.(*Repository).database.Pool().QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM model_access_connection_revisions
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
		`,
		tenantID,
		connectionID,
	).Scan(&count)
	require.NoError(t, err)

	return count
}

func (test *modelAccessIntegrationTest) outboxCountForAggregate(
	ctx context.Context,
	t *testing.T,
	connectionID string,
) int {
	t.Helper()

	var count int

	err := test.repository.(*Repository).database.Pool().QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM model_access_outbox
			WHERE partition_key = $1
		`,
		connectionID,
	).Scan(&count)
	require.NoError(t, err)

	return count
}

func TestCreateConnectionIsIdempotent(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	idempotencyKey := uuid.NewString()

	input := application.CreateConnectionInput{
		ActorUserID:    test.userA,
		TenantID:       test.tenantA,
		IdempotencyKey: idempotencyKey,
		ProviderKey:    "openai",
		ConnectionType: domain.ConnectionTypeBYOK,
		DisplayName:    "Production OpenAI",
	}

	first, err := test.service.CreateConnection(ctx, input)
	require.NoError(t, err)

	second, err := test.service.CreateConnection(ctx, input)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.EqualValues(t, 1, test.connectionCount(ctx, t, test.tenantA))
	require.EqualValues(t, 1, test.revisionCount(ctx, t, test.tenantA, first.ID))
	require.EqualValues(t, 1, test.outboxCountForAggregate(ctx, t, first.ID))
}

func TestIdempotencyKeyCannotChangeRequest(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	key := uuid.NewString()

	input := application.CreateConnectionInput{
		ActorUserID:    test.userA,
		TenantID:       test.tenantA,
		IdempotencyKey: key,
		ProviderKey:    "openai",
		ConnectionType: domain.ConnectionTypeBYOK,
		DisplayName:    "OpenAI",
	}

	_, err := test.service.CreateConnection(ctx, input)
	require.NoError(t, err)

	input.DisplayName = "Different connection"

	_, err = test.service.CreateConnection(ctx, input)
	require.ErrorIs(t, err, domain.ErrIdempotencyConflict)
}

func TestConnectionRLSPreventsCrossTenantRead(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"Tenant A OpenAI",
	)

	_, err := test.repository.GetConnection(
		context.Background(),
		test.userB,
		test.tenantB,
		connection.ID,
	)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConnectionRLSPreventsCrossTenantWrite(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"Tenant A OpenAI",
	)

	_, err := test.repository.UpdateConnection(
		context.Background(),
		ports.UpdateConnectionParams{
			ActorUserID:          test.userB,
			TenantID:             test.tenantB,
			ConnectionID:         connection.ID,
			ExpectedVersion:      connection.Version,
			DisplayName:          "Stolen",
			UpdatedAt:            connection.UpdatedAt,
			IdempotencyKey:       uuid.NewString(),
			RequestHash:          make([]byte, 32),
			IdempotencyExpiresAt: connection.UpdatedAt.Add(24 * time.Hour),
		},
	)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUnscopedConnectionQueryReturnsZeroRows(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	test.createConnection(t, test.tenantA, test.userA, "Tenant A OpenAI")

	pool := test.repository.(*Repository).database.Pool()

	transaction, err := pool.Begin(context.Background())
	require.NoError(t, err)

	defer func() { _ = transaction.Rollback(context.Background()) }()

	_, err = transaction.Exec(
		context.Background(),
		`
			SELECT set_config('app.scope_kind', 'tenant', TRUE);
			SELECT set_config('app.tenant_id', $1, TRUE);
		`,
		test.tenantB,
	)
	require.NoError(t, err)

	var count int

	err = transaction.QueryRow(
		context.Background(),
		`
			SELECT count(*)
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
		`,
		test.tenantA,
	).Scan(&count)
	require.NoError(t, err)

	require.Zero(t, count)
}

func TestProvidersListedInSortOrder(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	providers, err := test.service.ListProviders(
		context.Background(),
		test.userA,
		test.tenantA,
	)
	require.NoError(t, err)

	require.NotEmpty(t, providers)

	keys := make([]string, 0, len(providers))

	for _, provider := range providers {
		keys = append(keys, provider.Key)
	}

	require.Equal(t, "openai", keys[0])
	require.Equal(t, "custom", keys[len(keys)-1])

	for _, provider := range providers {
		require.True(t, provider.Enabled)
		require.NotEmpty(t, provider.SupportedConnectionTypes)
	}
}

func TestUnsupportedConnectionTypeRejected(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	_, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "nous",
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
			DisplayName:    "Nous private endpoint",
		},
	)
	require.ErrorIs(t, err, domain.ErrUnsupportedConnectionType)
}

func TestUnknownProviderRejected(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	_, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "not-a-provider",
			ConnectionType: domain.ConnectionTypeBYOK,
			DisplayName:    "Bogus",
		},
	)
	require.ErrorIs(t, err, domain.ErrProviderNotFound)
}

func TestUpdateConnectionOptimisticConcurrency(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"Original name",
	)

	updated, err := test.service.UpdateConnection(
		ctx,
		application.UpdateConnectionInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: connection.Version,
			DisplayName:     "Renamed",
		},
	)
	require.NoError(t, err)

	require.EqualValues(t, connection.Version+1, updated.Version)
	require.Equal(t, "Renamed", updated.DisplayName)

	// Stale version must fail with no revision/event written.
	_, err = test.service.UpdateConnection(
		ctx,
		application.UpdateConnectionInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: connection.Version,
			DisplayName:     "Stale rename",
		},
	)
	require.ErrorIs(t, err, domain.ErrVersionConflict)

	require.EqualValues(
		t,
		2,
		test.revisionCount(ctx, t, test.tenantA, connection.ID),
	)
	require.EqualValues(
		t,
		2,
		test.outboxCountForAggregate(ctx, t, connection.ID),
	)
}

func TestArchiveConnection(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"To archive",
	)

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

	require.Equal(
		t,
		domain.ConnectionStatusArchived,
		archived.Status,
	)
	require.NotNil(t, archived.ArchivedAt)
	require.EqualValues(t, connection.Version+1, archived.Version)

	// Second archive (new key, stale version) must be rejected.
	_, err = test.service.ArchiveConnection(
		ctx,
		application.ArchiveConnectionInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: archived.Version,
		},
	)
	require.ErrorIs(t, err, domain.ErrConnectionArchived)

	// Update after archive must be rejected.
	_, err = test.service.UpdateConnection(
		ctx,
		application.UpdateConnectionInput{
			ActorUserID:     test.userA,
			TenantID:        test.tenantA,
			ConnectionID:    connection.ID,
			IdempotencyKey:  uuid.NewString(),
			ExpectedVersion: archived.Version,
			DisplayName:     "Zombie rename",
		},
	)
	require.ErrorIs(t, err, domain.ErrConnectionArchived)
}

func TestSameDisplayNameCannotBeReusedWhileActive(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	test.createConnection(t, test.tenantA, test.userA, "Unique name")

	_, err := test.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypeBYOK,
			DisplayName:    "Unique name",
		},
	)
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestDifferentTenantCanReuseDisplayName(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	test.createConnection(t, test.tenantA, test.userA, "Shared name")

	other, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    test.userB,
			TenantID:       test.tenantB,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypeBYOK,
			DisplayName:    "Shared name",
		},
	)
	require.NoError(t, err)
	require.Equal(t, test.tenantB, other.TenantID)
}

func TestRuntimeRoleHasNoBypassRLS(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	var bypassRLS bool
	var isSuperuser bool

	err := test.repository.(*Repository).database.Pool().QueryRow(
		context.Background(),
		`
			SELECT
				rolbypassrls,
				rolsuper
			FROM pg_roles
			WHERE rolname = current_user
		`,
	).Scan(&bypassRLS, &isSuperuser)
	require.NoError(t, err)

	require.False(t, bypassRLS)
	require.False(t, isSuperuser)
}

func TestSchemaContainsNoCredentialColumns(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	rows, err := test.repository.(*Repository).database.Pool().Query(
		context.Background(),
		`
			SELECT
				table_name,
				column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND (
				column_name ILIKE '%credential%'
				OR column_name ILIKE '%secret%'
				OR column_name ILIKE '%api_key%'
				OR column_name ILIKE '%token%'
				OR column_name ILIKE '%key%'
			  )
			ORDER BY table_name, column_name
		`,
	)
	require.NoError(t, err)
	defer rows.Close()

	var offenders []string

	for rows.Next() {
		var tableName string
		var columnName string

		require.NoError(t, rows.Scan(&tableName, &columnName))

		offenders = append(
			offenders,
			tableName+"."+columnName,
		)
	}

	require.Empty(t, offenders, "schema must contain no credential columns")
}

func TestEventEnvelopeCarriesNoSecretFields(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"Event check",
	)

	var envelope []byte

	err := test.repository.(*Repository).database.Pool().QueryRow(
		context.Background(),
		`
			SELECT envelope
			FROM model_access_outbox
			WHERE partition_key = $1
			ORDER BY outbox_id
			LIMIT 1
		`,
		connection.ID,
	).Scan(&envelope)
	require.NoError(t, err)

	require.NotContains(t, string(envelope), "apiKey")
	require.NotContains(t, string(envelope), "credential")
	require.NotContains(t, string(envelope), "secret")
	require.NotContains(t, string(envelope), "bearer")
}
