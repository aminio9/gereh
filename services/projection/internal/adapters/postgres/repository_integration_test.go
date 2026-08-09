package postgres

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func projectionTestRepository(t *testing.T) *Repository {
	t.Helper()

	databaseURL := os.Getenv("PROJECTION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROJECTION_TEST_DATABASE_URL is not configured")
	}

	pool, err := pgxpool.New(
		context.Background(),
		databaseURL,
	)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	return New(pool)
}

func testServicePrincipalID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate service principal UUID: %v", err)
	}

	return id.String()
}

// scopedQueryRow runs one read inside a tenant-scoped transaction so FORCE
// RLS permits the row to be observed.
func scopedQueryRow(
	t *testing.T,
	repository *Repository,
	tenantID string,
	query string,
	args ...any,
) pgx.Row {
	t.Helper()

	ctx := context.Background()

	transaction, err := repository.database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			tenantID,
			testServicePrincipalID(t),
			"projection-test-read",
			"projection-test-read",
		),
		pgx.TxOptions{},
	)
	require.NoError(t, err)

	t.Cleanup(func() { _ = transaction.Rollback(ctx) })

	return transaction.QueryRow(ctx, query, args...)
}

func cleanProjectionTenant(
	t *testing.T,
	repository *Repository,
	tenantIDs ...string,
) {
	t.Helper()

	ctx := context.Background()

	for _, tenantID := range tenantIDs {
		// FORCE RLS means a raw pool Exec cannot see any row: cleanups
		// must run inside a tenant-scoped transaction.
		transaction, err := repository.database.Begin(
			ctx,
			platformpostgres.ServiceTenantScope(
				tenantID,
				testServicePrincipalID(t),
				"projection-test-cleanup",
				"projection-test-cleanup",
			),
			pgx.TxOptions{},
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_search_documents
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_task_activity
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_task_assignments
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_task_dependencies
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_tasks
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_projects
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_goals
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_agents
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_companies
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_tenants
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_consumed_events
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		_, err = transaction.Exec(
			ctx,
			`
				DELETE FROM projection_tenant_watermarks
				WHERE tenant_id = $1::uuid
			`,
			tenantID,
		)
		require.NoError(t, err)

		require.NoError(t, transaction.Commit(ctx))
	}
}

func companyEvent(
	t *testing.T,
	tenantID string,
	version uint64,
	eventID string,
) (domain.EventMeta, ports.ApplyFunc) {
	return companyEventPartition(
		t,
		tenantID,
		version,
		eventID,
		testPartition(),
	)
}

// testPartition returns a unique Kafka partition per test so the
// (topic, partition, offset) inbox constraint never collides across tests.
var testPartitionCounter atomic.Int32

func testPartition() int32 {
	return testPartitionCounter.Add(1)
}

func companyEventPartition(
	t *testing.T,
	tenantID string,
	version uint64,
	eventID string,
	partition int32,
) (domain.EventMeta, ports.ApplyFunc) {
	t.Helper()

	now := time.Now().UTC()

	companyID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate company UUID: %v", err)
	}

	var hashBytes [32]byte
	for index := range hashBytes {
		hashBytes[index] = byte(index + 1)
	}

	meta := domain.EventMeta{
		EventID:          eventID,
		TenantID:         tenantID,
		Topic:            "gereh.organization.company.events.v1",
		Partition:        partition,
		Offset:           int64(version),
		EventType:        "company.created",
		EventVersion:     1,
		AggregateType:    "company",
		AggregateID:      companyID.String(),
		AggregateVersion: version,
		EventHash:        hashBytes[:],
		OccurredAt:       now,
		ProcessedAt:      now,
	}

	company := domain.Company{
		TenantID:    tenantID,
		ID:          companyID.String(),
		Slug:        "acme-" + companyID.String()[:8],
		DisplayName: "Acme Corporation",
		Description: "Test company",
		Status:      "active",
		IsDefault:   true,

		SourceVersion: version,
		SourceEventID: eventID,
		SourceEventAt: now,
		ProjectedAt:   now,
	}

	apply := func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		return transaction.UpsertCompany(ctx, company)
	}

	return meta, apply
}

