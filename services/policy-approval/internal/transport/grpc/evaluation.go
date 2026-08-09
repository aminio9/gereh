package grpc

import (
	"context"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/application"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
	"google.golang.org/protobuf/types/known/structpb"
)

// EvaluationServer implements PolicyEvaluationService.
type EvaluationServer struct {
	policyv1.UnimplementedPolicyEvaluationServiceServer

	service *application.Service
}

// NewEvaluation creates the Policy Evaluation Service gRPC transport.
func NewEvaluation(
	service *application.Service,
) *EvaluationServer {
	return &EvaluationServer{
		service: service,
	}
}

// Evaluate evaluates one action and returns a signed decision.
func (server *EvaluationServer) Evaluate(
	ctx context.Context,
	request *policyv1.EvaluateRequest,
) (*policyv1.EvaluateResponse, error) {
	if request.GetSubject() == nil {
		return nil, mapError(domain.ErrInvalidArgument)
	}

	if request.GetResource() == nil {
		return nil, mapError(domain.ErrInvalidArgument)
	}

	subjectType, err := protoutil.DomainSubjectType(
		request.GetSubject().GetType(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	risk, err := protoutil.DomainRisk(request.GetRisk())
	if err != nil {
		return nil, mapError(err)
	}

	decision, err := server.service.Evaluate(
		ctx,
		application.EvaluateInput{
			RequestID:     request.GetEvaluationRequestId(),
			TenantID:      request.GetTenantId(),
			CallerService: requestCallerService(ctx),
			Subject: domain.Subject{
				Type:      subjectType,
				ID:        request.GetSubject().GetSubjectId(),
				CompanyID: request.GetSubject().CompanyId,
			},
			Action: request.GetAction(),
			Resource: domain.Resource{
				Type:       request.GetResource().GetType(),
				ID:         request.GetResource().GetResourceId(),
				Attributes: structValueMap(request.GetResource().GetAttributes()),
			},
			Risk:                  risk,
			EstimatedCostMicroUSD: request.GetEstimatedCostMicroUsd(),
			Context:               structValueMap(request.GetContext()),
		},
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &policyv1.EvaluateResponse{
		Decision: protoutil.Decision(decision),
	}, nil
}

// requestCallerService returns the authenticated caller identity.
func requestCallerService(
	ctx context.Context,
) string {
	return callerService(ctx)
}

func structValueMap(
	value *structpb.Struct,
) map[string]any {
	if value == nil {
		return nil
	}

	return value.AsMap()
}
