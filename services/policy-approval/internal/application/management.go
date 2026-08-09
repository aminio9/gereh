package application

import (
	"context"
	"fmt"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/engine"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
)

// CreatePolicyInput carries a new policy set.
type CreatePolicyInput struct {
	ActorUserID string
	TenantID    string

	ScopeType   domain.ScopeType
	ScopeID     *string
	Name        string
	Description string
}

// GetPolicyInput identifies one policy set.
type GetPolicyInput struct {
	ActorUserID string
	TenantID    string
	PolicyID    string
}

// ListPoliciesInput paginates policy sets.
type ListPoliciesInput struct {
	ActorUserID     string
	TenantID        string
	Limit           int
	Cursor          *ports.PolicyCursor
	IncludeArchived bool
}

// CreateVersionInput carries a new immutable version.
type CreateVersionInput struct {
	ActorUserID string
	TenantID    string
	PolicyID    string

	ExpectedResourceVersion int64
	DefaultEffect           domain.Effect
	Rules                   []domain.Rule
	Notes                   string
}

// ActivatePolicyInput activates an existing version.
type ActivatePolicyInput struct {
	ActorUserID string
	TenantID    string
	PolicyID    string

	ExpectedResourceVersion int64
	PolicyVersion           int64
}

// ArchivePolicyInput archives a policy set.
type ArchivePolicyInput struct {
	ActorUserID string
	TenantID    string
	PolicyID    string

	ExpectedResourceVersion int64
}

// GetDecisionInput identifies one decision.
type GetDecisionInput struct {
	ActorUserID string
	TenantID    string
	DecisionID  string
}

// ListDecisionsInput paginates decision history.
type ListDecisionsInput struct {
	ActorUserID string
	TenantID    string
	SubjectID   *string
	Limit       int
	Cursor      *ports.DecisionCursor
}

