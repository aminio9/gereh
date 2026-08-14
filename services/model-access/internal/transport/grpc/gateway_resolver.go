package grpc

import (
	"context"
	"time"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GatewayResolverServer implements ModelGatewayResolverService.
type GatewayResolverServer struct {
	modelgatewayv1.UnimplementedModelGatewayResolverServiceServer
	repository ports.Repository
}

// NewGatewayResolverServer creates a new internal Model Gateway resolver server.
func NewGatewayResolverServer(repository ports.Repository) *GatewayResolverServer {
	return &GatewayResolverServer{
		repository: repository,
	}
}

// ResolveInferencePlan looks up the active agent-model binding and routes for inference.
func (server *GatewayResolverServer) ResolveInferencePlan(
	ctx context.Context,
	request *modelgatewayv1.ResolveInferencePlanRequest,
) (*modelgatewayv1.ResolveInferencePlanResponse, error) {
	if request.GetTenantId() == "" || request.GetAgentId() == "" {
		return nil, mapError(domain.ErrInvalidArgument)
	}

	plan, err := server.repository.ResolveInferencePlan(
		ctx,
		request.GetTenantId(),
		request.GetAgentId(),
		time.Now().UTC(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &modelgatewayv1.ResolveInferencePlanResponse{
		Plan: toProtoInferencePlan(plan),
	}, nil
}

func toProtoInferencePlan(plan domain.InferencePlan) *modelgatewayv1.InferencePlan {
	res := &modelgatewayv1.InferencePlan{
		TenantId:             plan.TenantID,
		AgentId:              plan.AgentID,
		CompanyId:            plan.CompanyID,
		BindingVersion:       plan.BindingVersion,
		PrimaryRoute:         toProtoInferenceRoute(plan.PrimaryRoute),
		MaxModelCostMicrousd: plan.MaxModelCostMicroUSD,
		ResolvedAt:           timestamppb.New(plan.ResolvedAt),
	}

	if plan.FastRoute != nil {
		res.FastRoute = toProtoInferenceRoute(*plan.FastRoute)
	}

	for _, route := range plan.FallbackRoutes {
		res.FallbackRoutes = append(res.FallbackRoutes, toProtoInferenceRoute(route))
	}

	return res
}

func toProtoInferenceRoute(route domain.InferenceRoute) *modelgatewayv1.InferenceRoute {
	return &modelgatewayv1.InferenceRoute{
		OfferingId:          route.OfferingID,
		ConnectionId:        route.ConnectionID,
		ProviderKey:         route.ProviderKey,
		ProviderModelId:     route.ProviderModelID,
		ConnectionType:      route.ConnectionType,
		ProviderPoolKey:     route.ProviderPoolKey,
		ContextWindowTokens: route.ContextWindowTokens,
		MaxOutputTokens:     route.MaxOutputTokens,
		Capabilities:        route.Capabilities,
	}
}
