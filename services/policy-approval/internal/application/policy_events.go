package application

import (
	"context"
	"time"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
)

func (service *Service) newPolicyCreatedEvent(
	ctx context.Context,
	policy domain.Policy,
	now time.Time,
) (domain.OutboxEvent, error) {
	return newOutboxEvent(
		ctx,
		service.config.EventTopic,
		policy.ID,
		"policy.created",
		policy.TenantID,
		"policy",
		policy.ID,
		1,
		&policyv1.PolicyCreated{
			Policy: protoutil.Policy(policy),
		},
		now,
	)
}

func (service *Service) newPolicyVersionCreatedEvent(
	ctx context.Context,
	tenantID string,
	policyID string,
	version domain.PolicyVersion,
	now time.Time,
) (domain.OutboxEvent, error) {
	return newOutboxEvent(
		ctx,
		service.config.EventTopic,
		policyID,
		"policy.version_created",
		tenantID,
		"policy",
		policyID,
		version.PolicyVersion,
		&policyv1.PolicyVersionCreated{
			Policy: &policyv1.Policy{
				TenantId: tenantID,
				PolicyId: policyID,
			},
			Version: protoutil.PolicyVersion(version),
		},
		now,
	)
}

func (service *Service) newPolicyActivatedEvent(
	ctx context.Context,
	tenantID string,
	policyID string,
	policyVersion int64,
	now time.Time,
) (domain.OutboxEvent, error) {
	version := domain.PolicyVersion{
		TenantID:      tenantID,
		PolicyID:      policyID,
		PolicyVersion: policyVersion,
	}

	return newOutboxEvent(
		ctx,
		service.config.EventTopic,
		policyID,
		"policy.activated",
		tenantID,
		"policy",
		policyID,
		policyVersion,
		&policyv1.PolicyActivated{
			Policy: &policyv1.Policy{
				TenantId: tenantID,
				PolicyId: policyID,
			},
			Version: protoutil.PolicyVersion(version),
		},
		now,
	)
}

func (service *Service) newPolicyArchivedEvent(
	ctx context.Context,
	tenantID string,
	policyID string,
	archivedByUserID string,
	now time.Time,
) (domain.OutboxEvent, error) {
	return newOutboxEvent(
		ctx,
		service.config.EventTopic,
		policyID,
		"policy.archived",
		tenantID,
		"policy",
		policyID,
		0,
		&policyv1.PolicyArchived{
			Policy: &policyv1.Policy{
				TenantId: tenantID,
				PolicyId: policyID,
			},
			ArchivedByUserId: archivedByUserID,
		},
		now,
	)
}

func (service *Service) newDecisionEvent(
	ctx context.Context,
	decision domain.Decision,
) (domain.OutboxEvent, error) {
	if decision.Subject.CompanyID != nil {
		return newOutboxEvent(
			ctx,
			service.config.EventTopic,
			*decision.Subject.CompanyID,
			"policy.decision_recorded",
			decision.TenantID,
			"policy_decision",
			decision.ID,
			0,
			&policyv1.PolicyDecisionRecorded{
				Decision: protoutil.Decision(decision),
			},
			decision.DecidedAt,
		)
	}

	return newOutboxEvent(
		ctx,
		service.config.EventTopic,
		decision.Subject.ID,
		"policy.decision_recorded",
		decision.TenantID,
		"policy_decision",
		decision.ID,
		0,
		&policyv1.PolicyDecisionRecorded{
			Decision: protoutil.Decision(decision),
		},
		decision.DecidedAt,
	)
}
