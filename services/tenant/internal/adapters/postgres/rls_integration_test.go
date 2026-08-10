package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rlsTestDatabase(t *testing.T) *platformpostgres.Database {
	t.Helper()

	databaseURL := os.Getenv("TENANT_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TENANT_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(
		ctx,
		databaseURL,
	)
	if err != nil {
		t.Fatalf("create RLS test pool: %v", err)
	}

	t.Cleanup(pool.Close)

	database := platformpostgres.Wrap(pool)

	if err := database.VerifyRuntimeRole(
		ctx,
		[]string{"public"},
	); err != nil {
		t.Fatalf(
			"verify runtime role: %v",
			err,
		)
	}

	return database
}

func seedRLSTenant(
	ctx context.Context,
	t *testing.T,
	repository *Repository,
	actorUserID string,
	requestID string,
	slug string,
) string {
	t.Helper()

	result, err := createTestTenant(
		ctx,
		t,
		repository,
		actorUserID,
		requestID,
		slug,
	)
	if err != nil {
		t.Fatalf("seed RLS tenant: %v", err)
	}

	t.Cleanup(func() {
		cleanupTenant(
			t,
			testCleanupPool(t),
			result.Context.Tenant.ID,
		)
	})

	return result.Context.Tenant.ID
}

func TestTenantRLSIsolationAndScopeCleanup(t *testing.T) {
	database := rlsTestDatabase(t)

	repository := New(database.Pool())

	ctx := context.Background()

	userA := uuid.NewString()
	userB := uuid.NewString()

	tenantA := seedRLSTenant(
		ctx,
		t,
		repository,
		userA,
		"rls-isolation-a",
		"rls-a-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)
	tenantB := seedRLSTenant(
		ctx,
		t,
		repository,
		userB,
		"rls-isolation-b",
		"rls-b-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)

	// Tenant A cannot read tenant B.
	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantA,
			userA,
			"rls-test-a",
			"rls-test-a",
		),
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		t.Fatalf("begin tenant A scope: %v", err)
	}

	var visibleCount int

	err = transaction.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM tenant_tenants
		`,
	).Scan(&visibleCount)
	if err != nil {
		t.Fatalf("count tenants under scope A: %v", err)
	}

	if visibleCount > 1 {
		t.Fatalf(
			"scope A can see %d tenants, want ≤ 1",
			visibleCount,
		)
	}

	var crossTenantID string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT tenant_id::text
			FROM tenant_tenants
			WHERE tenant_id = $1::uuid
		`,
		tenantB,
	).Scan(&crossTenantID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"cross-tenant read error = %v, want pgx.ErrNoRows",
			err,
		)
	}

	if err := transaction.Commit(ctx); err != nil {
		t.Fatalf("commit scope A transaction: %v", err)
	}

	// Transaction-local context must not survive COMMIT. An unscoped pooled
	// query must not see any tenant rows (RLS default-deny without scope).
	err = database.Pool().QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM tenant_tenants
		`,
	).Scan(&visibleCount)
	if err != nil {
		t.Fatalf("count unscoped tenants: %v", err)
	}

	if visibleCount != 0 {
		t.Fatalf(
			"unscoped tenant count = %d, want 0",
			visibleCount,
		)
	}
}

func TestTenantRLSCannotUpdateAnotherTenant(t *testing.T) {
	database := rlsTestDatabase(t)

	repository := New(database.Pool())

	ctx := context.Background()

	userA := uuid.NewString()
	userB := uuid.NewString()

	tenantA := seedRLSTenant(
		ctx,
		t,
		repository,
		userA,
		"rls-update-a",
		"rls-upd-a-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)
	tenantB := seedRLSTenant(
		ctx,
		t,
		repository,
		userB,
		"rls-update-b",
		"rls-upd-b-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)

	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantA,
			userA,
			"rls-update",
			"rls-update",
		),
		pgx.TxOptions{},
	)
	if err != nil {
		t.Fatalf("begin scoped transaction: %v", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	// The UPDATE must affect zero rows because tenant B's rows are invisible
	// to the tenant-A scope.
	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE tenant_tenants
			SET display_name = 'pwned'
			WHERE tenant_id = $1::uuid
		`,
		tenantB,
	)
	if err != nil {
		t.Fatalf("cross-tenant update: %v", err)
	}

	if commandTag.RowsAffected() != 0 {
		t.Fatalf(
			"cross-tenant update affected %d rows",
			commandTag.RowsAffected(),
		)
	}
}

func TestTenantRLSRejectsWrongTenantInsert(t *testing.T) {
	database := rlsTestDatabase(t)

	ctx := context.Background()

	scopeTenantID := uuid.NewString()
	insertTenantID := uuid.NewString()
	userID := uuid.NewString()

	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			scopeTenantID,
			userID,
			"wrong-tenant-insert",
			"wrong-tenant-insert",
		),
		pgx.TxOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO tenant_tenants (
				tenant_id,
				slug,
				display_name,
				region,
				retention_days,
				created_by_user_id,
				creation_request_id
			)
			VALUES (
				$1::uuid,
				'rls-wrong',
				'Wrong tenant',
				'local',
				90,
				$2::uuid,
				'wrong'
			)
		`,
		insertTenantID,
		userID,
	)

	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) ||
		postgresError.Code != "42501" {
		t.Fatalf(
			"insert error = %v, want RLS SQLSTATE 42501",
			err,
		)
	}
}

func TestPrincipalScopeDiscoversOnlyOwnMemberships(t *testing.T) {
	database := rlsTestDatabase(t)

	repository := New(database.Pool())

	ctx := context.Background()

	userA := uuid.NewString()
	userB := uuid.NewString()

	tenantA := seedRLSTenant(
		ctx,
		t,
		repository,
		userA,
		"rls-principal-a",
		"rls-p-a-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)
	seedRLSTenant(
		ctx,
		t,
		repository,
		userB,
		"rls-principal-b",
		"rls-p-b-"+fmt.Sprintf("%d", time.Now().UTC().UnixNano()),
	)

	transaction, err := database.Begin(
		ctx,
		platformpostgres.PrincipalScope(
			userA,
			"rls-principal",
			"rls-principal",
		),
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		t.Fatalf("begin principal scope: %v", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var tenantID string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT tenant_id::text
			FROM tenant_tenants
			ORDER BY tenant_id
		`,
	).Scan(&tenantID)
	if err != nil {
		t.Fatalf("list tenants under principal scope: %v", err)
	}

	if tenantID != tenantA {
		t.Fatalf(
			"principal sees tenant %q, want %q",
			tenantID,
			tenantA,
		)
	}
}
