package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/jackc/pgx/v5"
)

const bindingColumns = `
	tenant_id::text,
	agent_id::text,
	company_id::text,
	status,
	primary_offering_id::text,
	fast_offering_id::text,
	fallback_policy,
	max_model_cost_microusd,
	version,
	created_by_user_id::text,
	updated_by_user_id::text,
	created_at,
	updated_at,
	removed_at
`

type bindingSnapshot struct {
	TenantID             string     `json:"tenantId"`
	AgentID              string     `json:"agentId"`
	CompanyID            string     `json:"companyId"`
	Status               string     `json:"status"`
	PrimaryOfferingID    string     `json:"primaryOfferingId"`
	FastOfferingID       *string    `json:"fastOfferingId,omitempty"`
	FallbackOfferingIDs  []string   `json:"fallbackOfferingIds"`
	FallbackPolicy       string     `json:"fallbackPolicy"`
	MaxModelCostMicroUSD *int64     `json:"maxModelCostMicrousd,omitempty"`
	Version              int64      `json:"version"`
	CreatedByUserID      string     `json:"createdByUserId"`
	UpdatedByUserID      string     `json:"updatedByUserId"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	RemovedAt            *time.Time `json:"removedAt,omitempty"`
}

func snapshotBinding(value domain.AgentModelBinding) bindingSnapshot {
	return bindingSnapshot{
		TenantID:             value.TenantID,
		AgentID:              value.AgentID,
		CompanyID:            value.CompanyID,
		Status:               string(value.Status),
		PrimaryOfferingID:    value.PrimaryOfferingID,
		FastOfferingID:       value.FastOfferingID,
		FallbackOfferingIDs:  append([]string(nil), value.FallbackOfferingIDs...),
		FallbackPolicy:       string(value.FallbackPolicy),
		MaxModelCostMicroUSD: value.MaxModelCostMicroUSD,
		Version:              value.Version,
		CreatedByUserID:      value.CreatedByUserID,
		UpdatedByUserID:      value.UpdatedByUserID,
		CreatedAt:            value.CreatedAt,
		UpdatedAt:            value.UpdatedAt,
		RemovedAt:            value.RemovedAt,
	}
}

func (value bindingSnapshot) domainBinding() domain.AgentModelBinding {
	return domain.AgentModelBinding{
		TenantID:             value.TenantID,
		AgentID:              value.AgentID,
		CompanyID:            value.CompanyID,
		Status:               domain.BindingStatus(value.Status),
		PrimaryOfferingID:    value.PrimaryOfferingID,
		FastOfferingID:       value.FastOfferingID,
		FallbackOfferingIDs:  append([]string(nil), value.FallbackOfferingIDs...),
		FallbackPolicy:       domain.FallbackPolicy(value.FallbackPolicy),
		MaxModelCostMicroUSD: value.MaxModelCostMicroUSD,
		Version:              value.Version,
		CreatedByUserID:      value.CreatedByUserID,
		UpdatedByUserID:      value.UpdatedByUserID,
		CreatedAt:            value.CreatedAt,
		UpdatedAt:            value.UpdatedAt,
		RemovedAt:            value.RemovedAt,
	}
}

func lookupBindingIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	actorUserID string,
	operation string,
	idempotencyKey string,
	requestHash []byte,
) (domain.AgentModelBinding, bool, error) {
	var (
		existingHash []byte
		rawSnapshot  []byte
	)

	err := tx.QueryRow(
		ctx,
		`
			SELECT
				request_hash,
				response_snapshot
			FROM
				model_access_binding_idempotency
			WHERE tenant_id = $1::uuid
			  AND actor_user_id = $2::uuid
			  AND operation = $3
			  AND idempotency_key = $4::uuid
		`,
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
	).Scan(
		&existingHash,
		&rawSnapshot,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentModelBinding{}, false, nil
	}

	if err != nil {
		return domain.AgentModelBinding{}, false, mapDatabaseError(err)
	}

	if !bytes.Equal(existingHash, requestHash) {
		return domain.AgentModelBinding{}, false, domain.ErrIdempotencyConflict
	}

	var snapshot bindingSnapshot
	if err := json.Unmarshal(rawSnapshot, &snapshot); err != nil {
		return domain.AgentModelBinding{}, false, fmt.Errorf("decode binding idempotency snapshot: %w", err)
	}

	return snapshot.domainBinding(), true, nil
}

func insertBindingIdempotency(
	ctx context.Context,
	tx pgx.Tx,
	tenantID string,
	actorUserID string,
	operation string,
	idempotencyKey string,
	requestHash []byte,
	value domain.AgentModelBinding,
	now time.Time,
	expiresAt time.Time,
) error {
	snapshot, err := json.Marshal(snapshotBinding(value))
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		ctx,
		`
			INSERT INTO model_access_binding_idempotency (
				tenant_id,
				actor_user_id,
				operation,
				idempotency_key,
				request_hash,
				response_snapshot,
				created_at,
				expires_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				$5,
				$6::jsonb,
				$7,
				$8
			)
		`,
		tenantID,
		actorUserID,
		operation,
		idempotencyKey,
		requestHash,
		snapshot,
		now,
		expiresAt,
	)

	return mapDatabaseError(err)
}

func scanBinding(row rowScanner) (domain.AgentModelBinding, error) {
	var result domain.AgentModelBinding
	var status string
	var fallbackPolicy string
	var fastOfferingID *string

	err := row.Scan(
		&result.TenantID,
		&result.AgentID,
		&result.CompanyID,
		&status,
		&result.PrimaryOfferingID,
		&fastOfferingID,
		&fallbackPolicy,
		&result.MaxModelCostMicroUSD,
		&result.Version,
		&result.CreatedByUserID,
		&result.UpdatedByUserID,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.RemovedAt,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	result.Status = domain.BindingStatus(status)
	result.FallbackPolicy = domain.FallbackPolicy(fallbackPolicy)
	result.FastOfferingID = fastOfferingID

	return result, nil
}

// GetAgentBinding retrieves the active binding and fallback list for an agent.
func (repository *Repository) GetAgentBinding(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (domain.AgentModelBinding, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	binding, err := scanBinding(
		transaction.QueryRow(
			ctx,
			`
				SELECT `+bindingColumns+`
				FROM model_access_agent_bindings
				WHERE tenant_id = $1::uuid
				  AND agent_id = $2::uuid
			`,
			tenantID,
			agentID,
		),
	)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.AgentModelBinding{}, domain.ErrBindingNotFound
		}
		return domain.AgentModelBinding{}, err
	}

	if binding.Status == domain.BindingStatusRemoved {
		return domain.AgentModelBinding{}, domain.ErrBindingNotFound
	}

	// Fetch fallbacks
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
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}
	defer rows.Close()

	fallbacks := make([]string, 0)
	for rows.Next() {
		var offeringID string
		if err := rows.Scan(&offeringID); err != nil {
			return domain.AgentModelBinding{}, err
		}
		fallbacks = append(fallbacks, offeringID)
	}
	if err := rows.Err(); err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	binding.FallbackOfferingIDs = fallbacks

	if err := commit(ctx, transaction); err != nil {
		return domain.AgentModelBinding{}, err
	}

	return binding, nil
}

// SetAgentBinding transactionally upserts an agent model binding, creates revisions, and records outbox events.
func (repository *Repository) SetAgentBinding(
	ctx context.Context,
	params ports.SetBindingParams,
) (domain.AgentModelBinding, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	// Verify idempotency
	cached, matched, err := lookupBindingIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		"set_agent_model_binding",
		params.IdempotencyKey,
		params.RequestHash,
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	if matched {
		return cached, nil
	}

	// Check existing binding for optimistic concurrency
	var existingVersion int64
	var currentStatus string
	row := transaction.QueryRow(
		ctx,
		`
			SELECT version, status
			FROM model_access_agent_bindings
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
			FOR UPDATE
		`,
		params.TenantID,
		params.AgentID,
	)
	err = row.Scan(&existingVersion, &currentStatus)
	exists := !errors.Is(err, pgx.ErrNoRows)

	if exists && err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	if !exists {
		if params.ExpectedVersion != 0 {
			return domain.AgentModelBinding{}, domain.ErrBindingVersionConflict
		}
	} else if params.ExpectedVersion != existingVersion {
		return domain.AgentModelBinding{}, domain.ErrBindingVersionConflict
	}

	var newVersion int64 = 1
	if exists {
		newVersion = existingVersion + 1
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_agent_bindings (
				tenant_id,
				agent_id,
				company_id,
				status,
				primary_offering_id,
				fast_offering_id,
				fallback_policy,
				max_model_cost_microusd,
				version,
				created_by_user_id,
				updated_by_user_id,
				created_at,
				updated_at,
				removed_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				'active',
				$4::uuid,
				$5::uuid,
				$6,
				$7,
				$8,
				$9::uuid,
				$9::uuid,
				$10,
				$10,
				NULL
			)
			ON CONFLICT (tenant_id, agent_id)
			DO UPDATE SET
				company_id = EXCLUDED.company_id,
				status = 'active',
				primary_offering_id = EXCLUDED.primary_offering_id,
				fast_offering_id = EXCLUDED.fast_offering_id,
				fallback_policy = EXCLUDED.fallback_policy,
				max_model_cost_microusd = EXCLUDED.max_model_cost_microusd,
				version = $8,
				updated_by_user_id = EXCLUDED.updated_by_user_id,
				updated_at = EXCLUDED.updated_at,
				removed_at = NULL
		`,
		params.TenantID,
		params.AgentID,
		params.CompanyID,
		params.PrimaryOfferingID,
		params.FastOfferingID,
		string(params.FallbackPolicy),
		params.MaxModelCostMicroUSD,
		newVersion,
		params.ActorUserID,
		params.Now,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	// Update fallbacks: delete existing, insert new with 0-based position
	_, err = transaction.Exec(
		ctx,
		`
			DELETE FROM model_access_agent_binding_fallbacks
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		params.TenantID,
		params.AgentID,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	for position, offeringID := range params.FallbackOfferingIDs {
		_, err = transaction.Exec(
			ctx,
			`
				INSERT INTO model_access_agent_binding_fallbacks (
					tenant_id,
					agent_id,
					position,
					offering_id
				)
				VALUES (
					$1::uuid,
					$2::uuid,
					$3,
					$4::uuid
				)
			`,
			params.TenantID,
			params.AgentID,
			position,
			offeringID,
		)
		if err != nil {
			return domain.AgentModelBinding{}, mapDatabaseError(err)
		}
	}

	changeKind := "created"
	if exists {
		if currentStatus == string(domain.BindingStatusRemoved) {
			changeKind = "reactivated"
		} else {
			changeKind = "updated"
		}
	}

	// Insert revision
	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_agent_binding_revisions (
				tenant_id,
				agent_id,
				revision,
				company_id,
				status,
				primary_offering_id,
				fast_offering_id,
				fallback_offering_ids,
				fallback_policy,
				max_model_cost_microusd,
				change_kind,
				changed_by_user_id,
				created_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				'active',
				$5::uuid,
				$6::uuid,
				$7,
				$8,
				$9,
				$10,
				$11::uuid,
				$12
			)
		`,
		params.TenantID,
		params.AgentID,
		newVersion,
		params.CompanyID,
		params.PrimaryOfferingID,
		params.FastOfferingID,
		params.FallbackOfferingIDs,
		string(params.FallbackPolicy),
		params.MaxModelCostMicroUSD,
		changeKind,
		params.ActorUserID,
		params.Now,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	binding := domain.AgentModelBinding{
		TenantID:             params.TenantID,
		AgentID:              params.AgentID,
		CompanyID:            params.CompanyID,
		Status:               domain.BindingStatusActive,
		PrimaryOfferingID:    params.PrimaryOfferingID,
		FastOfferingID:       params.FastOfferingID,
		FallbackOfferingIDs:  params.FallbackOfferingIDs,
		FallbackPolicy:       params.FallbackPolicy,
		MaxModelCostMicroUSD: params.MaxModelCostMicroUSD,
		Version:              newVersion,
		CreatedByUserID:      params.ActorUserID,
		UpdatedByUserID:      params.ActorUserID,
		CreatedAt:            params.Now,
		UpdatedAt:            params.Now,
	}

	if err := insertBindingIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		"set_agent_model_binding",
		params.IdempotencyKey,
		params.RequestHash,
		binding,
		params.Now,
		params.IdempotencyExpiresAt,
	); err != nil {
		return domain.AgentModelBinding{}, err
	}

	if params.EventFactory != nil {
		event, err := params.EventFactory(binding)
		if err != nil {
			return domain.AgentModelBinding{}, err
		}
		if err := insertOutbox(ctx, transaction, params.TenantID, event); err != nil {
			return domain.AgentModelBinding{}, err
		}
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.AgentModelBinding{}, err
	}

	return binding, nil
}

