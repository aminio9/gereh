package work

import (
	"net/http"
	"strconv"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
)

func (handler *Handler) registerTaskRoutes(
	tenantRouter chi.Router,
	authHandler *authhttp.Handler,
) {
	tenantRouter.Route(
		"/projects/{projectID}/tasks",
		func(taskRouter chi.Router) {
			taskRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.createTask,
			)

			taskRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_WORK_READ,
				),
			).Get(
				"/",
				handler.listTasks,
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}",
		func(resource chi.Router) {
			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_WORK_READ,
				),
			).Get(
				"/",
				handler.getTask,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_UPDATE,
				),
				authHandler.RequireCSRF,
			).Patch(
				"/",
				handler.updateTask,
			)

			resource.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_STATUS_UPDATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/status",
				handler.changeTaskStatus,
			)
		},
	)
}

type createTaskRequest struct {
	CompanyID    string              `json:"companyId"`
	ParentTaskID *string             `json:"parentTaskId"`
	Title        string              `json:"title"`
	Description  string              `json:"description"`
	Priority     workv1.TaskPriority `json:"priority"`
}

type updateTaskRequest struct {
	ExpectedVersion int64                `json:"expectedVersion"`
	Title           *string              `json:"title"`
	Description     *string              `json:"description"`
	Priority        *workv1.TaskPriority `json:"priority"`
}

func (handler *Handler) createTask(
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

	var input createTaskRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid task request",
		)
		return
	}

	response, err := handler.client.CreateTask(
		request.Context(),
		&workv1.CreateTaskRequest{
			ActorUserId:  principal.UserID,
			TenantId:     tenantID(request),
			CompanyId:    input.CompanyID,
			ProjectId:    chi.URLParam(request, "projectID"),
			ParentTaskId: input.ParentTaskID,
			Title:        input.Title,
			Description:  input.Description,
			Priority:     input.Priority,
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

func (handler *Handler) getTask(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetTask(
		request.Context(),
		&workv1.GetTaskRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId: chi.URLParam(
				request,
				"taskID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listTasks(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListTasks(
		request.Context(),
		&workv1.ListTasksRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId: request.URL.Query().
				Get("company_id"),
			ProjectId: chi.URLParam(
				request,
				"projectID",
			),
			PageSize: int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
			IncludeCanceled: request.URL.Query().
				Get("include_canceled") == "true",
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateTask(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateTaskRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid task request",
		)
		return
	}

	taskRequest := &workv1.UpdateTaskRequest{
		ActorUserId:     principal.UserID,
		TenantId:        tenantID(request),
		TaskId:          chi.URLParam(request, "taskID"),
		ExpectedVersion: input.ExpectedVersion,
		Title:           input.Title,
		Description:     input.Description,
	}

	if input.Priority != nil {
		taskRequest.Priority = input.Priority
	}

	response, err := handler.client.UpdateTask(
		request.Context(),
		taskRequest,
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) changeTaskStatus(
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
			"Invalid task status request",
		)
		return
	}

	response, err := handler.client.ChangeTaskStatus(
		request.Context(),
		&workv1.ChangeTaskStatusRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			ExpectedVersion: input.ExpectedVersion,
			Status: workv1.TaskStatus(
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
