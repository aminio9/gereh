package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/aminio9/gereh/services/work-management/internal/protoutil"
	"github.com/google/uuid"
)

// AddDependencyInput is the input to dependency creation.
type AddDependencyInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	DependsOnTaskID string
}

// RemoveDependencyInput is the input to dependency removal.
type RemoveDependencyInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	DependsOnTaskID string
}

// AddTaskDependency links a task to a prerequisite.
func (service *Service) AddTaskDependency(
	ctx context.Context,
	input AddDependencyInput,
) (domain.TaskDependency, error) {
	for name, value := range map[string]string{
		"actor_user_id":      input.ActorUserID,
		"tenant_id":          input.TenantID,
		"task_id":            input.TaskID,
		"depends_on_task_id": input.DependsOnTaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskDependency{}, err
		}
	}

	if input.TaskID == input.DependsOnTaskID {
		return domain.TaskDependency{}, fmt.Errorf(
			"%w: a task cannot depend on itself",
			domain.ErrInvalidArgument,
		)
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE,
	); err != nil {
		return domain.TaskDependency{}, err
	}

	task, err := service.repository.GetTask(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return domain.TaskDependency{}, err
	}

	now := service.now().UTC()

	dependency := domain.TaskDependency{
		TenantID:        input.TenantID,
		ProjectID:       task.ProjectID,
		TaskID:          input.TaskID,
		DependsOnTaskID: input.DependsOnTaskID,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		dependency.TaskID,
		"task.dependency_added",
		dependency.TenantID,
		"task",
		dependency.TaskID,
		0,
		&workv1.TaskDependencyAdded{
			Dependency: protoutil.TaskDependency(dependency),
		},
		now,
	)
	if err != nil {
		return domain.TaskDependency{}, err
	}

	return service.repository.AddDependency(
		ctx,
		ports.AddDependencyParams{
			ActorUserID: input.ActorUserID,
			Dependency:  dependency,
			Event:       event,
		},
	)
}

// RemoveTaskDependency removes a prerequisite link.
func (service *Service) RemoveTaskDependency(
	ctx context.Context,
	input RemoveDependencyInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id":      input.ActorUserID,
		"tenant_id":          input.TenantID,
		"task_id":            input.TaskID,
		"depends_on_task_id": input.DependsOnTaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_DEPENDENCY_MANAGE,
	); err != nil {
		return err
	}

	task, err := service.repository.GetTask(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return err
	}

	now := service.now().UTC()

	dependency := domain.TaskDependency{
		TenantID:        input.TenantID,
		ProjectID:       task.ProjectID,
		TaskID:          input.TaskID,
		DependsOnTaskID: input.DependsOnTaskID,
		CreatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		dependency.TaskID,
		"task.dependency_removed",
		dependency.TenantID,
		"task",
		dependency.TaskID,
		0,
		&workv1.TaskDependencyRemoved{
			Dependency:      protoutil.TaskDependency(dependency),
			RemovedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return err
	}

	return service.repository.RemoveDependency(
		ctx,
		ports.RemoveDependencyParams{
			ActorUserID: input.ActorUserID,
			Dependency:  dependency,
			Event:       event,
		},
	)
}

// AssignTaskInput is the input to task assignment.
type AssignTaskInput struct {
	ActorUserID string
	TenantID    string
	TaskID      string
	Assignee    workv1.AssigneeType
	UserID      *string
	AgentID     *string
	Role        workv1.AssignmentRole
}