// RemoveAgentBinding removes an agent model binding and records an outbox event.
func (repository *Repository) RemoveAgentBinding(
	ctx context.Context,
	params ports.RemoveBindingParams,
) (domain.AgentModelBinding, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	// Verify idempotency
	cached, matched, err := lookupBindingIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		"remove_agent_model_binding",
		params.IdempotencyKey,
		params.RequestHash,
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	if matched {
		return cached, nil
	}

	// Fetch current binding
	binding, err := scanBinding(
		transaction.QueryRow(
			ctx,
			`
				SELECT `+bindingColumns+`
				FROM model_access_agent_bindings
				WHERE tenant_id = $1::uuid
				  AND agent_id = $2::uuid
				FOR UPDATE
			`,
			params.TenantID,
			params.AgentID,
		),
	)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.AgentModelBinding{}, domain.ErrBindingNotFound
		}
		return domain.AgentModelBinding{}, err
	}

	if binding.Status == domain.BindingStatusRemoved {
		return domain.AgentModelBinding{}, domain.ErrBindingNotFound
	}

	if params.ExpectedVersion != binding.Version {
		return domain.AgentModelBinding{}, domain.ErrBindingVersionConflict
	}

	newVersion := binding.Version + 1

	_, err = transaction.Exec(
		ctx,
		`
			UPDATE model_access_agent_bindings
			SET
				status = 'removed',
				version = $3,
				updated_by_user_id = $4::uuid,
				updated_at = $5,
				removed_at = $5
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		params.TenantID,
		params.AgentID,
		newVersion,
		params.ActorUserID,
		params.Now,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	// Delete fallbacks
	_, err = transaction.Exec(
		ctx,
		`
			DELETE FROM model_access_agent_binding_fallbacks
			WHERE tenant_id = $1::uuid
			  AND agent_id = $2::uuid
		`,
		params.TenantID,
		params.AgentID,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	// Insert revision
	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO model_access_agent_binding_revisions (
				tenant_id,
				agent_id,
				revision,
				company_id,
				status,
				primary_offering_id,
				fast_offering_id,
				fallback_offering_ids,
				fallback_policy,
				max_model_cost_microusd,
				change_kind,
				changed_by_user_id,
				created_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				'removed',
				$5::uuid,
				$6::uuid,
				$7,
				$8,
				$9,
				'removed',
				$10::uuid,
				$11
			)
		`,
		params.TenantID,
		params.AgentID,
		newVersion,
		binding.CompanyID,
		binding.PrimaryOfferingID,
		binding.FastOfferingID,
		binding.FallbackOfferingIDs,
		string(binding.FallbackPolicy),
		binding.MaxModelCostMicroUSD,
		params.ActorUserID,
		params.Now,
	)
	if err != nil {
		return domain.AgentModelBinding{}, mapDatabaseError(err)
	}

	binding.Status = domain.BindingStatusRemoved
	binding.Version = newVersion
	binding.UpdatedByUserID = params.ActorUserID
	binding.UpdatedAt = params.Now
	binding.RemovedAt = &params.Now

	if err := insertBindingIdempotency(
		ctx,
		transaction,
		params.TenantID,
		params.ActorUserID,
		"remove_agent_model_binding",
		params.IdempotencyKey,
		params.RequestHash,
		binding,
		params.Now,
		params.IdempotencyExpiresAt,
	); err != nil {
		return domain.AgentModelBinding{}, err
	}

	if params.EventFactory != nil {
		event, err := params.EventFactory(binding)
		if err != nil {
			return domain.AgentModelBinding{}, err
		}
		if err := insertOutbox(ctx, transaction, params.TenantID, event); err != nil {
			return domain.AgentModelBinding{}, err
		}
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.AgentModelBinding{}, err
	}

	return binding, nil
}
