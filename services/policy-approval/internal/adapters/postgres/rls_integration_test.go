package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func policyRLSTestDatabase(t *testing.T) *platformpostgres.Database {
	t.Helper()

	databaseURL := os.Getenv("POLICY_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("POLICY_TEST_DATABASE_URL is not configured")
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

func seedRLSPolicy(
	ctx context.Context,
	t *testing.T,
	database *platformpostgres.Database,
	tenantID string,
	principalID string,
) {
	t.Helper()

	transaction, err := database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			tenantID,
			principalID,
			"seed-policy",
			"seed-policy",
		),
		pgx.TxOptions{},
	)
	if err != nil {
		t.Fatalf("begin seed policy transaction: %v", err)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO policy_sets (
				tenant_id,
				policy_id,
				scope_type,
				scope_id,
				name,
				description,
				status,
				resource_version,
				created_by_user_id,
				created_at,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				'tenant',
				NULL,
				'rls policy',
				'',
				'draft',
				1,
				$3::uuid,
				clock_timestamp(),
				clock_timestamp()
			)
		`,
		tenantID,
		uuid.NewString(),
		principalID,
	)
	if err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		t.Fatalf("commit seed policy: %v", err)
	}

	t.Cleanup(func() {
		cleanupScopedTables(ctx, t, database, tenantID, principalID)
	})
}

func cleanupScopedTables(
	ctx context.Context,
	t *testing.T,
	database *platformpostgres.Database,
	tenantID string,
	principalID string,
) {
	t.Helper()

	transaction, err := database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			tenantID,
			principalID,
			"cleanup-policy",
			"cleanup-policy",
		),
		pgx.TxOptions{},
	)
	if err != nil {
		return
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM policy_bootstrap_requests
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM policy_decisions
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM policy_rules
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM policy_versions
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)

	_, _ = transaction.Exec(
		ctx,
		`
			DELETE FROM policy_sets
			WHERE tenant_id = $1::uuid
		`,
		tenantID,
	)

	_ = transaction.Commit(ctx)
}

func TestPolicyRLSIsolation(t *testing.T) {
	database := policyRLSTestDatabase(t)

	ctx := context.Background()

	servicePrincipal := uuid.NewString()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()

	seedRLSPolicy(ctx, t, database, tenantA, servicePrincipal)
	seedRLSPolicy(ctx, t, database, tenantB, servicePrincipal)

	transaction, err := database.Begin(
		ctx,
		platformpostgres.TenantScope(
			tenantA,
			servicePrincipal,
			"rls-policy-a",
			"rls-policy-a",
		),
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		t.Fatalf("begin scope A: %v", err)
	}

	var crossTenantID string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT policy_id::text
			FROM policy_sets
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
		t.Fatalf("commit scope A: %v", err)
	}

	// Unscoped pooled queries must see nothing (RLS default-deny).
	var visibleCount int

	err = database.Pool().QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM policy_sets
		`,
	).Scan(&visibleCount)
	if err != nil {
		t.Fatalf("count unscoped policies: %v", err)
	}

	if visibleCount != 0 {
		t.Fatalf(
			"unscoped policy count = %d, want 0",
			visibleCount,
		)
	}
}

func TestPolicyRLSCannotUpdateAnotherTenant(t *testing.T) {
	database := policyRLSTestDatabase(t)

	ctx := context.Background()

	servicePrincipal := uuid.NewString()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()

	seedRLSPolicy(ctx, t, database, tenantA, servicePrincipal)
	seedRLSPolicy(ctx, t, database, tenantB, servicePrincipal)

	transaction, err := database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			tenantA,
			servicePrincipal,
			"rls-policy-update",
			"rls-policy-update",
		),
		pgx.TxOptions{},
	)
	if err != nil {
		t.Fatalf("begin scoped transaction: %v", err)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	commandTag, err := transaction.Exec(
		ctx,
		`
			UPDATE policy_sets
			SET name = 'pwned'
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

func TestPolicyRLSRejectsWrongTenantInsert(t *testing.T) {
	database := policyRLSTestDatabase(t)

	ctx := context.Background()

	scopeTenantID := uuid.NewString()
	insertTenantID := uuid.NewString()
	servicePrincipal := uuid.NewString()

	transaction, err := database.Begin(
		ctx,
		platformpostgres.ServiceTenantScope(
			scopeTenantID,
			servicePrincipal,
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
			INSERT INTO policy_sets (
				tenant_id,
				policy_id,
				scope_type,
				scope_id,
				name,
				description,
				status,
				resource_version,
				created_by_user_id,
				created_at,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				'tenant',
				NULL,
				'wrong tenant',
				'',
				'draft',
				1,
				$3::uuid,
				clock_timestamp(),
				clock_timestamp()
			)
		`,
		insertTenantID,
		uuid.NewString(),
		servicePrincipal,
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