// CreatePolicy creates a new policy set.
func (service *Service) CreatePolicy(
	ctx context.Context,
	input CreatePolicyInput,
) (domain.Policy, error) {
	if err := validateCreatePolicy(input); err != nil {
		return domain.Policy{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_CREATE,
	); err != nil {
		return domain.Policy{}, err
	}

	policyID, err := newUUID()
	if err != nil {
		return domain.Policy{}, err
	}

	now := service.now().UTC()

	policy := domain.Policy{
		TenantID:        input.TenantID,
		ID:              policyID,
		ScopeType:       input.ScopeType,
		ScopeID:         input.ScopeID,
		Name:            input.Name,
		Description:     input.Description,
		Status:          domain.PolicyStatusDraft,
		ResourceVersion: 1,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	event, err := service.newPolicyCreatedEvent(
		ctx,
		policy,
		now,
	)
	if err != nil {
		return domain.Policy{}, err
	}

	return service.repository.CreatePolicy(
		ctx,
		ports.CreatePolicyParams{
			ActorUserID: input.ActorUserID,
			Policy:      policy,
			Event:       event,
		},
	)
}

// GetPolicy returns one policy set with its active version.
func (service *Service) GetPolicy(
	ctx context.Context,
	input GetPolicyInput,
) (domain.Policy, *domain.PolicyVersion, error) {
	if err := validateGetPolicy(input); err != nil {
		return domain.Policy{}, nil, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_READ,
	); err != nil {
		return domain.Policy{}, nil, err
	}

	return service.repository.GetPolicy(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.PolicyID,
	)
}

// ListPolicies lists the tenant policy sets.
func (service *Service) ListPolicies(
	ctx context.Context,
	input ListPoliciesInput,
) ([]domain.Policy, error) {
	if err := validateListPolicies(input); err != nil {
		return nil, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListPolicies(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.Limit,
		input.Cursor,
		input.IncludeArchived,
	)
}

// CreateVersion creates a new immutable policy version.
func (service *Service) CreateVersion(
	ctx context.Context,
	input CreateVersionInput,
) (domain.Policy, domain.PolicyVersion, error) {
	if err := validateCreateVersion(input); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_UPDATE,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	now := service.now().UTC()

	version := domain.PolicyVersion{
		TenantID:        input.TenantID,
		PolicyID:        input.PolicyID,
		PolicyVersion:   0,
		DefaultEffect:   input.DefaultEffect,
		Rules:           input.Rules,
		Notes:           input.Notes,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
	}

	event, err := service.newPolicyVersionCreatedEvent(
		ctx,
		input.TenantID,
		input.PolicyID,
		version,
		now,
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	return service.repository.CreateVersion(
		ctx,
		ports.CreateVersionParams{
			ActorUserID:             input.ActorUserID,
			Policy:                  domain.Policy{TenantID: input.TenantID, ID: input.PolicyID},
			Version:                 version,
			ExpectedResourceVersion: input.ExpectedResourceVersion,
			Event:                   event,
		},
	)
}

// ActivatePolicy activates an existing policy version.
func (service *Service) ActivatePolicy(
	ctx context.Context,
	input ActivatePolicyInput,
) (domain.Policy, domain.PolicyVersion, error) {
	if err := validateActivatePolicy(input); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_ACTIVATE,
	); err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	event, err := service.newPolicyActivatedEvent(
		ctx,
		input.TenantID,
		input.PolicyID,
		input.PolicyVersion,
		service.now().UTC(),
	)
	if err != nil {
		return domain.Policy{}, domain.PolicyVersion{}, err
	}

	return service.repository.ActivatePolicy(
		ctx,
		ports.ActivatePolicyParams{
			ActorUserID:             input.ActorUserID,
			TenantID:                input.TenantID,
			PolicyID:                input.PolicyID,
			PolicyVersion:           input.PolicyVersion,
			ExpectedResourceVersion: input.ExpectedResourceVersion,
			ActivatedAt:             service.now().UTC(),
			Event:                   event,
		},
	)
}

// ArchivePolicy archives a policy set.
func (service *Service) ArchivePolicy(
	ctx context.Context,
	input ArchivePolicyInput,
) (domain.Policy, error) {
	if err := validateArchivePolicy(input); err != nil {
		return domain.Policy{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_ARCHIVE,
	); err != nil {
		return domain.Policy{}, err
	}

	event, err := service.newPolicyArchivedEvent(
		ctx,
		input.TenantID,
		input.PolicyID,
		input.ActorUserID,
		service.now().UTC(),
	)
	if err != nil {
		return domain.Policy{}, err
	}

	return service.repository.ArchivePolicy(
		ctx,
		ports.ArchivePolicyParams{
			ActorUserID:             input.ActorUserID,
			TenantID:                input.TenantID,
			PolicyID:                input.PolicyID,
			ExpectedResourceVersion: input.ExpectedResourceVersion,
			ArchivedAt:              service.now().UTC(),
			Event:                   event,
		},
	)
}

// GetDecision returns one signed decision.
func (service *Service) GetDecision(
	ctx context.Context,
	input GetDecisionInput,
) (domain.Decision, error) {
	if err := validateGetDecision(input); err != nil {
		return domain.Decision{}, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_DECISION_READ,
	); err != nil {
		return domain.Decision{}, err
	}

	return service.repository.GetDecision(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.DecisionID,
	)
}

// ListDecisions lists signed decisions.
func (service *Service) ListDecisions(
	ctx context.Context,
	input ListDecisionsInput,
) ([]domain.Decision, error) {
	if err := validateListDecisions(input); err != nil {
		return nil, err
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_POLICY_DECISION_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListDecisions(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.SubjectID,
		input.Limit,
		input.Cursor,
	)
}

func validateCreatePolicy(
	input CreatePolicyInput,
) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	switch input.ScopeType {
	case domain.ScopeTenant:
		if input.ScopeID != nil {
			return fmt.Errorf(
				"%w: tenant scope must not carry a scope ID",
				domain.ErrInvalidArgument,
			)
		}

	case domain.ScopeCompany, domain.ScopeAgent:
		if input.ScopeID == nil {
			return fmt.Errorf(
				"%w: %s scope requires a scope ID",
				domain.ErrInvalidArgument,
				input.ScopeType,
			)
		}

		if err := validateUUID("scope_id", *input.ScopeID); err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"%w: unsupported policy scope type",
			domain.ErrInvalidArgument,
		)
	}

	if err := boundedText("name", input.Name, 120); err != nil {
		return err
	}

	return optionalBoundedText("description", input.Description, 4000)
}

func validateGetPolicy(input GetPolicyInput) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	return validateUUID("policy_id", input.PolicyID)
}

func validateListPolicies(input ListPoliciesInput) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if input.Limit < 1 || input.Limit > 100 {
		return fmt.Errorf(
			"%w: limit must be between 1 and 100",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func validateCreateVersion(
	input CreateVersionInput,
) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if err := validateUUID("policy_id", input.PolicyID); err != nil {
		return err
	}

	if input.ExpectedResourceVersion < 1 {
		return fmt.Errorf(
			"%w: expected_resource_version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	if input.DefaultEffect != domain.EffectDeny &&
		input.DefaultEffect != domain.EffectRequireApproval {
		return fmt.Errorf(
			"%w: default effect must be deny or require_approval",
			domain.ErrInvalidArgument,
		)
	}

	return optionalBoundedText("notes", input.Notes, 4000)
}

func validateActivatePolicy(
	input ActivatePolicyInput,
) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if err := validateUUID("policy_id", input.PolicyID); err != nil {
		return err
	}

	if input.ExpectedResourceVersion < 1 {
		return fmt.Errorf(
			"%w: expected_resource_version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	if input.PolicyVersion < 1 {
		return fmt.Errorf(
			"%w: policy_version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func validateArchivePolicy(
	input ArchivePolicyInput,
) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if err := validateUUID("policy_id", input.PolicyID); err != nil {
		return err
	}

	if input.ExpectedResourceVersion < 1 {
		return fmt.Errorf(
			"%w: expected_resource_version must be positive",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func validateGetDecision(input GetDecisionInput) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	return validateUUID("decision_id", input.DecisionID)
}

func validateListDecisions(input ListDecisionsInput) error {
	if err := validateUUID("actor_user_id", input.ActorUserID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if input.SubjectID != nil {
		if err := validateUUID("subject_id", *input.SubjectID); err != nil {
			return err
		}
	}

	if input.Limit < 1 || input.Limit > 100 {
		return fmt.Errorf(
			"%w: limit must be between 1 and 100",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

// RuleFromProto converts a request rule into a domain rule.
func RuleFromProto(
	value *policyv1.PolicyRule,
	celEngine *engine.CEL,
) (domain.Rule, error) {
	if value == nil {
		return domain.Rule{}, fmt.Errorf(
			"%w: policy rule is required",
			domain.ErrInvalidArgument,
		)
	}

	ruleID, err := newUUID()
	if err != nil {
		return domain.Rule{}, err
	}

	if value.GetRuleId() != "" {
		ruleID = value.GetRuleId()
	}

	effect, err := protoutil.DomainEffect(value.GetEffect())
	if err != nil {
		return domain.Rule{}, fmt.Errorf(
			"%w: rule %s has an invalid effect",
			domain.ErrInvalidArgument,
			ruleID,
		)
	}

	if value.GetPriority() < 0 || value.GetPriority() > 100000 {
		return domain.Rule{}, fmt.Errorf(
			"%w: rule priority out of range",
			domain.ErrInvalidArgument,
		)
	}

	if err := boundedText("rule_name", value.GetName(), 120); err != nil {
		return domain.Rule{}, err
	}

	if len(value.GetActionPatterns()) == 0 {
		return domain.Rule{}, fmt.Errorf(
			"%w: rule requires at least one action pattern",
			domain.ErrInvalidArgument,
		)
	}

	for _, pattern := range value.GetActionPatterns() {
		if err := engine.ValidateActionPattern(pattern); err != nil {
			return domain.Rule{}, err
		}
	}

	condition := value.GetCondition()
	if condition == "" {
		condition = "true"
	}

	if err := celEngine.Validate(condition); err != nil {
		return domain.Rule{}, err
	}

	constraints, err := domainConstraints(value.GetConstraints())
	if err != nil {
		return domain.Rule{}, err
	}

	reason := value.GetReason()
	if reason == "" {
		reason = "Rule applied"
	}

	if err := boundedText("rule_reason", reason, 1000); err != nil {
		return domain.Rule{}, err
	}

	riskLevels := make([]domain.Risk, 0, len(value.GetRiskLevels()))
	for _, risk := range value.GetRiskLevels() {
		converted, err := protoutil.DomainRisk(risk)
		if err != nil {
			return domain.Rule{}, err
		}

		riskLevels = append(riskLevels, converted)
	}

	maximumCost := value.MaximumEstimatedCostMicroUsd

	return domain.Rule{
		ID:       ruleID,
		Priority: value.GetPriority(),
		Name:     value.GetName(),
		Enabled:  value.GetEnabled(),
		Effect:   effect,

		ActionPatterns: value.GetActionPatterns(),
		ResourceTypes:  value.GetResourceTypes(),
		RiskLevels:     riskLevels,

		MaximumEstimatedCostMicroUSD: maximumCost,

		Condition:   condition,
		Constraints: constraints,
		Reason:      reason,
	}, nil
}