// AssignTask assigns a user or agent to a task.
func (service *Service) AssignTask(
	ctx context.Context,
	input AssignTaskInput,
) (domain.TaskAssignment, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskAssignment{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_ASSIGN,
	); err != nil {
		return domain.TaskAssignment{}, err
	}

	var assigneeType domain.AssigneeType

	switch input.Assignee {
	case workv1.AssigneeType_ASSIGNEE_TYPE_USER:
		assigneeType = domain.AssigneeTypeUser

		if input.UserID == nil {
			return domain.TaskAssignment{}, fmt.Errorf(
				"%w: user_id is required for user assignment",
				domain.ErrInvalidArgument,
			)
		}

		if input.AgentID != nil {
			return domain.TaskAssignment{}, fmt.Errorf(
				"%w: agent_id must be empty for user assignment",
				domain.ErrInvalidArgument,
			)
		}

		if err := validateUUID("user_id", *input.UserID); err != nil {
			return domain.TaskAssignment{}, err
		}

	case workv1.AssigneeType_ASSIGNEE_TYPE_AGENT:
		assigneeType = domain.AssigneeTypeAgent

		if input.AgentID == nil {
			return domain.TaskAssignment{}, fmt.Errorf(
				"%w: agent_id is required for agent assignment",
				domain.ErrInvalidArgument,
			)
		}

		if input.UserID != nil {
			return domain.TaskAssignment{}, fmt.Errorf(
				"%w: user_id must be empty for agent assignment",
				domain.ErrInvalidArgument,
			)
		}

		if err := validateUUID("agent_id", *input.AgentID); err != nil {
			return domain.TaskAssignment{}, err
		}

	default:
		return domain.TaskAssignment{}, fmt.Errorf(
			"%w: assignee type is required",
			domain.ErrInvalidArgument,
		)
	}

	var role domain.AssignmentRole

	switch input.Role {
	case workv1.AssignmentRole_ASSIGNMENT_ROLE_PRIMARY:
		role = domain.AssignmentRolePrimary

	case workv1.AssignmentRole_ASSIGNMENT_ROLE_COLLABORATOR:
		role = domain.AssignmentRoleCollaborator

	case workv1.AssignmentRole_ASSIGNMENT_ROLE_REVIEWER:
		role = domain.AssignmentRoleReviewer

	default:
		return domain.TaskAssignment{}, fmt.Errorf(
			"%w: assignment role is required",
			domain.ErrInvalidArgument,
		)
	}

	assignmentID, err := uuid.NewV7()
	if err != nil {
		return domain.TaskAssignment{}, fmt.Errorf(
			"generate assignment ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	assignment := domain.TaskAssignment{
		TenantID:         input.TenantID,
		TaskID:           input.TaskID,
		ID:               assignmentID.String(),
		AssigneeType:     assigneeType,
		UserID:           input.UserID,
		AgentID:          input.AgentID,
		Role:             role,
		AssignedByUserID: input.ActorUserID,
		AssignedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		assignment.TaskID,
		"task.assigned",
		assignment.TenantID,
		"task",
		assignment.TaskID,
		0,
		&workv1.TaskAssigned{
			Assignment: protoutil.TaskAssignment(assignment),
		},
		now,
	)
	if err != nil {
		return domain.TaskAssignment{}, err
	}

	return service.repository.AssignTask(
		ctx,
		ports.AssignTaskParams{
			ActorUserID: input.ActorUserID,
			Assignment:  assignment,
			Event:       event,
		},
	)
}

// UnassignTaskInput is the input to assignment removal.
type UnassignTaskInput struct {
	ActorUserID  string
	TenantID     string
	TaskID       string
	AssignmentID string
}

// UnassignTask removes a task assignment.
func (service *Service) UnassignTask(
	ctx context.Context,
	input UnassignTaskInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"assignment_id": input.AssignmentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_ASSIGN,
	); err != nil {
		return err
	}

	assignment, err := service.repository.GetAssignment(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.AssignmentID,
	)
	if err != nil {
		return err
	}

	now := service.now().UTC()

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		assignment.TaskID,
		"task.unassigned",
		assignment.TenantID,
		"task",
		assignment.TaskID,
		0,
		&workv1.TaskUnassigned{
			Assignment:      protoutil.TaskAssignment(assignment),
			RemovedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return err
	}

	return service.repository.UnassignTask(
		ctx,
		ports.UnassignTaskParams{
			ActorUserID: input.ActorUserID,
			Assignment:  assignment,
			Event:       event,
		},
	)
}

// AddCommentInput is the input to comment creation.
type AddCommentInput struct {
	ActorUserID   string
	TenantID      string
	TaskID        string
	AuthorType    workv1.CommentAuthorType
	AuthorUserID  *string
	AuthorAgentID *string
	Body          string
}

