package application

import (
	"context"
	"fmt"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/protoutil"
)

// SetAgentBindingInput specifies the parameters for creating or updating an agent model binding.
type SetAgentBindingInput struct {
	ActorUserID string
	TenantID    string

	AgentID string

	IdempotencyKey string

	ExpectedVersion int64

	PrimaryOfferingID   string
	FastOfferingID      *string
	FallbackOfferingIDs []string
	FallbackPolicy      domain.FallbackPolicy

	MaxModelCostMicroUSD *int64
}

// RemoveAgentBindingInput specifies the parameters for removing an agent model binding.
type RemoveAgentBindingInput struct {
	ActorUserID string
	TenantID    string

	AgentID string

	IdempotencyKey string

	ExpectedVersion int64
}

// GetAgentModelBinding retrieves the model binding assigned to an agent.
func (service *Service) GetAgentModelBinding(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (domain.AgentModelBinding, error) {
	if err := validateUUID("actor_user_id", actorUserID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("tenant_id", tenantID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("agent_id", agentID); err != nil {
		return domain.AgentModelBinding{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_MODEL_BINDING_READ,
	); err != nil {
		return domain.AgentModelBinding{}, err
	}

	return service.repository.GetAgentBinding(
		ctx,
		actorUserID,
		tenantID,
		agentID,
	)
}

// SetAgentModelBinding creates or updates the model offerings assigned to an agent.
func (service *Service) SetAgentModelBinding(
	ctx context.Context,
	input SetAgentBindingInput,
) (domain.AgentModelBinding, error) {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("agent_id", input.AgentID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("primary_offering_id", input.PrimaryOfferingID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if input.FastOfferingID != nil && *input.FastOfferingID != "" {
		if err := validateUUID("fast_offering_id", *input.FastOfferingID); err != nil {
			return domain.AgentModelBinding{}, err
		}
	}
	for _, fallbackID := range input.FallbackOfferingIDs {
		if err := validateUUID("fallback_offering_id", fallbackID); err != nil {
			return domain.AgentModelBinding{}, err
		}
	}
	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.AgentModelBinding{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_BINDING_UPDATE,
	); err != nil {
		return domain.AgentModelBinding{}, err
	}

	// 1. Validate agent exists in Organization service
	if service.agentDirectory == nil {
		return domain.AgentModelBinding{}, fmt.Errorf("agent directory is required")
	}

	agentRef, err := service.agentDirectory.GetAgent(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.AgentID,
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}

	if agentRef.Status == organizationv1.AgentStatus_AGENT_STATUS_DELETING ||
		agentRef.Status == organizationv1.AgentStatus_AGENT_STATUS_DELETED {
		return domain.AgentModelBinding{}, domain.ErrAgentDeleted
	}

	// 2. Validate primary offering exists, is available and agent_usable
	primaryOffering, err := service.repository.GetOffering(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.PrimaryOfferingID,
	)
	if err != nil {
		return domain.AgentModelBinding{}, err
	}
	if primaryOffering.Status != domain.OfferingStatusAvailable {
		return domain.AgentModelBinding{}, domain.ErrOfferingUnavailable
	}
	if !primaryOffering.AgentUsable {
		return domain.AgentModelBinding{}, domain.ErrOfferingNotAgentUsable
	}

	// 3. Validate fast offering if present
	if input.FastOfferingID != nil && *input.FastOfferingID != "" {
		fastOffering, err := service.repository.GetOffering(
			ctx,
			input.ActorUserID,
			input.TenantID,
			*input.FastOfferingID,
		)
		if err != nil {
			return domain.AgentModelBinding{}, err
		}
		if fastOffering.Status != domain.OfferingStatusAvailable {
			return domain.AgentModelBinding{}, domain.ErrOfferingUnavailable
		}
		if !fastOffering.AgentUsable {
			return domain.AgentModelBinding{}, domain.ErrOfferingNotAgentUsable
		}
	}

	// 4. Validate fallback offerings if present
	for _, fallbackID := range input.FallbackOfferingIDs {
		fallbackOffering, err := service.repository.GetOffering(
			ctx,
			input.ActorUserID,
			input.TenantID,
			fallbackID,
		)
		if err != nil {
			return domain.AgentModelBinding{}, err
		}
		if fallbackOffering.Status != domain.OfferingStatusAvailable {
			return domain.AgentModelBinding{}, domain.ErrOfferingUnavailable
		}
		if !fallbackOffering.AgentUsable {
			return domain.AgentModelBinding{}, domain.ErrOfferingNotAgentUsable
		}
	}

	if len(input.FallbackOfferingIDs) > 5 {
		return domain.AgentModelBinding{}, fmt.Errorf("%w: at most five fallback offerings are allowed", domain.ErrInvalidArgument)
	}

	seen := map[string]struct{}{
		input.PrimaryOfferingID: {},
	}

	if input.FastOfferingID != nil && *input.FastOfferingID != "" {
		if _, duplicate := seen[*input.FastOfferingID]; duplicate {
			return domain.AgentModelBinding{}, fmt.Errorf("%w: duplicate model offering", domain.ErrInvalidArgument)
		}
		seen[*input.FastOfferingID] = struct{}{}
	}

	for _, fallbackID := range input.FallbackOfferingIDs {
		if _, duplicate := seen[fallbackID]; duplicate {
			return domain.AgentModelBinding{}, fmt.Errorf("%w: duplicate model offering", domain.ErrInvalidArgument)
		}
		seen[fallbackID] = struct{}{}
	}

	switch input.FallbackPolicy {
	case domain.FallbackPolicyNone:
		if len(input.FallbackOfferingIDs) != 0 {
			return domain.AgentModelBinding{}, domain.ErrInvalidFallbackPolicy
		}
	case domain.FallbackPolicyOrdered:
		if len(input.FallbackOfferingIDs) == 0 {
			return domain.AgentModelBinding{}, domain.ErrInvalidFallbackPolicy
		}
	default:
		return domain.AgentModelBinding{}, domain.ErrInvalidFallbackPolicy
	}

	if input.MaxModelCostMicroUSD != nil && *input.MaxModelCostMicroUSD <= 0 {
		return domain.AgentModelBinding{}, domain.ErrInvalidArgument
	}

	fallbackPolicy := input.FallbackPolicy

	fastStr := ""
	if input.FastOfferingID != nil {
		fastStr = *input.FastOfferingID
	}
	costStr := ""
	if input.MaxModelCostMicroUSD != nil {
		costStr = fmt.Sprintf("%d", *input.MaxModelCostMicroUSD)
	}

	requestHash, err := hashCanonical(struct {
		AgentID              string   `json:"agentId"`
		ExpectedVersion      int64    `json:"expectedVersion"`
		PrimaryOfferingID    string   `json:"primaryOfferingId"`
		FastOfferingID       string   `json:"fastOfferingId"`
		FallbackOfferingIDs  []string `json:"fallbackOfferingIds"`
		FallbackPolicy       string   `json:"fallbackPolicy"`
		MaxModelCostMicroUSD string   `json:"maxModelCostMicroUSD"`
	}{
		AgentID:              input.AgentID,
		ExpectedVersion:      input.ExpectedVersion,
		PrimaryOfferingID:    input.PrimaryOfferingID,
		FastOfferingID:       fastStr,
		FallbackOfferingIDs:  input.FallbackOfferingIDs,
		FallbackPolicy:       string(fallbackPolicy),
		MaxModelCostMicroUSD: costStr,
	})
	if err != nil {
		return domain.AgentModelBinding{}, err
	}

	now := service.now().UTC()

	return service.repository.SetAgentBinding(
		ctx,
		ports.SetBindingParams{
			ActorUserID:          input.ActorUserID,
			TenantID:             input.TenantID,
			AgentID:              input.AgentID,
			CompanyID:            agentRef.CompanyID,
			ExpectedVersion:      input.ExpectedVersion,
			PrimaryOfferingID:    input.PrimaryOfferingID,
			FastOfferingID:       input.FastOfferingID,
			FallbackOfferingIDs:  input.FallbackOfferingIDs,
			FallbackPolicy:       fallbackPolicy,
			MaxModelCostMicroUSD: input.MaxModelCostMicroUSD,
			IdempotencyKey:       input.IdempotencyKey,
			RequestHash:          requestHash,
			IdempotencyExpiresAt: now.Add(service.config.IdempotencyTTL),
			Now:                  now,
			EventFactory: func(result domain.AgentModelBinding) (domain.OutboxEvent, error) {
				return service.bindingEvent(
					ctx,
					"model.agent_binding.set",
					result,
					&modelv1.AgentModelBindingSet{
						Binding: protoutil.AgentModelBinding(result),
					},
					now,
				)
			},
		},
	)
}

// RemoveAgentModelBinding marks an agent's model binding as removed.
func (service *Service) RemoveAgentModelBinding(
	ctx context.Context,
	input RemoveAgentBindingInput,
) (domain.AgentModelBinding, error) {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateUUID("agent_id", input.AgentID); err != nil {
		return domain.AgentModelBinding{}, err
	}
	if err := validateIdempotencyKey(input.IdempotencyKey); err != nil {
		return domain.AgentModelBinding{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_MODEL_BINDING_REMOVE,
	); err != nil {
		return domain.AgentModelBinding{}, err
	}

	requestHash, err := hashCanonical(struct {
		AgentID         string `json:"agentId"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}{
		AgentID:         input.AgentID,
		ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return domain.AgentModelBinding{}, err
	}

	now := service.now().UTC()

	return service.repository.RemoveAgentBinding(
		ctx,
		ports.RemoveBindingParams{
			ActorUserID:          input.ActorUserID,
			TenantID:             input.TenantID,
			AgentID:              input.AgentID,
			ExpectedVersion:      input.ExpectedVersion,
			IdempotencyKey:       input.IdempotencyKey,
			RequestHash:          requestHash,
			IdempotencyExpiresAt: now.Add(service.config.IdempotencyTTL),
			Now:                  now,
			EventFactory: func(result domain.AgentModelBinding) (domain.OutboxEvent, error) {
				return service.bindingEvent(
					ctx,
					"model.agent_binding.removed",
					result,
					&modelv1.AgentModelBindingRemoved{
						Binding:         protoutil.AgentModelBinding(result),
						RemovedByUserId: input.ActorUserID,
					},
					now,
				)
			},
		},
	)
}
