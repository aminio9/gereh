package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/model-access/internal/application"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (allowAllAuthorizer) RequireWithContext(
	_ context.Context,
	_ string,
	_ string,
	_ tenantv1.Permission,
) (ports.TenantAccessContext, error) {
	return ports.TenantAccessContext{
		Region:  "global",
		PlanKey: "test",
		Features: map[string]bool{
			"platform_managed_models": true,
		},
		Limits: map[string]int64{},
	}, nil
}

// modelAccessIntegrationTest wires a repository-backed application service
// against the local model_access_db runtime role.
type modelAccessIntegrationTest struct {
	repository ports.Repository
	service    *application.Service

	secretStore *integrationSecretStore

	adminPool *pgxpool.Pool

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

	adminURL := os.Getenv("MODEL_ACCESS_TEST_ADMIN_DATABASE_URL")
	if adminURL == "" {
		adminURL = databaseURL
	}

	adminPool, err := pgxpool.New(ctx, adminURL)
	require.NoError(t, err)

	t.Cleanup(adminPool.Close)

	repository := New(pool)

	store := newIntegrationSecretStore()

	service, err := application.New(
		repository,
		allowAllAuthorizer{},
		store,
		integrationVerifier,
		integrationFingerprinter(t),
		application.Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
	)
	require.NoError(t, err)

	return &modelAccessIntegrationTest{
		repository:  repository,
		service:     service,
		secretStore: store,
		adminPool:   adminPool,

		tenantA: uuid.NewString(),
		userA:   uuid.NewString(),

		tenantB: uuid.NewString(),
		userB:   uuid.NewString(),
	}
}

func (test *modelAccessIntegrationTest) createBYOK(
	t *testing.T,
	apiKey string,
) domain.Connection {
	t.Helper()

	value, err := test.service.CreateBYOKConnection(
		context.Background(),
		application.CreateBYOKConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			DisplayName:    "Customer OpenAI",
			APIKey:         apiKey,
		},
	)
	require.NoError(t, err)

	return value
}

// newModelAccessServiceWithVerifier wires a service with a specific verifier
// against the same repository and database, reusing the test's secret store.
func newModelAccessServiceWithVerifier(
	t *testing.T,
	pool *pgxpool.Pool,
	store *integrationSecretStore,
	verifier ports.CredentialVerifier,
) *application.Service {
	t.Helper()

	repository := New(pool)

	service, err := application.New(
		repository,
		allowAllAuthorizer{},
		store,
		verifier,
		integrationFingerprinter(t),
		application.Config{
			EventTopic:     "gereh.model.events.v1",
			IdempotencyTTL: 24 * time.Hour,
		},
	)
	require.NoError(t, err)

	return service
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
			ProviderKey:    "custom",
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
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

	return test.scopedCount(
		ctx,
		t,
		tenantID,
		`
			SELECT count(*)
			FROM model_access_connections
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)
}

// scopedCount runs a count query inside a tenant-scoped read-only
// transaction so FORCE RLS sees the selected tenant.
func (test *modelAccessIntegrationTest) scopedCount(
	ctx context.Context,
	t *testing.T,
	tenantID string,
	query string,
	args ...any,
) int {
	t.Helper()

	database := test.repository.(*Repository).database

	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantID,
			test.userA,
			"",
			"",
		),
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	require.NoError(t, err)

	defer func() { _ = transaction.Rollback(ctx) }()

	var count int

	err = transaction.QueryRow(ctx, query, args...).Scan(&count)
	require.NoError(t, err)

	return count
}

// scopedLookup scans a single value inside a tenant-scoped transaction.
func (test *modelAccessIntegrationTest) scopedLookup(
	ctx context.Context,
	t *testing.T,
	tenantID string,
	dest any,
	query string,
	args ...any,
) {
	t.Helper()

	database := test.repository.(*Repository).database

	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantID,
			test.userA,
			"",
			"",
		),
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	require.NoError(t, err)

	defer func() { _ = transaction.Rollback(ctx) }()

	err = transaction.QueryRow(ctx, query, args...).Scan(dest)
	require.NoError(t, err)
}

func (test *modelAccessIntegrationTest) revisionCount(
	ctx context.Context,
	t *testing.T,
	tenantID string,
	connectionID string,
) int {
	t.Helper()

	return test.scopedCount(
		ctx,
		t,
		tenantID,
		`
			SELECT count(*)
			FROM model_access_connection_revisions
			WHERE tenant_id = $1::uuid
			  AND connection_id = $2::uuid
		`,
		tenantID,
		connectionID,
	)
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
		ProviderKey:    "custom",
		ConnectionType: domain.ConnectionTypePrivateEndpoint,
		DisplayName:    "Production endpoint",
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
		ProviderKey:    "custom",
		ConnectionType: domain.ConnectionTypePrivateEndpoint,
		DisplayName:    "Endpoint",
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

	err = platformpostgres.ApplyScope(
		context.Background(),
		transaction,
		platformpostgres.TenantScope(
			test.tenantB,
			test.userB,
			"",
			"",
		),
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
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
			DisplayName:    "OpenAI private endpoint",
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
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
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
			ProviderKey:    "custom",
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
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
			ProviderKey:    "custom",
			ConnectionType: domain.ConnectionTypePrivateEndpoint,
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

	// The phase-18 schema intentionally contains internal metadata such as
	// secret_ref and credential_fingerprint. These are references and keyed
	// HMAC digests, never the raw credential. This test asserts that no
	// column can hold a raw provider secret: no api_key/secret/token/bearer
	// value columns and no provider_credential storage.
	rows, err := test.repository.(*Repository).database.Pool().Query(
		context.Background(),
		`
			SELECT
				table_name,
				column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND (
				column_name ILIKE '%api_key%'
				OR column_name ILIKE '%bearer%'
				OR column_name ILIKE '%credential_value%'
				OR column_name ILIKE '%raw_key%'
				OR column_name ILIKE '%provider_secret%'
				OR (
					column_name ILIKE '%secret%'
					AND column_name NOT IN (
						'secret_ref',
						'secret_version',
						'secret_cleanup'
					)
				)
				OR (
					column_name ILIKE '%token%'
					AND column_name NOT ILIKE '%bucket%'
				)
				OR (
					column_name ILIKE '%key%'
					AND column_name NOT IN (
						'provider_key',
						'pool_key',
						'fingerprint_key_id',
						'partition_key',
						'actor_user_id'
					)
					AND column_name NOT ILIKE '%provider_pool_key%'
					AND column_name NOT ILIKE '%idempotency_key%'
				)
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

	require.Empty(t, offenders, "schema must contain no raw credential columns")
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

func TestCreatePlatformManagedConnectionBecomesActive(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	connection, err := test.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "Gereh OpenAI",
		},
	)
	require.NoError(t, err)

	require.Equal(t, domain.ConnectionStatusActive, connection.Status)
	require.NotNil(t, connection.ProviderPoolKey)
	require.Equal(t, "gereh-openai-global", *connection.ProviderPoolKey)
	require.EqualValues(t, 1, connection.Version)
	require.EqualValues(t, 1, test.connectionCount(ctx, t, test.tenantA))
	require.EqualValues(t, 1, test.revisionCount(ctx, t, test.tenantA, connection.ID))
	require.EqualValues(t, 1, test.outboxCountForAggregate(ctx, t, connection.ID))
}

func TestPlatformManagedRejectsUnsupportedProvider(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	_, err := test.service.CreateConnection(
		context.Background(),
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "custom",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "Custom managed",
		},
	)
	require.ErrorIs(t, err, domain.ErrUnsupportedConnectionType)
}