// AddComment posts a comment on a task.
func (service *Service) AddComment(
	ctx context.Context,
	input AddCommentInput,
) (domain.Comment, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Comment{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_COMMENT_CREATE,
	); err != nil {
		return domain.Comment{}, err
	}

	var authorType domain.AssigneeType

	switch input.AuthorType {
	case workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_USER:
		authorType = domain.AssigneeTypeUser

		if input.AuthorUserID == nil {
			return domain.Comment{}, fmt.Errorf(
				"%w: author_user_id is required for user comments",
				domain.ErrInvalidArgument,
			)
		}

		if *input.AuthorUserID != input.ActorUserID {
			return domain.Comment{}, fmt.Errorf(
				"%w: author_user_id must match the actor",
				domain.ErrInvalidArgument,
			)
		}

		if input.AuthorAgentID != nil {
			return domain.Comment{}, fmt.Errorf(
				"%w: author_agent_id must be empty for user comments",
				domain.ErrInvalidArgument,
			)
		}

	case workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_AGENT:
		authorType = domain.AssigneeTypeAgent

		if input.AuthorAgentID == nil {
			return domain.Comment{}, fmt.Errorf(
				"%w: author_agent_id is required for agent comments",
				domain.ErrInvalidArgument,
			)
		}

		if input.AuthorUserID != nil {
			return domain.Comment{}, fmt.Errorf(
				"%w: author_user_id must be empty for agent comments",
				domain.ErrInvalidArgument,
			)
		}

	default:
		return domain.Comment{}, fmt.Errorf(
			"%w: comment author type is required",
			domain.ErrInvalidArgument,
		)
	}

	body, err := boundedText(
		"body",
		input.Body,
		1,
		16000,
	)
	if err != nil {
		return domain.Comment{}, err
	}

	commentID, err := uuid.NewV7()
	if err != nil {
		return domain.Comment{}, fmt.Errorf(
			"generate comment ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	comment := domain.Comment{
		TenantID:      input.TenantID,
		TaskID:        input.TaskID,
		ID:            commentID.String(),
		AuthorType:    authorType,
		AuthorUserID:  input.AuthorUserID,
		AuthorAgentID: input.AuthorAgentID,
		Body:          body,
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		comment.TaskID,
		"task.comment_added",
		comment.TenantID,
		"task",
		comment.TaskID,
		0,
		&workv1.TaskCommentAdded{
			Comment: protoutil.Comment(comment),
		},
		now,
	)
	if err != nil {
		return domain.Comment{}, err
	}

	return service.repository.AddComment(
		ctx,
		ports.AddCommentParams{
			ActorUserID: input.ActorUserID,
			Comment:     comment,
			Event:       event,
		},
	)
}

// UpdateCommentInput is the input to a comment update.
type UpdateCommentInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	CommentID       string
	ExpectedVersion int64
	Body            string
}

