// Package protoutil maps Work Management domain values to Protobuf and back.
package protoutil

import (
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"google.golang.org/protobuf/types/known/structpb"
)

// Goal maps a domain goal to Protobuf.
func Goal(value domain.Goal) *workv1.Goal {
	return &workv1.Goal{
		TenantId:        value.TenantID,
		CompanyId:       value.CompanyID,
		GoalId:          value.ID,
		Title:           value.Title,
		Description:     value.Description,
		Status:          GoalStatus(value.Status),
		Version:         value.Version,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       protoTimestamp(value.CreatedAt),
		UpdatedAt:       protoTimestamp(value.UpdatedAt),
		CompletedAt:     protoTimestampPointer(value.CompletedAt),
		ArchivedAt:      protoTimestampPointer(value.ArchivedAt),
	}
}

// Project maps a domain project to Protobuf.
func Project(value domain.Project) *workv1.Project {
	return &workv1.Project{
		TenantId:        value.TenantID,
		CompanyId:       value.CompanyID,
		GoalId:          value.GoalID,
		ProjectId:       value.ID,
		Title:           value.Title,
		Description:     value.Description,
		Status:          ProjectStatus(value.Status),
		Version:         value.Version,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       protoTimestamp(value.CreatedAt),
		UpdatedAt:       protoTimestamp(value.UpdatedAt),
		CompletedAt:     protoTimestampPointer(value.CompletedAt),
		ArchivedAt:      protoTimestampPointer(value.ArchivedAt),
	}
}

// Task maps a domain task view to Protobuf.
func Task(value domain.TaskView) *workv1.Task {
	parentTaskID := optionalString(value.ParentTaskID)

	return &workv1.Task{
		TenantId:                  value.TenantID,
		CompanyId:                 value.CompanyID,
		ProjectId:                 value.ProjectID,
		TaskId:                    value.ID,
		ParentTaskId:              parentTaskID,
		Title:                     value.Title,
		Description:               value.Description,
		Status:                    TaskStatus(value.Status),
		Priority:                  TaskPriority(value.Priority),
		Blocked:                   value.Blocked,
		DependencyCount:           value.DependencyCount,
		IncompleteDependencyCount: value.IncompleteDependencyCount,
		Version:                   value.Version,
		CreatedByUserId:           value.CreatedByUserID,
		CreatedAt:                 protoTimestamp(value.CreatedAt),
		UpdatedAt:                 protoTimestamp(value.UpdatedAt),
		CompletedAt:               protoTimestampPointer(value.CompletedAt),
		CanceledAt:                protoTimestampPointer(value.CanceledAt),
	}
}

// TaskDependency maps a domain dependency to Protobuf.
func TaskDependency(value domain.TaskDependency) *workv1.TaskDependency {
	return &workv1.TaskDependency{
		TenantId:        value.TenantID,
		ProjectId:       value.ProjectID,
		TaskId:          value.TaskID,
		DependsOnTaskId: value.DependsOnTaskID,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       protoTimestamp(value.CreatedAt),
	}
}

// TaskAssignment maps a domain assignment to Protobuf.
func TaskAssignment(value domain.TaskAssignment) *workv1.TaskAssignment {
	return &workv1.TaskAssignment{
		TenantId:         value.TenantID,
		TaskId:           value.TaskID,
		AssignmentId:     value.ID,
		AssigneeType:     AssigneeType(value.AssigneeType),
		UserId:           optionalString(value.UserID),
		AgentId:          optionalString(value.AgentID),
		Role:             AssignmentRole(value.Role),
		AssignedByUserId: value.AssignedByUserID,
		AssignedAt:       protoTimestamp(value.AssignedAt),
	}
}

// Comment maps a domain comment to Protobuf.
func Comment(value domain.Comment) *workv1.Comment {
	return &workv1.Comment{
		TenantId:      value.TenantID,
		TaskId:        value.TaskID,
		CommentId:     value.ID,
		AuthorType:    CommentAuthorType(value.AuthorType),
		AuthorUserId:  optionalString(value.AuthorUserID),
		AuthorAgentId: optionalString(value.AuthorAgentID),
		Body:          value.Body,
		Version:       value.Version,
		CreatedAt:     protoTimestamp(value.CreatedAt),
		UpdatedAt:     protoTimestamp(value.UpdatedAt),
		DeletedAt:     protoTimestampPointer(value.DeletedAt),
	}
}

// Artifact maps a domain artifact to Protobuf.
func Artifact(value domain.Artifact) *workv1.Artifact {
	metadata, err := structpb.NewStruct(value.Metadata)
	if err != nil {
		metadata, _ = structpb.NewStruct(nil)
	}

	return &workv1.Artifact{
		TenantId:        value.TenantID,
		CompanyId:       value.CompanyID,
		TaskId:          value.TaskID,
		ArtifactId:      value.ID,
		ObjectKey:       value.ObjectKey,
		FileName:        value.FileName,
		ContentType:     value.ContentType,
		SizeBytes:       value.SizeBytes,
		Sha256:          value.SHA256,
		Metadata:        metadata,
		CreatedByUserId: value.CreatedByUserID,
		CreatedAt:       protoTimestamp(value.CreatedAt),
		DeletedAt:       protoTimestampPointer(value.DeletedAt),
	}
}

// ChecklistItem maps a domain checklist item to Protobuf.
func ChecklistItem(value domain.ChecklistItem) *workv1.ChecklistItem {
	return &workv1.ChecklistItem{
		TenantId:  value.TenantID,
		TaskId:    value.TaskID,
		ItemId:    value.ID,
		Title:     value.Title,
		Completed: value.Completed,
		Position:  value.Position,
		Version:   value.Version,
		CreatedAt: protoTimestamp(value.CreatedAt),
		UpdatedAt: protoTimestamp(value.UpdatedAt),
	}
}

// TaskSchedule maps a domain schedule to Protobuf.
func TaskSchedule(value domain.TaskSchedule) *workv1.TaskSchedule {
	return &workv1.TaskSchedule{
		TenantId:  value.TenantID,
		TaskId:    value.TaskID,
		NotBefore: protoTimestampPointer(value.NotBefore),
		DueAt:     protoTimestampPointer(value.DueAt),
		Timezone:  value.Timezone,
		Version:   value.Version,
		UpdatedAt: protoTimestamp(value.UpdatedAt),
	}
}
