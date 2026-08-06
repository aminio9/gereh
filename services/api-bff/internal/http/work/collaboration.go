package work

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (handler *Handler) registerCollaborationRoutes(
	tenantRouter chi.Router,
	authHandler *authhttp.Handler,
) {
	tenantRouter.Route(
		"/tasks/{taskID}/dependencies",
		func(dependencyRouter chi.Router) {
			dependencyRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.addTaskDependency,
			)

			dependencyRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE,
				),
				authHandler.RequireCSRF,
			).Delete(
				"/{dependsOnTaskID}",
				handler.removeTaskDependency,
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}/assignments",
		func(assignmentRouter chi.Router) {
			assignmentRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_ASSIGN,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.assignTask,
			)

			assignmentRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_ASSIGN,
				),
				authHandler.RequireCSRF,
			).Delete(
				"/{assignmentID}",
				handler.unassignTask,
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}/comments",
		func(commentRouter chi.Router) {
			commentRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.addComment,
			)

			commentRouter.Route(
				"/{commentID}",
				func(resource chi.Router) {
					resource.With(
						handler.RequireAnyPermission(
							tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE,
							tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/",
						handler.updateComment,
					)

					resource.With(
						handler.RequireAnyPermission(
							tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE,
							tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE,
						),
						authHandler.RequireCSRF,
					).Delete(
						"/",
						handler.deleteComment,
					)
				},
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}/artifacts",
		func(artifactRouter chi.Router) {
			artifactRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.addArtifact,
			)

			artifactRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE,
				),
				authHandler.RequireCSRF,
			).Delete(
				"/{artifactID}",
				handler.deleteArtifact,
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}/checklist",
		func(checklistRouter chi.Router) {
			checklistRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.addChecklistItem,
			)

			checklistRouter.Route(
				"/{itemID}",
				func(resource chi.Router) {
					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
						),
						authHandler.RequireCSRF,
					).Patch(
						"/",
						handler.updateChecklistItem,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
						),
						authHandler.RequireCSRF,
					).Delete(
						"/",
						handler.deleteChecklistItem,
					)
				},
			)
		},
	)

	tenantRouter.Route(
		"/tasks/{taskID}/schedule",
		func(scheduleRouter chi.Router) {
			scheduleRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE,
				),
				authHandler.RequireCSRF,
			).Put(
				"/",
				handler.upsertTaskSchedule,
			)

			scheduleRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE,
				),
				authHandler.RequireCSRF,
			).Delete(
				"/",
				handler.deleteTaskSchedule,
			)
		},
	)
}

type addDependencyRequest struct {
	DependsOnTaskID string `json:"dependsOnTaskId"`
}

type assignTaskRequest struct {
	AssigneeType workv1.AssigneeType   `json:"assigneeType"`
	UserID       *string               `json:"userId"`
	AgentID      *string               `json:"agentId"`
	Role         workv1.AssignmentRole `json:"role"`
}

type addCommentRequest struct {
	AuthorType    workv1.CommentAuthorType `json:"authorType"`
	AuthorUserID  *string                  `json:"authorUserId"`
	AuthorAgentID *string                  `json:"authorAgentId"`
	Body          string                   `json:"body"`
}

type updateCommentRequest struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	Body            string `json:"body"`
}

type deleteCommentRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type addArtifactRequest struct {
	CompanyID   string         `json:"companyId"`
	ObjectKey   string         `json:"objectKey"`
	FileName    string         `json:"fileName"`
	ContentType string         `json:"contentType"`
	SizeBytes   int64          `json:"sizeBytes"`
	SHA256      string         `json:"sha256"`
	Metadata    map[string]any `json:"metadata"`
}

type addChecklistItemRequest struct {
	Title string `json:"title"`
}

type updateChecklistItemRequest struct {
	ExpectedVersion int64   `json:"expectedVersion"`
	Title           *string `json:"title"`
	Completed       *bool   `json:"completed"`
}

type deleteChecklistItemRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type upsertScheduleRequest struct {
	NotBefore *timestamppb.Timestamp `json:"notBefore"`
	DueAt     *timestamppb.Timestamp `json:"dueAt"`
	Timezone  string                 `json:"timezone"`
}