// UpdateComment edits a comment the actor authored or may moderate.
func (service *Service) UpdateComment(
	ctx context.Context,
	input UpdateCommentInput,
) (domain.Comment, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"comment_id":    input.CommentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Comment{}, err
		}
	}

	comment, err := service.repository.GetComment(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.CommentID,
	)
	if err != nil {
		return domain.Comment{}, err
	}

	if err := service.authorizeCommentMutation(
		ctx,
		input.ActorUserID,
		input.TenantID,
		comment,
	); err != nil {
		return domain.Comment{}, err
	}

	body, err := boundedText(
		"body",
		input.Body,
		1,
		16000,
	)
	if err != nil {
		return domain.Comment{}, err
	}

	now := service.now().UTC()

	comment.Body = body
	comment.Version++
	comment.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		comment.TaskID,
		"task.comment_updated",
		comment.TenantID,
		"task",
		comment.TaskID,
		0,
		&workv1.TaskCommentUpdated{
			Comment: protoutil.Comment(comment),
		},
		now,
	)
	if err != nil {
		return domain.Comment{}, err
	}

	return service.repository.UpdateComment(
		ctx,
		ports.UpdateCommentParams{
			ActorUserID:     input.ActorUserID,
			Comment:         comment,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// DeleteCommentInput is the input to comment deletion.
type DeleteCommentInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	CommentID       string
	ExpectedVersion int64
}

// DeleteComment soft-deletes a comment.
func (service *Service) DeleteComment(
	ctx context.Context,
	input DeleteCommentInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"comment_id":    input.CommentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	comment, err := service.repository.GetComment(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.CommentID,
	)
	if err != nil {
		return err
	}

	if err := service.authorizeCommentMutation(
		ctx,
		input.ActorUserID,
		input.TenantID,
		comment,
	); err != nil {
		return err
	}

	now := service.now().UTC()

	comment.DeletedAt = &now
	comment.Version++
	comment.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		comment.TaskID,
		"task.comment_deleted",
		comment.TenantID,
		"task",
		comment.TaskID,
		0,
		&workv1.TaskCommentDeleted{
			Comment: protoutil.Comment(comment),
		},
		now,
	)
	if err != nil {
		return err
	}

	_, err = service.repository.DeleteComment(
		ctx,
		ports.DeleteCommentParams{
			ActorUserID:     input.ActorUserID,
			Comment:         comment,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)

	return err
}

// ListComments returns a task's comments.
func (service *Service) ListComments(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Comment, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListComments(
		ctx,
		actorUserID,
		tenantID,
		taskID,
	)
}

// authorizeCommentMutation permits the author or a moderator.
func (service *Service) authorizeCommentMutation(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	comment domain.Comment,
) error {
	if comment.AuthorType == domain.AssigneeTypeUser &&
		comment.AuthorUserID != nil &&
		*comment.AuthorUserID == actorUserID {
		return nil
	}

	return service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_TASK_COMMENT_MODERATE,
	)
}

// AddArtifactInput is the input to artifact metadata creation.
type AddArtifactInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string
	TaskID      string
	ObjectKey   string
	FileName    string
	ContentType string
	SizeBytes   int64
	SHA256      string
	Metadata    map[string]any
}

// AddArtifact records artifact metadata for a task.
func (service *Service) AddArtifact(
	ctx context.Context,
	input AddArtifactInput,
) (domain.Artifact, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Artifact{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE,
	); err != nil {
		return domain.Artifact{}, err
	}

	if err := validateArtifact(
		input.ObjectKey,
		input.FileName,
		input.ContentType,
		input.SizeBytes,
		input.SHA256,
	); err != nil {
		return domain.Artifact{}, err
	}

	task, err := service.repository.GetTask(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return domain.Artifact{}, err
	}

	if task.CompanyID != input.CompanyID {
		return domain.Artifact{}, domain.ErrNotFound
	}

	artifactID, err := uuid.NewV7()
	if err != nil {
		return domain.Artifact{}, fmt.Errorf(
			"generate artifact ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	artifact := domain.Artifact{
		TenantID:        input.TenantID,
		CompanyID:       input.CompanyID,
		TaskID:          input.TaskID,
		ID:              artifactID.String(),
		ObjectKey:       strings.TrimSpace(input.ObjectKey),
		FileName:        strings.TrimSpace(input.FileName),
		ContentType:     strings.TrimSpace(input.ContentType),
		SizeBytes:       input.SizeBytes,
		SHA256:          strings.ToLower(strings.TrimSpace(input.SHA256)),
		Metadata:        input.Metadata,
		CreatedByUserID: input.ActorUserID,
		CreatedAt:       now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		artifact.TaskID,
		"task.artifact_added",
		artifact.TenantID,
		"task",
		artifact.TaskID,
		0,
		&workv1.TaskArtifactAdded{
			Artifact: protoutil.Artifact(artifact),
		},
		now,
	)
	if err != nil {
		return domain.Artifact{}, err
	}

	return service.repository.AddArtifact(
		ctx,
		ports.AddArtifactParams{
			ActorUserID: input.ActorUserID,
			Artifact:    artifact,
			Event:       event,
		},
	)
}

// DeleteArtifactInput is the input to artifact deletion.
type DeleteArtifactInput struct {
	ActorUserID string
	TenantID    string
	TaskID      string
	ArtifactID  string
}

// DeleteArtifact soft-deletes artifact metadata.
func (service *Service) DeleteArtifact(
	ctx context.Context,
	input DeleteArtifactInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"artifact_id":   input.ArtifactID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_ARTIFACT_MANAGE,
	); err != nil {
		return err
	}

	artifact, err := service.repository.GetArtifact(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.ArtifactID,
	)
	if err != nil {
		return err
	}

	now := service.now().UTC()

	artifact.DeletedAt = &now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		artifact.TaskID,
		"task.artifact_deleted",
		artifact.TenantID,
		"task",
		artifact.TaskID,
		0,
		&workv1.TaskArtifactDeleted{
			Artifact: protoutil.Artifact(artifact),
		},
		now,
	)
	if err != nil {
		return err
	}

	_, err = service.repository.DeleteArtifact(
		ctx,
		ports.DeleteArtifactParams{
			ActorUserID: input.ActorUserID,
			Artifact:    artifact,
			Event:       event,
		},
	)

	return err
}

