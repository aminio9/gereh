// Package postgres provides tenant-aware pgx pool and transaction helpers.
//
// Tenant-owned business queries must run through Scope-bound transactions so
// that transaction-local app.* settings drive PostgreSQL RLS. Service-internal
// operational tables may access the underlying pool directly.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const applyScopeSQL = `
SELECT
    set_config('app.scope_kind', $1, true),
    set_config('app.tenant_id', $2, true),
    set_config('app.principal_id', $3, true),
    set_config('app.principal_type', $4, true),
    set_config('app.request_id', $5, true),
    set_config('app.correlation_id', $6, true)
`

// Config defines a production application pool.
type Config struct {
	URL             string
	ApplicationName string

	MaxConnections int32
	MinConnections int32

	MaxConnectionLifetime time.Duration
	MaxConnectionIdleTime time.Duration
	HealthCheckPeriod     time.Duration

	StatementTimeout         time.Duration
	LockTimeout              time.Duration
	IdleInTransactionTimeout time.Duration

	// Runtime application roles must not own tables in these schemas.
	OwnedTableSchemas []string
}

// Database wraps pgxpool with tenant-aware transaction handling.
type Database struct {
	pool *pgxpool.Pool
}

// Open creates and validates a runtime application pool.
func Open(
	ctx context.Context,
	config Config,
) (*Database, error) {
	if strings.TrimSpace(config.URL) == "" {
		return nil, errors.New("PostgreSQL URL is required")
	}

	if strings.TrimSpace(config.ApplicationName) == "" {
		return nil, errors.New(
			"PostgreSQL application name is required",
		)
	}

	if config.MaxConnections <= 0 {
		return nil, errors.New(
			"PostgreSQL max connections must be positive",
		)
	}

	if config.MinConnections < 0 ||
		config.MinConnections > config.MaxConnections {
		return nil, errors.New(
			"invalid PostgreSQL minimum connection count",
		)
	}

	poolConfig, err := pgxpool.ParseConfig(config.URL)
	if err != nil {
		return nil, fmt.Errorf(
			"parse PostgreSQL URL: %w",
			err,
		)
	}

	poolConfig.MaxConns = config.MaxConnections
	poolConfig.MinConns = config.MinConnections
	poolConfig.MaxConnLifetime = defaultDuration(
		config.MaxConnectionLifetime,
		30*time.Minute,
	)
	poolConfig.MaxConnIdleTime = defaultDuration(
		config.MaxConnectionIdleTime,
		5*time.Minute,
	)
	poolConfig.HealthCheckPeriod = defaultDuration(
		config.HealthCheckPeriod,
		30*time.Second,
	)

	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams =
			make(map[string]string)
	}

	poolConfig.ConnConfig.RuntimeParams["application_name"] =
		config.ApplicationName

	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolConfig.ConnConfig.RuntimeParams["row_security"] = "on"

	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] =
		milliseconds(
			defaultDuration(
				config.StatementTimeout,
				15*time.Second,
			),
		)

	poolConfig.ConnConfig.RuntimeParams["lock_timeout"] =
		milliseconds(
			defaultDuration(
				config.LockTimeout,
				3*time.Second,
			),
		)

	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = milliseconds(
		defaultDuration(
			config.IdleInTransactionTimeout,
			15*time.Second,
		),
	)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf(
			"create PostgreSQL pool: %w",
			err,
		)
	}

	database := Wrap(pool)

	if err := database.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	if err := database.VerifyRuntimeRole(
		ctx,
		config.OwnedTableSchemas,
	); err != nil {
		pool.Close()
		return nil, err
	}

	return database, nil
}

// Wrap adds transaction helpers to an existing pool.
//
// Open should be used by service binaries. Wrap exists for tests and gradual
// migration of existing repository constructors.
func Wrap(pool *pgxpool.Pool) *Database {
	return &Database{pool: pool}
}

// Pool exposes the underlying pool for service-internal operational tables,
// such as the transactional outbox. Tenant-owned business-table access must
// use Begin.
func (database *Database) Pool() *pgxpool.Pool {
	return database.pool
}

// Close releases the underlying connection pool.
func (database *Database) Close() {
	database.pool.Close()
}

// Ping verifies the database is reachable.
func (database *Database) Ping(ctx context.Context) error {
	if err := database.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return nil
}

// Begin starts an explicit transaction and sets its RLS context before any
// service query is executed.
func (database *Database) Begin(
	ctx context.Context,
	scope Scope,
	options pgx.TxOptions,
) (pgx.Tx, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	transaction, err := database.pool.BeginTx(
		ctx,
		options,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"begin PostgreSQL transaction: %w",
			err,
		)
	}

	if err := ApplyScope(
		ctx,
		transaction,
		scope,
	); err != nil {
		_ = transaction.Rollback(
			context.WithoutCancel(ctx),
		)

		return nil, err
	}

	return transaction, nil
}

// ApplyScope changes the current transaction's local scope.
//
// Tenant onboarding initially uses principal scope for idempotency discovery,
// then changes to the newly allocated tenant before inserting tenant rows.
func ApplyScope(
	ctx context.Context,
	transaction pgx.Tx,
	scope Scope,
) error {
	if err := scope.Validate(); err != nil {
		return err
	}

	var returnedValues [6]string

	err := transaction.QueryRow(
		ctx,
		applyScopeSQL,
		string(scope.Kind),
		scope.TenantID,
		scope.PrincipalID,
		string(scope.PrincipalType),
		scope.RequestID,
		scope.CorrelationID,
	).Scan(
		&returnedValues[0],
		&returnedValues[1],
		&returnedValues[2],
		&returnedValues[3],
		&returnedValues[4],
		&returnedValues[5],
	)
	if err != nil {
		return fmt.Errorf(
			"apply PostgreSQL security scope: %w",
			err,
		)
	}

	return nil
}

// VerifyRuntimeRole prevents startup with a role that can silently defeat RLS.
func (database *Database) VerifyRuntimeRole(
	ctx context.Context,
	ownedTableSchemas []string,
) error {
	var roleName string
	var superuser bool
	var bypassRLS bool

	err := database.pool.QueryRow(
		ctx,
		`
			SELECT
				rolname,
				rolsuper,
				rolbypassrls
			FROM pg_roles
			WHERE rolname = current_user
		`,
	).Scan(
		&roleName,
		&superuser,
		&bypassRLS,
	)
	if err != nil {
		return fmt.Errorf(
			"inspect PostgreSQL runtime role: %w",
			err,
		)
	}

	if superuser || bypassRLS {
		return fmt.Errorf(
			"unsafe PostgreSQL runtime role %q: "+
				"superuser=%t bypassrls=%t",
			roleName,
			superuser,
			bypassRLS,
		)
	}

	if len(ownedTableSchemas) == 0 {
		return nil
	}

	var ownedTables string

	err = database.pool.QueryRow(
		ctx,
		`
			SELECT COALESCE(
				string_agg(
					format(
						'%I.%I',
						namespace.nspname,
						relation.relname
					),
					', '
				),
				''
			)
			FROM pg_class AS relation
			JOIN pg_namespace AS namespace
				ON namespace.oid = relation.relnamespace
			WHERE relation.relkind IN ('r', 'p')
			  AND namespace.nspname = ANY($1::text[])
			  AND pg_get_userbyid(relation.relowner) =
				current_user
		`,
		ownedTableSchemas,
	).Scan(&ownedTables)
	if err != nil {
		return fmt.Errorf(
			"inspect PostgreSQL table ownership: %w",
			err,
		)
	}

	if ownedTables != "" {
		return fmt.Errorf(
			"unsafe PostgreSQL runtime role %q "+
				"owns protected tables: %s",
			roleName,
			ownedTables,
		)
	}

	return nil
}

func defaultDuration(
	value time.Duration,
	fallback time.Duration,
) time.Duration {
	if value <= 0 {
		return fallback
	}

	return value
}

func milliseconds(value time.Duration) string {
	return strconv.FormatInt(
		value.Milliseconds(),
		10,
	)
}
