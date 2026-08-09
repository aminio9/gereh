package grpc

import (
	"context"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/application"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/engine"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
)

// ManagementServer implements PolicyManagementService.
type ManagementServer struct {
	policyv1.UnimplementedPolicyManagementServiceServer

	service   *application.Service
	celEngine *engine.CEL
}

// NewManagement creates the Policy Management Service gRPC transport.
func NewManagement(
	service *application.Service,
	celEngine *engine.CEL,
) *ManagementServer {
	return &ManagementServer{
		service:   service,
		celEngine: celEngine,
	}
}

// CreatePolicy creates a policy set.
func (server *ManagementServer) CreatePolicy(
	ctx context.Context,
	request *policyv1.CreatePolicyRequest,
) (*policyv1.CreatePolicyResponse, error) {
	var scopeID *string
	if request.GetScope() != nil {
		scopeID = request.GetScope().ScopeId
	}

	scopeType, err := protoutil.DomainScopeType(
		request.GetScope().GetType(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	policy, err := server.service.CreatePolicy(
		ctx,
		application.CreatePolicyInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			ScopeType:   scopeType,
			ScopeID:     scopeID,
			Name:        request.GetName(),
			Description: request.GetDescription(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.CreatePolicyResponse{
		Policy: protoutil.Policy(policy),
	}, nil
}

// GetPolicy returns a policy set with its active version.
func (server *ManagementServer) GetPolicy(
	ctx context.Context,
	request *policyv1.GetPolicyRequest,
) (*policyv1.GetPolicyResponse, error) {
	policy, version, err := server.service.GetPolicy(
		ctx,
		application.GetPolicyInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			PolicyID:    request.GetPolicyId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	response := &policyv1.GetPolicyResponse{
		Policy: protoutil.Policy(policy),
	}

	if version != nil {
		response.ActiveVersion = protoutil.PolicyVersion(*version)
	}

	return response, nil
}

// ListPolicies lists policy sets.
func (server *ManagementServer) ListPolicies(
	ctx context.Context,
	request *policyv1.ListPoliciesRequest,
) (*policyv1.ListPoliciesResponse, error) {
	cursor, err := decodePolicyToken(request.GetPageToken())
	if err != nil {
		return nil, mapError(err)
	}

	policies, err := server.service.ListPolicies(
		ctx,
		application.ListPoliciesInput{
			ActorUserID:     request.GetActorUserId(),
			TenantID:        request.GetTenantId(),
			Limit:           normalizePageSize(request.GetPageSize()),
			Cursor:          cursor,
			IncludeArchived: request.GetIncludeArchived(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*policyv1.Policy, 0, len(policies))

	for _, policy := range policies {
		items = append(items, protoutil.Policy(policy))
	}

	response := &policyv1.ListPoliciesResponse{
		Policies: items,
	}

	if len(policies) > 0 {
		last := policies[len(policies)-1]

		response.NextPageToken = encodePolicyToken(
			&ports.PolicyCursor{
				PolicyID: last.ID,
			},
		)
	}

	return response, nil
}

// CreatePolicyVersion creates a new immutable policy version.
func (server *ManagementServer) CreatePolicyVersion(
	ctx context.Context,
	request *policyv1.CreatePolicyVersionRequest,
) (*policyv1.CreatePolicyVersionResponse, error) {
	defaultEffect, err := protoutil.DomainEffect(
		request.GetDefaultEffect(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	rules := make([]domain.Rule, 0, len(request.GetRules()))

	for _, value := range request.GetRules() {
		rule, err := application.RuleFromProto(
			value,
			server.celEngine,
		)
		if err != nil {
			return nil, mapError(err)
		}

		rules = append(rules, rule)
	}

	policy, version, err := server.service.CreateVersion(
		ctx,
		application.CreateVersionInput{
			ActorUserID:             request.GetActorUserId(),
			TenantID:                request.GetTenantId(),
			PolicyID:                request.GetPolicyId(),
			ExpectedResourceVersion: request.GetExpectedResourceVersion(),
			DefaultEffect:           defaultEffect,
			Rules:                   rules,
			Notes:                   request.GetNotes(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.CreatePolicyVersionResponse{
		Policy:  protoutil.Policy(policy),
		Version: protoutil.PolicyVersion(version),
	}, nil
}

// ActivatePolicy activates an existing policy version.
func (server *ManagementServer) ActivatePolicy(
	ctx context.Context,
	request *policyv1.ActivatePolicyRequest,
) (*policyv1.ActivatePolicyResponse, error) {
	policy, version, err := server.service.ActivatePolicy(
		ctx,
		application.ActivatePolicyInput{
			ActorUserID:             request.GetActorUserId(),
			TenantID:                request.GetTenantId(),
			PolicyID:                request.GetPolicyId(),
			ExpectedResourceVersion: request.GetExpectedResourceVersion(),
			PolicyVersion:           request.GetPolicyVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.ActivatePolicyResponse{
		Policy:        protoutil.Policy(policy),
		ActiveVersion: protoutil.PolicyVersion(version),
	}, nil
}

// ArchivePolicy archives a policy set.
func (server *ManagementServer) ArchivePolicy(
	ctx context.Context,
	request *policyv1.ArchivePolicyRequest,
) (*policyv1.ArchivePolicyResponse, error) {
	policy, err := server.service.ArchivePolicy(
		ctx,
		application.ArchivePolicyInput{
			ActorUserID:             request.GetActorUserId(),
			TenantID:                request.GetTenantId(),
			PolicyID:                request.GetPolicyId(),
			ExpectedResourceVersion: request.GetExpectedResourceVersion(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.ArchivePolicyResponse{
		Policy: protoutil.Policy(policy),
	}, nil
}

// GetDecision returns a signed decision.
func (server *ManagementServer) GetDecision(
	ctx context.Context,
	request *policyv1.GetDecisionRequest,
) (*policyv1.GetDecisionResponse, error) {
	decision, err := server.service.GetDecision(
		ctx,
		application.GetDecisionInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			DecisionID:  request.GetDecisionId(),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.GetDecisionResponse{
		Decision: protoutil.Decision(decision),
	}, nil
}

// ListDecisions lists signed decisions.
func (server *ManagementServer) ListDecisions(
	ctx context.Context,
	request *policyv1.ListDecisionsRequest,
) (*policyv1.ListDecisionsResponse, error) {
	cursor, err := decodeDecisionToken(request.GetPageToken())
	if err != nil {
		return nil, mapError(err)
	}

	var subjectID *string
	if request.SubjectId != nil {
		subjectID = request.SubjectId
	}

	decisions, err := server.service.ListDecisions(
		ctx,
		application.ListDecisionsInput{
			ActorUserID: request.GetActorUserId(),
			TenantID:    request.GetTenantId(),
			SubjectID:   subjectID,
			Limit:       normalizePageSize(request.GetPageSize()),
			Cursor:      cursor,
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	items := make([]*policyv1.PolicyDecision, 0, len(decisions))

	for _, decision := range decisions {
		items = append(items, protoutil.Decision(decision))
	}

	response := &policyv1.ListDecisionsResponse{
		Decisions: items,
	}

	if len(decisions) > 0 {
		last := decisions[len(decisions)-1]

		response.NextPageToken = encodeDecisionToken(
			&ports.DecisionCursor{
				DecidedAt:  last.DecidedAt,
				DecisionID: last.ID,
			},
		)
	}

	return response, nil
}
