package protoutil

import (
	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// ScopeType maps a domain scope type to the Protobuf enum.
func ScopeType(value domain.ScopeType) policyv1.PolicyScopeType {
	switch value {
	case domain.ScopeTenant:
		return policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_TENANT

	case domain.ScopeCompany:
		return policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_COMPANY

	case domain.ScopeAgent:
		return policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_AGENT

	default:
		return policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_UNSPECIFIED
	}
}

// DomainScopeType maps a Protobuf scope type to the domain.
func DomainScopeType(
	value policyv1.PolicyScopeType,
) (domain.ScopeType, error) {
	switch value {
	case policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_TENANT:
		return domain.ScopeTenant, nil

	case policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_COMPANY:
		return domain.ScopeCompany, nil

	case policyv1.PolicyScopeType_POLICY_SCOPE_TYPE_AGENT:
		return domain.ScopeAgent, nil

	default:
		return "", domain.ErrInvalidArgument
	}
}

// PolicyStatus maps a domain policy status to the Protobuf enum.
func PolicyStatus(value domain.PolicyStatus) policyv1.PolicyStatus {
	switch value {
	case domain.PolicyStatusDraft:
		return policyv1.PolicyStatus_POLICY_STATUS_DRAFT

	case domain.PolicyStatusActive:
		return policyv1.PolicyStatus_POLICY_STATUS_ACTIVE

	case domain.PolicyStatusArchived:
		return policyv1.PolicyStatus_POLICY_STATUS_ARCHIVED

	default:
		return policyv1.PolicyStatus_POLICY_STATUS_UNSPECIFIED
	}
}

// Effect maps a domain effect to the Protobuf enum.
func Effect(value domain.Effect) policyv1.PolicyEffect {
	switch value {
	case domain.EffectAllow:
		return policyv1.PolicyEffect_POLICY_EFFECT_ALLOW

	case domain.EffectDeny:
		return policyv1.PolicyEffect_POLICY_EFFECT_DENY

	case domain.EffectRequireApproval:
		return policyv1.PolicyEffect_POLICY_EFFECT_REQUIRE_APPROVAL

	case domain.EffectAllowWithConstraints:
		return policyv1.PolicyEffect_POLICY_EFFECT_ALLOW_WITH_CONSTRAINTS

	default:
		return policyv1.PolicyEffect_POLICY_EFFECT_UNSPECIFIED
	}
}

// DomainEffect maps a Protobuf effect to the domain.
func DomainEffect(value policyv1.PolicyEffect) (domain.Effect, error) {
	switch value {
	case policyv1.PolicyEffect_POLICY_EFFECT_ALLOW:
		return domain.EffectAllow, nil

	case policyv1.PolicyEffect_POLICY_EFFECT_DENY:
		return domain.EffectDeny, nil

	case policyv1.PolicyEffect_POLICY_EFFECT_REQUIRE_APPROVAL:
		return domain.EffectRequireApproval, nil

	case policyv1.PolicyEffect_POLICY_EFFECT_ALLOW_WITH_CONSTRAINTS:
		return domain.EffectAllowWithConstraints, nil

	default:
		return "", domain.ErrInvalidArgument
	}
}

// Risk maps a domain risk to the Protobuf enum.
func Risk(value domain.Risk) policyv1.RiskLevel {
	switch value {
	case domain.RiskLow:
		return policyv1.RiskLevel_RISK_LEVEL_LOW

	case domain.RiskMedium:
		return policyv1.RiskLevel_RISK_LEVEL_MEDIUM

	case domain.RiskHigh:
		return policyv1.RiskLevel_RISK_LEVEL_HIGH

	case domain.RiskCritical:
		return policyv1.RiskLevel_RISK_LEVEL_CRITICAL

	default:
		return policyv1.RiskLevel_RISK_LEVEL_UNSPECIFIED
	}
}

// DomainRisk maps a Protobuf risk level to the domain.
func DomainRisk(value policyv1.RiskLevel) (domain.Risk, error) {
	switch value {
	case policyv1.RiskLevel_RISK_LEVEL_LOW:
		return domain.RiskLow, nil

	case policyv1.RiskLevel_RISK_LEVEL_MEDIUM:
		return domain.RiskMedium, nil

	case policyv1.RiskLevel_RISK_LEVEL_HIGH:
		return domain.RiskHigh, nil

	case policyv1.RiskLevel_RISK_LEVEL_CRITICAL:
		return domain.RiskCritical, nil

	default:
		return "", domain.ErrInvalidArgument
	}
}

// SubjectType maps a domain subject type to the Protobuf enum.
func SubjectType(value domain.SubjectType) policyv1.SubjectType {
	switch value {
	case domain.SubjectUser:
		return policyv1.SubjectType_SUBJECT_TYPE_USER

	case domain.SubjectAgent:
		return policyv1.SubjectType_SUBJECT_TYPE_AGENT

	case domain.SubjectService:
		return policyv1.SubjectType_SUBJECT_TYPE_SERVICE

	default:
		return policyv1.SubjectType_SUBJECT_TYPE_UNSPECIFIED
	}
}

// DomainSubjectType maps a Protobuf subject type to the domain.
func DomainSubjectType(
	value policyv1.SubjectType,
) (domain.SubjectType, error) {
	switch value {
	case policyv1.SubjectType_SUBJECT_TYPE_USER:
		return domain.SubjectUser, nil

	case policyv1.SubjectType_SUBJECT_TYPE_AGENT:
		return domain.SubjectAgent, nil

	case policyv1.SubjectType_SUBJECT_TYPE_SERVICE:
		return domain.SubjectService, nil

	default:
		return "", domain.ErrInvalidArgument
	}
}
