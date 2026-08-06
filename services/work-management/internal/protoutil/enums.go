package protoutil

import (
	"time"

	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GoalStatus maps a domain goal status to Protobuf.
func GoalStatus(value domain.GoalStatus) workv1.GoalStatus {
	switch value {
	case domain.GoalStatusActive:
		return workv1.GoalStatus_GOAL_STATUS_ACTIVE

	case domain.GoalStatusCompleted:
		return workv1.GoalStatus_GOAL_STATUS_COMPLETED

	case domain.GoalStatusCanceled:
		return workv1.GoalStatus_GOAL_STATUS_CANCELED

	case domain.GoalStatusArchived:
		return workv1.GoalStatus_GOAL_STATUS_ARCHIVED

	default:
		return workv1.GoalStatus_GOAL_STATUS_UNSPECIFIED
	}
}

// ProjectStatus maps a domain project status to Protobuf.
func ProjectStatus(value domain.ProjectStatus) workv1.ProjectStatus {
	switch value {
	case domain.ProjectStatusPlanned:
		return workv1.ProjectStatus_PROJECT_STATUS_PLANNED

	case domain.ProjectStatusActive:
		return workv1.ProjectStatus_PROJECT_STATUS_ACTIVE

	case domain.ProjectStatusOnHold:
		return workv1.ProjectStatus_PROJECT_STATUS_ON_HOLD

	case domain.ProjectStatusCompleted:
		return workv1.ProjectStatus_PROJECT_STATUS_COMPLETED

	case domain.ProjectStatusCanceled:
		return workv1.ProjectStatus_PROJECT_STATUS_CANCELED

	case domain.ProjectStatusArchived:
		return workv1.ProjectStatus_PROJECT_STATUS_ARCHIVED

	default:
		return workv1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED
	}
}

// TaskStatus maps a domain task status to Protobuf.
func TaskStatus(value domain.TaskStatus) workv1.TaskStatus {
	switch value {
	case domain.TaskStatusBacklog:
		return workv1.TaskStatus_TASK_STATUS_BACKLOG

	case domain.TaskStatusReady:
		return workv1.TaskStatus_TASK_STATUS_READY

	case domain.TaskStatusInProgress:
		return workv1.TaskStatus_TASK_STATUS_IN_PROGRESS

	case domain.TaskStatusWaitingApproval:
		return workv1.TaskStatus_TASK_STATUS_WAITING_APPROVAL

	case domain.TaskStatusCompleted:
		return workv1.TaskStatus_TASK_STATUS_COMPLETED

	case domain.TaskStatusCanceled:
		return workv1.TaskStatus_TASK_STATUS_CANCELED

	default:
		return workv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

// TaskPriority maps a domain task priority to Protobuf.
func TaskPriority(value domain.TaskPriority) workv1.TaskPriority {
	switch value {
	case domain.TaskPriorityLow:
		return workv1.TaskPriority_TASK_PRIORITY_LOW

	case domain.TaskPriorityNormal:
		return workv1.TaskPriority_TASK_PRIORITY_NORMAL

	case domain.TaskPriorityHigh:
		return workv1.TaskPriority_TASK_PRIORITY_HIGH

	case domain.TaskPriorityUrgent:
		return workv1.TaskPriority_TASK_PRIORITY_URGENT

	default:
		return workv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED
	}
}

// AssigneeType maps a domain assignee type to Protobuf.
func AssigneeType(value domain.AssigneeType) workv1.AssigneeType {
	switch value {
	case domain.AssigneeTypeUser:
		return workv1.AssigneeType_ASSIGNEE_TYPE_USER

	case domain.AssigneeTypeAgent:
		return workv1.AssigneeType_ASSIGNEE_TYPE_AGENT

	default:
		return workv1.AssigneeType_ASSIGNEE_TYPE_UNSPECIFIED
	}
}

// CommentAuthorType maps a domain author type to Protobuf.
func CommentAuthorType(value domain.AssigneeType) workv1.CommentAuthorType {
	switch value {
	case domain.AssigneeTypeUser:
		return workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_USER

	case domain.AssigneeTypeAgent:
		return workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_AGENT

	default:
		return workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_UNSPECIFIED
	}
}

// AssignmentRole maps a domain assignment role to Protobuf.
func AssignmentRole(value domain.AssignmentRole) workv1.AssignmentRole {
	switch value {
	case domain.AssignmentRolePrimary:
		return workv1.AssignmentRole_ASSIGNMENT_ROLE_PRIMARY

	case domain.AssignmentRoleCollaborator:
		return workv1.AssignmentRole_ASSIGNMENT_ROLE_COLLABORATOR

	case domain.AssignmentRoleReviewer:
		return workv1.AssignmentRole_ASSIGNMENT_ROLE_REVIEWER

	default:
		return workv1.AssignmentRole_ASSIGNMENT_ROLE_UNSPECIFIED
	}
}

func protoTimestamp(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value)
}

func protoTimestampPointer(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}

	return timestamppb.New(*value)
}

func optionalString(value *string) *string {
	return value
}