func (handler *Handler) addTaskDependency(
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

	var input addDependencyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid dependency request",
		)
		return
	}

	response, err := handler.client.AddTaskDependency(
		request.Context(),
		&workv1.AddTaskDependencyRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			DependsOnTaskId: input.DependsOnTaskID,
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

func (handler *Handler) removeTaskDependency(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.RemoveTaskDependency(
		request.Context(),
		&workv1.RemoveTaskDependencyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
			DependsOnTaskId: chi.URLParam(
				request,
				"dependsOnTaskID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) assignTask(
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

	var input assignTaskRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid assignment request",
		)
		return
	}

	response, err := handler.client.AssignTask(
		request.Context(),
		&workv1.AssignTaskRequest{
			ActorUserId:  principal.UserID,
			TenantId:     tenantID(request),
			TaskId:       chi.URLParam(request, "taskID"),
			AssigneeType: input.AssigneeType,
			UserId:       input.UserID,
			AgentId:      input.AgentID,
			Role:         input.Role,
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

func (handler *Handler) unassignTask(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.UnassignTask(
		request.Context(),
		&workv1.UnassignTaskRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
			AssignmentId: chi.URLParam(
				request,
				"assignmentID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) addComment(
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

	var input addCommentRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid comment request",
		)
		return
	}

	response, err := handler.client.AddComment(
		request.Context(),
		&workv1.AddCommentRequest{
			ActorUserId:   principal.UserID,
			TenantId:      tenantID(request),
			TaskId:        chi.URLParam(request, "taskID"),
			AuthorType:    input.AuthorType,
			AuthorUserId:  input.AuthorUserID,
			AuthorAgentId: input.AuthorAgentID,
			Body:          input.Body,
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

func (handler *Handler) updateComment(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateCommentRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid comment request",
		)
		return
	}

	response, err := handler.client.UpdateComment(
		request.Context(),
		&workv1.UpdateCommentRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			CommentId:       chi.URLParam(request, "commentID"),
			ExpectedVersion: input.ExpectedVersion,
			Body:            input.Body,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) deleteComment(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input deleteCommentRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid comment request",
		)
		return
	}

	response, err := handler.client.DeleteComment(
		request.Context(),
		&workv1.DeleteCommentRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			CommentId:       chi.URLParam(request, "commentID"),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) addArtifact(
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

	var input addArtifactRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid artifact request",
		)
		return
	}

	var metadata *structpb.Struct

	if input.Metadata != nil {
		metadata = configurationStruct(input.Metadata)
	}

	response, err := handler.client.AddArtifact(
		request.Context(),
		&workv1.AddArtifactRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			CompanyId:   input.CompanyID,
			TaskId:      chi.URLParam(request, "taskID"),
			ObjectKey:   input.ObjectKey,
			FileName:    input.FileName,
			ContentType: input.ContentType,
			SizeBytes:   input.SizeBytes,
			Sha256:      input.SHA256,
			Metadata:    metadata,
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

func (handler *Handler) deleteArtifact(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.DeleteArtifact(
		request.Context(),
		&workv1.DeleteArtifactRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
			ArtifactId:  chi.URLParam(request, "artifactID"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) addChecklistItem(
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

	var input addChecklistItemRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid checklist item request",
		)
		return
	}

	response, err := handler.client.AddChecklistItem(
		request.Context(),
		&workv1.AddChecklistItemRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
			Title:       input.Title,
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

func (handler *Handler) updateChecklistItem(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input updateChecklistItemRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid checklist item request",
		)
		return
	}

	response, err := handler.client.UpdateChecklistItem(
		request.Context(),
		&workv1.UpdateChecklistItemRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			ItemId:          chi.URLParam(request, "itemID"),
			ExpectedVersion: input.ExpectedVersion,
			Title:           input.Title,
			Completed:       input.Completed,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) deleteChecklistItem(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input deleteChecklistItemRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid checklist item request",
		)
		return
	}

	response, err := handler.client.DeleteChecklistItem(
		request.Context(),
		&workv1.DeleteChecklistItemRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			TaskId:          chi.URLParam(request, "taskID"),
			ItemId:          chi.URLParam(request, "itemID"),
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) upsertTaskSchedule(
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

	var input upsertScheduleRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid schedule request",
		)
		return
	}

	response, err := handler.client.UpsertTaskSchedule(
		request.Context(),
		&workv1.UpsertTaskScheduleRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
			NotBefore:   input.NotBefore,
			DueAt:       input.DueAt,
			Timezone:    input.Timezone,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) deleteTaskSchedule(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.DeleteTaskSchedule(
		request.Context(),
		&workv1.DeleteTaskScheduleRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			TaskId:      chi.URLParam(request, "taskID"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