func TestPlatformManagedFailsClosedWithoutPool(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	_, err := test.adminPool.Exec(
		ctx,
		`
			UPDATE model_access_provider_pools
			SET enabled = FALSE
			WHERE provider_key = 'openai'
		`,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = test.adminPool.Exec(
			context.Background(),
			`
				UPDATE model_access_provider_pools
				SET enabled = TRUE
				WHERE provider_key = 'openai'
			`,
		)
	})

	_, err = test.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "No Pool OpenAI",
		},
	)
	require.ErrorIs(t, err, domain.ErrPlatformManagedPoolUnavailable)
}

func TestPlatformManagedCreateIsIdempotent(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	key := uuid.NewString()

	input := application.CreateConnectionInput{
		ActorUserID:    test.userA,
		TenantID:       test.tenantA,
		IdempotencyKey: key,
		ProviderKey:    "openai",
		ConnectionType: domain.ConnectionTypePlatformManaged,
		DisplayName:    "Gereh OpenAI",
	}

	first, err := test.service.CreateConnection(ctx, input)
	require.NoError(t, err)

	second, err := test.service.CreateConnection(ctx, input)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.ProviderPoolKey, second.ProviderPoolKey)
	require.Equal(t, domain.ConnectionStatusActive, second.Status)
	require.EqualValues(t, 1, test.outboxCountForAggregate(ctx, t, first.ID))
}

func TestPlatformManagedRevisionContainsPoolKey(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	ctx := context.Background()

	connection, err := test.service.CreateConnection(
		ctx,
		application.CreateConnectionInput{
			ActorUserID:    test.userA,
			TenantID:       test.tenantA,
			IdempotencyKey: uuid.NewString(),
			ProviderKey:    "openai",
			ConnectionType: domain.ConnectionTypePlatformManaged,
			DisplayName:    "Revision Pool Check",
		},
	)
	require.NoError(t, err)

	var poolKey *string

	test.scopedLookup(
		ctx,
		t,
		test.tenantA,
		&poolKey,
		`
			SELECT provider_pool_key
			FROM model_access_connection_revisions
			WHERE connection_id = $1::uuid
			ORDER BY revision
			LIMIT 1
		`,
		connection.ID,
	)
	require.NotNil(t, poolKey)
	require.Equal(t, "gereh-openai-global", *poolKey)
}

func TestPrivateEndpointConnectionHasNoPoolKey(t *testing.T) {
	test := newModelAccessIntegrationTest(t)

	connection := test.createConnection(
		t,
		test.tenantA,
		test.userA,
		"Private endpoint no pool",
	)

	require.Nil(t, connection.ProviderPoolKey)
	require.Equal(t, domain.ConnectionStatusDraft, connection.Status)
}
