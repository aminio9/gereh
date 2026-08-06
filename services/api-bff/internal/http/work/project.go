package work

import (
	"net/http"
	"strconv"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
)

func (handler *Handler) registerProjectRoutes(
	tenantRouter chi.Router,
	authHandler *authhttp.Handler,
) {
	tenantRouter.Route(
		"/goals/{goalID}/projects",
		func(projectRouter chi.Router) {
			projectRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_PROJECT_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.createProject,
			)

			projectRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_WORK_READ,
				),
			).Get(
				"/",
				handler.listProjects,
			)
		},
	)

	tenantRouter.Route(
		"/projects/{projectID}",
		func(resource chi.Router) {
			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_WORK_READ,
				),
			).Get(
				"/",
				handler.getProject,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_PROJECT_UPDATE,
				),
				authHandler.RequireCSRF,
			).Patch(
				"/",
				handler.updateProject,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_PROJECT_UPDATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/status",
				handler.changeProjectStatus,
			)
		},
	)
}

type createProjectRequest struct {
	CompanyID   string `json:"companyId"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	Title           *string `json:"title"`
	Description     *string `json:"description"`
}

func (handler *Handler) createProject(
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

	var input createProjectRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid project request",
		)
		return
	}

	response, err := handler.client.CreateProject(
		request.Context(),
		&workv1.CreateProjectRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   input.CompanyID,
			GoalId:      chi.URLParam(request, "goalID"),
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

func (handler *Handler) getProject(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetProject(
		request.Context(),
		&workv1.GetProjectRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			ProjectId: chi.URLParam(
				request,
				"projectID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listProjects(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListProjects(
		request.Context(),
		&workv1.ListProjectsRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: request.URL.Query().
				Get("company_id"),
			GoalId: chi.URLParam(
				request,
				"goalID",
			),
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

func (handler *Handler) updateProject(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateProjectRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid project request",
		)
		return
	}

	response, err := handler.client.UpdateProject(
		request.Context(),
		&workv1.UpdateProjectRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			ProjectId:       chi.URLParam(request, "projectID"),
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

func (handler *Handler) changeProjectStatus(
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
			"Invalid project status request",
		)
		return
	}

	response, err := handler.client.ChangeProjectStatus(
		request.Context(),
		&workv1.ChangeProjectStatusRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			ProjectId:       chi.URLParam(request, "projectID"),
			ExpectedVersion: input.ExpectedVersion,
			Status: workv1.ProjectStatus(
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