// TestDuplicateEventIsAppliedOnce verifies the inbox idempotency contract:
// a redelivered event mutates nothing and is safely checkpointed again.
func TestDuplicateEventIsAppliedOnce(t *testing.T) {
	repository := projectionTestRepository(t)

	ctx := context.Background()

	tenantID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	principalID := testServicePrincipalID(t)

	t.Cleanup(func() {
		cleanProjectionTenant(t, repository, tenantID.String())
	})

	eventID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	meta, apply := companyEvent(
		t,
		tenantID.String(),
		3,
		eventID.String(),
	)

	applied, err := repository.ApplyEvent(
		ctx,
		principalID,
		meta,
		apply,
	)
	require.NoError(t, err)
	require.True(t, applied)

	// Redeliver the identical record.
	applied, err = repository.ApplyEvent(
		ctx,
		principalID,
		meta,
		apply,
	)
	require.NoError(t, err)
	require.False(t, applied)

	var companyCount int

	err = scopedQueryRow(
		t,
		repository,
		tenantID.String(),
		`
			SELECT count(*)
			FROM projection_companies
			WHERE tenant_id = $1::uuid
		`,
		tenantID.String(),
	).Scan(&companyCount)
	require.NoError(t, err)
	require.EqualValues(t, 1, companyCount)

	var consumedCount int

	err = scopedQueryRow(
		t,
		repository,
		tenantID.String(),
		`
			SELECT count(*)
			FROM projection_consumed_events
			WHERE event_id = $1::uuid
		`,
		eventID.String(),
	).Scan(&consumedCount)
	require.NoError(t, err)
	require.EqualValues(t, 1, consumedCount)
}

// TestEventIdentityConflict rejects the same event ID with different content.
func TestEventIdentityConflict(t *testing.T) {
	repository := projectionTestRepository(t)

	ctx := context.Background()

	tenantID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	principalID := testServicePrincipalID(t)

	t.Cleanup(func() {
		cleanProjectionTenant(t, repository, tenantID.String())
	})

	eventID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	meta, apply := companyEvent(
		t,
		tenantID.String(),
		3,
		eventID.String(),
	)

	applied, err := repository.ApplyEvent(
		ctx,
		principalID,
		meta,
		apply,
	)
	require.NoError(t, err)
	require.True(t, applied)

	// Reuse the event ID with a different content hash.
	altered := meta
	altered.EventHash[0] ^= 0xFF

	applied, err = repository.ApplyEvent(
		ctx,
		principalID,
		altered,
		apply,
	)
	require.ErrorIs(
		t,
		err,
		domain.ErrEventIdentityConflict,
	)
	require.False(t, applied)
}

// TestOlderAggregateVersionCannotOverwriteNewer verifies the stale-event
// guard: an older aggregate version must not clobber a newer row.
func TestOlderAggregateVersionCannotOverwriteNewer(t *testing.T) {
	repository := projectionTestRepository(t)

	ctx := context.Background()

	tenantID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	principalID := testServicePrincipalID(t)

	t.Cleanup(func() {
		cleanProjectionTenant(t, repository, tenantID.String())
	})

	// Apply version 3 first.
	eventNew, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	metaNew, applyNew := companyEvent(
		t,
		tenantID.String(),
		3,
		eventNew.String(),
	)

	applied, err := repository.ApplyEvent(
		ctx,
		principalID,
		metaNew,
		applyNew,
	)
	require.NoError(t, err)
	require.True(t, applied)

	// Then deliver version 2 for the same aggregate.
	eventOld, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	metaOld, applyOld := companyEvent(
		t,
		tenantID.String(),
		2,
		eventOld.String(),
	)

	oldAggregateID := metaNew.AggregateID
	metaOld.AggregateID = oldAggregateID

	applied, err = repository.ApplyEvent(
		ctx,
		principalID,
		metaOld,
		applyOld,
	)
	require.NoError(t, err)
	require.True(t, applied)

	var sourceVersion uint64
	var displayName string

	err = scopedQueryRow(
		t,
		repository,
		tenantID.String(),
		`
			SELECT source_version, display_name
			FROM projection_companies
			WHERE tenant_id = $1::uuid
		`,
		tenantID.String(),
	).Scan(&sourceVersion, &displayName)
	require.NoError(t, err)
	require.EqualValues(t, 3, sourceVersion)
	require.Equal(t, "Acme Corporation", displayName)
}

