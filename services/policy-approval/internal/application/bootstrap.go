package application

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
)

// EnsureDefaultPoliciesInput carries the default-policy bootstrap request.
type EnsureDefaultPoliciesInput struct {
	TenantID              string
	OnboardingOperationID string
	ActorUserID           string
}

// EnsureDefaultPolicies idempotently creates the tenant default policy set.
func (service *Service) EnsureDefaultPolicies(
	ctx context.Context,
	input EnsureDefaultPoliciesInput,
) ([]domain.Policy, error) {
	if err := validateBootstrap(input); err != nil {
		return nil, err
	}

	now := service.now().UTC()

	policies, versions, err := defaultPolicies(
		input.TenantID,
		input.ActorUserID,
		now,
	)
	if err != nil {
		return nil, err
	}

	events := make([]domain.OutboxEvent, 0, len(policies))

	for index, policy := range policies {
		event, eventErr := service.newPolicyCreatedEvent(
			ctx,
			policy,
			now,
		)
		if eventErr != nil {
			return nil, eventErr
		}

		events = append(events, event)

		activatedAt := now
		versions[index].ActivatedAt = &activatedAt

		activationEvent, activationErr :=
			service.newPolicyActivatedEvent(
				ctx,
				policy.TenantID,
				policy.ID,
				1,
				now,
			)
		if activationErr != nil {
			return nil, activationErr
		}

		events = append(events, activationEvent)
	}

	return service.repository.EnsureDefaults(
		ctx,
		ports.EnsureDefaultsParams{
			ServicePrincipalID:    service.config.BootstrapServicePrincipalID,
			TenantID:              input.TenantID,
			OnboardingOperationID: input.OnboardingOperationID,
			ActorUserID:           input.ActorUserID,
			Policies:              policies,
			Versions:              versions,
			Events:                events,
			CreatedAt:             now,
		},
	)
}

func validateBootstrap(
	input EnsureDefaultPoliciesInput,
) error {
	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if err := validateUUID(
		"onboarding_operation_id",
		input.OnboardingOperationID,
	); err != nil {
		return err
	}

	return validateUUID("actor_user_id", input.ActorUserID)
}

// defaultPolicies returns the three tenant-scoped default policies.
func defaultPolicies(
	tenantID string,
	actorUserID string,
	now time.Time,
) ([]domain.Policy, []domain.PolicyVersion, error) {
	securityPolicy, securityVersion, err := newDefaultPolicy(
		tenantID,
		actorUserID,
		now,
		"Platform security boundary",
		"Non-overridable protection for sensitive platform resources.",
		[]domain.Rule{
			{
				Priority: 10,
				Name:     "Deny secret administration",
				Enabled:  true,
				Effect:   domain.EffectDeny,

				ActionPatterns: []string{
					"secret.admin.*",
					"vault.admin.*",
					"kubernetes.admin.*",
					"cloud.metadata.*",
					"tenant.billing.admin.*",
				},

				Condition: "true",
				Reason:    "Platform security boundaries cannot be modified by agents",
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	approvalPolicy, approvalVersion, err := newDefaultPolicy(
		tenantID,
		actorUserID,
		now,
		"Sensitive actions",
		"Requires human approval for externally visible or destructive actions.",
		[]domain.Rule{
			{
				Priority: 20,
				Name:     "Require approval for deployments",
				Enabled:  true,
				Effect:   domain.EffectRequireApproval,

				ActionPatterns: []string{
					"deploy.*",
					"github.pull_request.merge",
					"email.send",
					"slack.post",
					"financial.*",
					"secret.read",
				},

				Condition: "true",

				Constraints: domain.Constraints{
					RequireHumanReview: true,
				},

				Reason: "Sensitive external or destructive action requires approval",
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	readPolicy, readVersion, err := newDefaultPolicy(
		tenantID,
		actorUserID,
		now,
		"Safe read actions",
		"Allows low-risk, non-mutating reads.",
		[]domain.Rule{
			{
				Priority: 100,
				Name:     "Allow low-risk reads",
				Enabled:  true,
				Effect:   domain.EffectAllow,

				ActionPatterns: []string{
					"read.*",
					"research.*",
					"observe.*",
				},

				RiskLevels: []domain.Risk{
					domain.RiskLow,
				},

				Condition: "request.estimated_cost_micro_usd <= 100000",

				Reason: "Low-risk read action is within the default cost limit",
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return []domain.Policy{
			securityPolicy,
			approvalPolicy,
			readPolicy,
		}, []domain.PolicyVersion{
			securityVersion,
			approvalVersion,
			readVersion,
		}, nil
}

func newDefaultPolicy(
	tenantID string,
	actorUserID string,
	now time.Time,
	name string,
	description string,
	rules []domain.Rule,
) (domain.Policy, domain.PolicyVersion, error) {
	policyID, err := newUUID()
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	activeVersion := int64(1)

	return domain.Policy{
			TenantID:            tenantID,
			ID:                  policyID,
			ScopeType:           domain.ScopeTenant,
			ScopeID:             nil,
			Name:                name,
			Description:         description,
			Status:              domain.PolicyStatusActive,
			ActivePolicyVersion: &activeVersion,
			ResourceVersion:     1,
			CreatedByUserID:     actorUserID,
			CreatedAt:           now,
			UpdatedAt:           now,
		}, domain.PolicyVersion{
			TenantID:        tenantID,
			PolicyID:        policyID,
			PolicyVersion:   1,
			DefaultEffect:   domain.EffectDeny,
			Rules:           rules,
			Notes:           "Bootstrap default policy",
			CreatedByUserID: actorUserID,
			CreatedAt:       now,
			ActivatedAt:     &now,
		}, nil
}
