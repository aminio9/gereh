package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github.com/aminio9/gereh/platform/go/postgres"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ResolveInferencePlan queries the agent binding, primary/fast/fallback offerings, and connection metadata.
func (repository *Repository) ResolveInferencePlan(
	ctx context.Context,
	tenantID string,
	agentID string,
	now time.Time,
) (domain.InferencePlan, error) {
	// Service principal scope for internal workload resolution
	scope := platformpostgres.ServiceTenantScope(
		tenantID,
		uuid.Nil.String(),
		"internal-plan-resolve",
		"internal-plan-resolve",
	)

	transaction, err := repository.database.Begin(ctx, scope, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.InferencePlan{}, fmt.Errorf("begin internal transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var (
		companyID            string
		status               string
		primaryOfferingID    string
		fastOfferingID       *string
		fallbackPolicy       string
		maxModelCostMicroUSD *int64
		version              int64
	)

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				company_id::text,
				status,
				primary_offering_id::text,
				fast_offering_id::text,
				fallback_policy,
				max_model_cost_microusd,
				version
			FROM model_access_agent_bindings
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		tenantID,
		agentID,
	).Scan(
		&companyID,
		&status,
		&primaryOfferingID,
		&fastOfferingID,
		&fallbackPolicy,
		&maxModelCostMicroUSD,
		&version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.InferencePlan{}, domain.ErrBindingNotFound
		}
		return domain.InferencePlan{}, mapDatabaseError(err)
	}

	if status != string(domain.BindingStatusActive) {
		return domain.InferencePlan{}, domain.ErrBindingNotFound
	}

	// Fetch primary route
	primaryRoute, err := fetchRoute(ctx, transaction, tenantID, primaryOfferingID)
	if err != nil {
		return domain.InferencePlan{}, fmt.Errorf("fetch primary route: %w", err)
	}

	var fastRoute *domain.InferenceRoute
	if fastOfferingID != nil && *fastOfferingID != "" {
		route, err := fetchRoute(ctx, transaction, tenantID, *fastOfferingID)
		if err == nil {
			fastRoute = &route
		}
	}

	// Fetch fallback offerings if ordered
	var fallbackRoutes []domain.InferenceRoute
	if fallbackPolicy == string(domain.FallbackPolicyOrdered) {
		rows, err := transaction.Query(
			ctx,
			`
				SELECT offering_id::text
				FROM model_access_agent_binding_fallbacks
				WHERE tenant_id = $1::uuid
				  AND agent_id = $2::uuid
				ORDER BY position ASC
			`,
			tenantID,
			agentID,
		)
		if err != nil {
			return domain.InferencePlan{}, mapDatabaseError(err)
		}
		defer rows.Close()

		var fallbackOfferingIDs []string
		for rows.Next() {
			var offID string
			if err := rows.Scan(&offID); err != nil {
				return domain.InferencePlan{}, err
			}
			fallbackOfferingIDs = append(fallbackOfferingIDs, offID)
		}

		for _, offID := range fallbackOfferingIDs {
			route, err := fetchRoute(ctx, transaction, tenantID, offID)
			if err == nil {
				fallbackRoutes = append(fallbackRoutes, route)
			}
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.InferencePlan{}, err
	}

	return domain.InferencePlan{
		TenantID:             tenantID,
		AgentID:              agentID,
		CompanyID:            companyID,
		BindingVersion:       version,
		PrimaryRoute:         primaryRoute,
		FastRoute:            fastRoute,
		FallbackRoutes:       fallbackRoutes,
		MaxModelCostMicroUSD: maxModelCostMicroUSD,
		ResolvedAt:           now,
	}, nil
}

func fetchRoute(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	offeringID string,
) (domain.InferenceRoute, error) {
	var (
		route            domain.InferenceRoute
		connectionStatus string
		offeringStatus   string
		agentUsable      bool
	)

	err := tx.QueryRow(
		ctx,
		`
			SELECT
				o.offering_id::text,
				o.connection_id::text,
				o.provider_key,
				o.provider_model_id,
				o.status,
				o.agent_usable,
				o.capabilities,
				o.context_window_tokens,
				o.max_output_tokens,
				c.connection_type,
				c.provider_pool_key,
				c.status
			FROM model_access_model_offerings o
			JOIN model_access_connections c
			  ON o.tenant_id = c.tenant_id
			 AND o.connection_id = c.connection_id
			WHERE o.tenant_id = $1::uuid
			  AND o.offering_id = $2::uuid
		`,
		tenantID,
		offeringID,
	).Scan(
		&route.OfferingID,
		&route.ConnectionID,
		&route.ProviderKey,
		&route.ProviderModelID,
		&offeringStatus,
		&agentUsable,
		&route.Capabilities,
		&route.ContextWindowTokens,
		&route.MaxOutputTokens,
		&route.ConnectionType,
		&route.ProviderPoolKey,
		&connectionStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.InferenceRoute{}, domain.ErrOfferingNotFound
		}
		return domain.InferenceRoute{}, mapDatabaseError(err)
	}

	if offeringStatus != string(domain.OfferingStatusAvailable) || !agentUsable {
		return domain.InferenceRoute{}, domain.ErrOfferingNotFound
	}

	if connectionStatus != string(domain.ConnectionStatusActive) {
		return domain.InferenceRoute{}, domain.ErrConnectionArchived
	}

	return route, nil
}
