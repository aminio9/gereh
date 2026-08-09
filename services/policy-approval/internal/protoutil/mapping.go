// Package protoutil maps policy domain values to Protobuf messages.
package protoutil

import (
	"time"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Policy maps a domain policy to the Protobuf message.
func Policy(value domain.Policy) *policyv1.Policy {
	return &policyv1.Policy{
		TenantId:            value.TenantID,
		PolicyId:            value.ID,
		Scope:               Scope(value.ScopeType, value.ScopeID),
		Name:                value.Name,
		Description:         value.Description,
		Status:              PolicyStatus(value.Status),
		ActivePolicyVersion: activeVersion(value.ActivePolicyVersion),
		ResourceVersion:     value.ResourceVersion,
		CreatedByUserId:     value.CreatedByUserID,
		CreatedAt:           protoTimestamp(value.CreatedAt),
		UpdatedAt:           protoTimestamp(value.UpdatedAt),
		ArchivedAt:          protoTimestampPointer(value.ArchivedAt),
	}
}

// Scope maps a scope type and ID to the Protobuf message.
func Scope(
	scopeType domain.ScopeType,
	scopeID *string,
) *policyv1.PolicyScope {
	return &policyv1.PolicyScope{
		Type:    ScopeType(scopeType),
		ScopeId: optionalString(scopeID),
	}
}

// PolicyVersion maps a domain policy version to the Protobuf message.
func PolicyVersion(value domain.PolicyVersion) *policyv1.PolicyVersion {
	rules := make([]*policyv1.PolicyRule, 0, len(value.Rules))
	for _, rule := range value.Rules {
		rules = append(rules, Rule(rule))
	}

	return &policyv1.PolicyVersion{
		TenantId:        value.TenantID,
		PolicyId:        value.PolicyID,
		PolicyVersion:   value.PolicyVersion,
		DefaultEffect:   Effect(value.DefaultEffect),
		Rules:           rules,
		Notes:           value.Notes,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       protoTimestamp(value.CreatedAt),
		ActivatedAt:     protoTimestampPointer(value.ActivatedAt),
	}
}

// Rule maps a domain rule to the Protobuf message.
func Rule(value domain.Rule) *policyv1.PolicyRule {
	riskLevels := make([]policyv1.RiskLevel, 0, len(value.RiskLevels))
	for _, risk := range value.RiskLevels {
		riskLevels = append(riskLevels, Risk(risk))
	}

	return &policyv1.PolicyRule{
		RuleId:         value.ID,
		Priority:       value.Priority,
		Name:           value.Name,
		Enabled:        value.Enabled,
		Effect:         Effect(value.Effect),
		ActionPatterns: value.ActionPatterns,
		ResourceTypes:  value.ResourceTypes,
		RiskLevels:     riskLevels,
		MaximumEstimatedCostMicroUsd: optionalInt64(
			value.MaximumEstimatedCostMicroUSD,
		),
		Condition: value.Condition,
		Constraints: structFromConstraints(
			value.Constraints,
		),
		Reason: value.Reason,
	}
}

// Constraints maps a domain constraint set to the Protobuf message.
func Constraints(value domain.Constraints) *policyv1.PolicyConstraints {
	return &policyv1.PolicyConstraints{
		MaxCostMicroUsd:    optionalInt64(value.MaxCostMicroUSD),
		MaxRuntimeSeconds:  optionalInt64(value.MaxRuntimeSeconds),
		AllowedDomains:     value.AllowedDomains,
		AllowedResourceIds: value.AllowedResourceIDs,
		RequireHumanReview: value.RequireHumanReview,
	}
}

// Subject maps a domain subject to the Protobuf message.
func Subject(value domain.Subject) *policyv1.PolicySubject {
	return &policyv1.PolicySubject{
		Type:      SubjectType(value.Type),
		SubjectId: value.ID,
		CompanyId: optionalString(value.CompanyID),
	}
}

// Resource maps a domain resource to the Protobuf message.
func Resource(value domain.Resource) *policyv1.PolicyResource {
	return &policyv1.PolicyResource{
		Type:       value.Type,
		ResourceId: value.ID,
		Attributes: structFromMap(value.Attributes),
	}
}

// Decision maps a domain decision to the Protobuf message.
func Decision(value domain.Decision) *policyv1.PolicyDecision {
	return &policyv1.PolicyDecision{
		DecisionId:            value.ID,
		EvaluationRequestId:   value.RequestID,
		TenantId:              value.TenantID,
		Subject:               Subject(value.Subject),
		Action:                value.Action,
		Resource:              Resource(value.Resource),
		Risk:                  Risk(value.Risk),
		EstimatedCostMicroUsd: value.EstimatedCostMicroUSD,
		Effect:                Effect(value.Effect),
		Constraints:           Constraints(value.Constraints),
		Reason:                value.Reason,
		MatchedPolicyId:       optionalString(value.MatchedPolicyID),
		MatchedPolicyVersion:  optionalInt64(value.MatchedPolicyVersion),
		MatchedRuleId:         optionalString(value.MatchedRuleID),
		DecidedAt:             protoTimestamp(value.DecidedAt),
		ExpiresAt:             protoTimestamp(value.ExpiresAt),
		SigningKeyId:          value.SigningKeyID,
		Signature:             value.Signature,
	}
}

// EvaluationInput maps a domain evaluation input to the request message used
// for deterministic idempotency hashing.
func EvaluationInput(
	value domain.EvaluationInput,
) *policyv1.EvaluateRequest {
	return &policyv1.EvaluateRequest{
		EvaluationRequestId:   value.RequestID,
		TenantId:              value.TenantID,
		Subject:               Subject(value.Subject),
		Action:                value.Action,
		Resource:              Resource(value.Resource),
		Risk:                  Risk(value.Risk),
		EstimatedCostMicroUsd: value.EstimatedCostMicroUSD,
		Context:               structFromMap(value.Context),
	}
}

func activeVersion(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}

func structFromMap(value map[string]any) *structpb.Struct {
	converted, err := structpb.NewStruct(value)
	if err != nil {
		converted, _ = structpb.NewStruct(nil)
	}

	return converted
}

func structFromConstraints(
	value domain.Constraints,
) *structpb.Struct {
	fields := map[string]any{}

	if value.MaxCostMicroUSD != nil {
		fields["max_cost_micro_usd"] = *value.MaxCostMicroUSD
	}

	if value.MaxRuntimeSeconds != nil {
		fields["max_runtime_seconds"] = *value.MaxRuntimeSeconds
	}

	if len(value.AllowedDomains) > 0 {
		fields["allowed_domains"] = value.AllowedDomains
	}

	if len(value.AllowedResourceIDs) > 0 {
		fields["allowed_resource_ids"] = value.AllowedResourceIDs
	}

	if value.RequireHumanReview {
		fields["require_human_review"] = true
	}

	return structFromMap(fields)
}

func protoTimestamp(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value)
}

func protoTimestampPointer(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}

	return timestamppb.New(*value)
}

func optionalString(value *string) *string {
	if value == nil {
		return nil
	}

	copyValue := *value
	return &copyValue
}

func optionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}

	copyValue := *value
	return &copyValue
}
