package policy

import (
	"net/http"
	"strconv"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/go-chi/chi/v5"
)

func (handler *Handler) registerDecisionRoutes(
	tenantRouter chi.Router,
) {
	tenantRouter.Route(
		"/decisions",
		func(decisionRouter chi.Router) {
			decisionRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_POLICY_DECISION_READ,
				),
			).Get(
				"/",
				handler.listDecisions,
			)

			decisionRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_POLICY_DECISION_READ,
				),
			).Get(
				"/{decisionID}",
				handler.getDecision,
			)
		},
	)
}

func (handler *Handler) getDecision(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetDecision(
		request.Context(),
		&policyv1.GetDecisionRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			DecisionId: chi.URLParam(
				request,
				"decisionID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listDecisions(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListDecisions(
		request.Context(),
		&policyv1.ListDecisionsRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			PageSize:    int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
