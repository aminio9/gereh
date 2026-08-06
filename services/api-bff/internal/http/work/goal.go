package work

import (
	"net/http"
	"strconv"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
)

func (handler *Handler) registerGoalRoutes(
	tenantRouter chi.Router,
	authHandler *authhttp.Handler,
) {
	tenantRouter.Route(
		"/goals",
		func(goalRouter chi.Router) {
			goalRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_GOAL_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.createGoal,
			)

			goalRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_WORK_READ,
				),
			).Get(
				"/",
				handler.listGoals,
			)

			goalRouter.Route(
				"/{goalID}",
				func(resource chi.Router) {
					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_WORK_READ,
						),
					).Get(
						"/",
						handler.getGoal,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_GOAL_UPDATE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/",
						handler.updateGoal,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_GOAL_UPDATE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/status",
						handler.changeGoalStatus,
					)
				},
			)
		},
	)
}

type createGoalRequest struct {
	CompanyID   string `json:"companyId"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateGoalRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	Title           *string `json:"title"`
	Description     *string `json:"description"`
}

type changeStatusRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
	Status          int32 `json:"status"`
}

func (handler *Handler) createGoal(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := principal(request)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	var input createGoalRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid goal request",
		)
		return
	}

	response, err := handler.client.CreateGoal(
		request.Context(),
		&workv1.CreateGoalRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   input.CompanyID,
			Title:       input.Title,
			Description: input.Description,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(
		writer,
		http.StatusCreated,
		response,
	)
}

func (handler *Handler) getGoal(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetGoal(
		request.Context(),
		&workv1.GetGoalRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			GoalId: chi.URLParam(
				request,
				"goalID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listGoals(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListGoals(
		request.Context(),
		&workv1.ListGoalsRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: request.URL.Query().
				Get("company_id"),
			PageSize: int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
			IncludeArchived: request.URL.Query().
				Get("include_archived") == "true",
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateGoal(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateGoalRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid goal request",
		)
		return
	}

	response, err := handler.client.UpdateGoal(
		request.Context(),
		&workv1.UpdateGoalRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			GoalId:          chi.URLParam(request, "goalID"),
			ExpectedVersion: input.ExpectedVersion,
			Title:           input.Title,
			Description:     input.Description,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) changeGoalStatus(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input changeStatusRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid goal status request",
		)
		return
	}

	response, err := handler.client.ChangeGoalStatus(
		request.Context(),
		&workv1.ChangeGoalStatusRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			GoalId:          chi.URLParam(request, "goalID"),
			ExpectedVersion: input.ExpectedVersion,
			Status: workv1.GoalStatus(
				input.Status,
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