// TestServiceScopeCannotWriteAnotherTenant verifies forced RLS: a service
// transaction scoped to tenant A cannot mutate tenant B rows.
func TestServiceScopeCannotWriteAnotherTenant(t *testing.T) {
	repository := projectionTestRepository(t)

	ctx := context.Background()

	tenantA, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	tenantB, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	principalID := testServicePrincipalID(t)

	t.Cleanup(func() {
		cleanProjectionTenant(
			t,
			repository,
			tenantA.String(),
			tenantB.String(),
		)
	})

	// Seed tenant A through the service transaction path.
	eventA, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	metaA, applyA := companyEvent(
		t,
		tenantA.String(),
		1,
		eventA.String(),
	)

	applied, err := repository.ApplyEvent(
		ctx,
		principalID,
		metaA,
		applyA,
	)
	require.NoError(t, err)
	require.True(t, applied)

	// Now attempt to insert tenant B's company inside a transaction that
	// is RLS-scoped to tenant A. The write must be rejected by RLS.
	transaction, err := repository.beginServiceTenant(
		ctx,
		tenantA.String(),
		principalID,
		pgxTxOptions(),
	)
	require.NoError(t, err)

	defer func() { _ = transaction.Rollback(ctx) }()

	eventB, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate event UUID: %v", err)
	}

	_, applyB := companyEvent(
		t,
		tenantB.String(),
		1,
		eventB.String(),
	)

	err = applyB(
		ctx,
		&projectionTransaction{tx: transaction},
	)
	require.Error(t, err)

	// Tenant B sees no company rows.
	var count int

	err = scopedQueryRow(
		t,
		repository,
		tenantB.String(),
		`
			SELECT count(*)
			FROM projection_companies
			WHERE tenant_id = $1::uuid
		`,
		tenantB.String(),
	).Scan(&count)
	require.NoError(t, err)
	require.EqualValues(t, 0, count)
}

// TestCrossTenantReadIsolation verifies tenant-scoped read queries cannot
// observe another tenant's rows.
func TestCrossTenantReadIsolation(t *testing.T) {
	repository := projectionTestRepository(t)

	ctx := context.Background()

	tenantA, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	tenantB, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("generate tenant UUID: %v", err)
	}

	principalID := testServicePrincipalID(t)

	t.Cleanup(func() {
		cleanProjectionTenant(
			t,
			repository,
			tenantA.String(),
			tenantB.String(),
		)
	})

	// Seed two tenants, one per partition so checkpoint rows do not clash.
	for _, tenant := range []uuid.UUID{tenantA, tenantB} {
		eventID, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("generate event UUID: %v", err)
		}

		meta, apply := companyEventPartition(
			t,
			tenant.String(),
			1,
			eventID.String(),
			testPartition(),
		)

		applied, err := repository.ApplyEvent(
			ctx,
			principalID,
			meta,
			apply,
		)
		require.NoError(t, err)
		require.True(t, applied)
	}

	// Read tenant A's dashboard as tenant A.
	summary, _, err := repository.GetDashboardSummary(
		ctx,
		"00000000-0000-0000-0000-000000000001",
		tenantA.String(),
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.CompaniesTotal)
}

func pgxTxOptions() pgx.TxOptions {
	return pgx.TxOptions{}
}