// ListArtifacts returns a task's artifacts.
func (service *Service) ListArtifacts(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Artifact, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListArtifacts(
		ctx,
		actorUserID,
		tenantID,
		taskID,
	)
}

// AddChecklistItemInput is the input to checklist item creation.
type AddChecklistItemInput struct {
	ActorUserID string
	TenantID    string
	TaskID      string
	Title       string
}

// AddChecklistItem appends a checklist item to a task.
func (service *Service) AddChecklistItem(
	ctx context.Context,
	input AddChecklistItemInput,
) (domain.ChecklistItem, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.ChecklistItem{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
	); err != nil {
		return domain.ChecklistItem{}, err
	}

	title, err := boundedText(
		"title",
		input.Title,
		1,
		500,
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	items, err := service.repository.ListChecklist(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	itemID, err := uuid.NewV7()
	if err != nil {
		return domain.ChecklistItem{}, fmt.Errorf(
			"generate checklist item ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	item := domain.ChecklistItem{
		TenantID:  input.TenantID,
		TaskID:    input.TaskID,
		ID:        itemID.String(),
		Title:     title,
		Completed: false,
		Position:  int32(len(items)),
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		item.TaskID,
		"task.checklist_changed",
		item.TenantID,
		"task",
		item.TaskID,
		0,
		&workv1.TaskChecklistChanged{
			Item:       protoutil.ChecklistItem(item),
			ChangeKind: "added",
		},
		now,
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	return service.repository.AddChecklistItem(
		ctx,
		ports.AddChecklistItemParams{
			ActorUserID: input.ActorUserID,
			Item:        item,
			Event:       event,
		},
	)
}

// UpdateChecklistItemInput is the input to a checklist item update.
type UpdateChecklistItemInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	ItemID          string
	ExpectedVersion int64
	Title           *string
	Completed       *bool
}

// UpdateChecklistItem edits a checklist item.
func (service *Service) UpdateChecklistItem(
	ctx context.Context,
	input UpdateChecklistItemInput,
) (domain.ChecklistItem, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"item_id":       input.ItemID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.ChecklistItem{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
	); err != nil {
		return domain.ChecklistItem{}, err
	}

	item, err := service.repository.GetChecklistItem(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.ItemID,
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	if input.Title != nil {
		title, err := boundedText(
			"title",
			*input.Title,
			1,
			500,
		)
		if err != nil {
			return domain.ChecklistItem{}, err
		}

		item.Title = title
	}

	if input.Completed != nil {
		item.Completed = *input.Completed
	}

	now := service.now().UTC()

	item.Version++
	item.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		item.TaskID,
		"task.checklist_changed",
		item.TenantID,
		"task",
		item.TaskID,
		0,
		&workv1.TaskChecklistChanged{
			Item:       protoutil.ChecklistItem(item),
			ChangeKind: "updated",
		},
		now,
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	return service.repository.UpdateChecklistItem(
		ctx,
		ports.UpdateChecklistItemParams{
			ActorUserID:     input.ActorUserID,
			Item:            item,
			ExpectedVersion: input.ExpectedVersion,
			Event:           event,
		},
	)
}

// DeleteChecklistItemInput is the input to checklist item deletion.
type DeleteChecklistItemInput struct {
	ActorUserID     string
	TenantID        string
	TaskID          string
	ItemID          string
	ExpectedVersion int64
}

// DeleteChecklistItem removes a checklist item.
func (service *Service) DeleteChecklistItem(
	ctx context.Context,
	input DeleteChecklistItemInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
		"item_id":       input.ItemID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_CHECKLIST_MANAGE,
	); err != nil {
		return err
	}

	item, err := service.repository.GetChecklistItem(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
		input.ItemID,
	)
	if err != nil {
		return err
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		item.TaskID,
		"task.checklist_changed",
		item.TenantID,
		"task",
		item.TaskID,
		0,
		&workv1.TaskChecklistChanged{
			Item:       protoutil.ChecklistItem(item),
			ChangeKind: "deleted",
		},
		service.now().UTC(),
	)
	if err != nil {
		return err
	}

	return service.repository.DeleteChecklistItem(
		ctx,
		ports.DeleteChecklistItemParams{
			ActorUserID: input.ActorUserID,
			Item:        item,
			Event:       event,
		},
	)
}

// ListChecklist returns a task's checklist.
func (service *Service) ListChecklist(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.ChecklistItem, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"task_id":       taskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_WORK_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.ListChecklist(
		ctx,
		actorUserID,
		tenantID,
		taskID,
	)
}

// UpsertScheduleInput is the input to schedule upsert.
type UpsertScheduleInput struct {
	ActorUserID string
	TenantID    string
	TaskID      string
	NotBefore   *time.Time
	DueAt       *time.Time
	Timezone    string
}

// UpsertTaskSchedule sets a task's time window.
func (service *Service) UpsertTaskSchedule(
	ctx context.Context,
	input UpsertScheduleInput,
) (domain.TaskSchedule, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.TaskSchedule{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE,
	); err != nil {
		return domain.TaskSchedule{}, err
	}

	if err := validateSchedule(
		input.NotBefore,
		input.DueAt,
		input.Timezone,
	); err != nil {
		return domain.TaskSchedule{}, err
	}

	if _, err := service.repository.GetTask(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	); err != nil {
		return domain.TaskSchedule{}, err
	}

	now := service.now().UTC()

	schedule := domain.TaskSchedule{
		TenantID:  input.TenantID,
		TaskID:    input.TaskID,
		NotBefore: input.NotBefore,
		DueAt:     input.DueAt,
		Timezone:  strings.TrimSpace(input.Timezone),
		Version:   1,
		UpdatedAt: now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		schedule.TaskID,
		"task.schedule_changed",
		schedule.TenantID,
		"task",
		schedule.TaskID,
		0,
		&workv1.TaskScheduleChanged{
			Schedule:   protoutil.TaskSchedule(schedule),
			ChangeKind: "upserted",
		},
		now,
	)
	if err != nil {
		return domain.TaskSchedule{}, err
	}

	return service.repository.UpsertSchedule(
		ctx,
		ports.UpsertScheduleParams{
			ActorUserID: input.ActorUserID,
			Schedule:    schedule,
			Event:       event,
		},
	)
}

// DeleteScheduleInput is the input to schedule deletion.
type DeleteScheduleInput struct {
	ActorUserID string
	TenantID    string
	TaskID      string
}

// DeleteTaskSchedule removes a task schedule.
func (service *Service) DeleteTaskSchedule(
	ctx context.Context,
	input DeleteScheduleInput,
) error {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"task_id":       input.TaskID,
	} {
		if err := validateUUID(name, value); err != nil {
			return err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_TASK_SCHEDULE_MANAGE,
	); err != nil {
		return err
	}

	schedule, err := service.repository.GetSchedule(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.TaskID,
	)
	if err != nil {
		return err
	}

	now := service.now().UTC()

	schedule.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.EventTopic,
		schedule.TaskID,
		"task.schedule_changed",
		schedule.TenantID,
		"task",
		schedule.TaskID,
		0,
		&workv1.TaskScheduleChanged{
			Schedule:   protoutil.TaskSchedule(schedule),
			ChangeKind: "deleted",
		},
		now,
	)
	if err != nil {
		return err
	}

	return service.repository.DeleteSchedule(
		ctx,
		ports.DeleteScheduleParams{
			ActorUserID: input.ActorUserID,
			Schedule:    schedule,
			Event:       event,
		},
	)
}
