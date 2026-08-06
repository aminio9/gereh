package application

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/google/uuid"
)

var sha256Pattern = regexp.MustCompile(
	`^[a-f0-9]{64}$`,
)

func validateUUID(
	name string,
	value string,
) error {
	if _, err := uuid.Parse(
		strings.TrimSpace(value),
	); err != nil {
		return fmt.Errorf(
			"%w: %s must be a UUID",
			domain.ErrInvalidArgument,
			name,
		)
	}

	return nil
}

func boundedText(
	name string,
	value string,
	minimum int,
	maximum int,
) (string, error) {
	value = strings.TrimSpace(value)
	length := len([]rune(value))

	if length < minimum ||
		length > maximum {
		return "", fmt.Errorf(
			"%w: %s must contain %d-%d characters",
			domain.ErrInvalidArgument,
			name,
			minimum,
			maximum,
		)
	}

	return value, nil
}

func validateArtifact(
	objectKey string,
	fileName string,
	contentType string,
	sizeBytes int64,
	sha256 string,
) error {
	if strings.TrimSpace(objectKey) == "" ||
		len(objectKey) > 1024 {
		return fmt.Errorf(
			"%w: object_key must contain 1-1024 characters",
			domain.ErrInvalidArgument,
		)
	}

	if _, err := boundedText(
		"file_name",
		fileName,
		1,
		512,
	); err != nil {
		return err
	}

	if _, err := boundedText(
		"content_type",
		contentType,
		1,
		255,
	); err != nil {
		return err
	}

	if sizeBytes < 0 {
		return fmt.Errorf(
			"%w: size_bytes must not be negative",
			domain.ErrInvalidArgument,
		)
	}

	if !sha256Pattern.MatchString(
		strings.ToLower(strings.TrimSpace(sha256)),
	) {
		return fmt.Errorf(
			"%w: sha256 must be a 64-character hex digest",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func validateSchedule(
	notBefore *time.Time,
	dueAt *time.Time,
	timezone string,
) error {
	if _, err := boundedText(
		"timezone",
		timezone,
		1,
		64,
	); err != nil {
		return err
	}

	if notBefore != nil && dueAt != nil &&
		notBefore.After(*dueAt) {
		return fmt.Errorf(
			"%w: not_before must not be after due_at",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func domainGoalStatus(
	value workv1.GoalStatus,
) (domain.GoalStatus, error) {
	switch value {
	case workv1.GoalStatus_GOAL_STATUS_ACTIVE:
		return domain.GoalStatusActive, nil

	case workv1.GoalStatus_GOAL_STATUS_COMPLETED:
		return domain.GoalStatusCompleted, nil

	case workv1.GoalStatus_GOAL_STATUS_CANCELED:
		return domain.GoalStatusCanceled, nil

	case workv1.GoalStatus_GOAL_STATUS_ARCHIVED:
		return domain.GoalStatusArchived, nil

	default:
		return "", fmt.Errorf(
			"%w: unknown goal status",
			domain.ErrInvalidArgument,
		)
	}
}

func domainProjectStatus(
	value workv1.ProjectStatus,
) (domain.ProjectStatus, error) {
	switch value {
	case workv1.ProjectStatus_PROJECT_STATUS_PLANNED:
		return domain.ProjectStatusPlanned, nil

	case workv1.ProjectStatus_PROJECT_STATUS_ACTIVE:
		return domain.ProjectStatusActive, nil

	case workv1.ProjectStatus_PROJECT_STATUS_ON_HOLD:
		return domain.ProjectStatusOnHold, nil

	case workv1.ProjectStatus_PROJECT_STATUS_COMPLETED:
		return domain.ProjectStatusCompleted, nil

	case workv1.ProjectStatus_PROJECT_STATUS_CANCELED:
		return domain.ProjectStatusCanceled, nil

	case workv1.ProjectStatus_PROJECT_STATUS_ARCHIVED:
		return domain.ProjectStatusArchived, nil

	default:
		return "", fmt.Errorf(
			"%w: unknown project status",
			domain.ErrInvalidArgument,
		)
	}
}

func domainTaskStatus(
	value workv1.TaskStatus,
) (domain.TaskStatus, error) {
	switch value {
	case workv1.TaskStatus_TASK_STATUS_BACKLOG:
		return domain.TaskStatusBacklog, nil

	case workv1.TaskStatus_TASK_STATUS_READY:
		return domain.TaskStatusReady, nil

	case workv1.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return domain.TaskStatusInProgress, nil

	case workv1.TaskStatus_TASK_STATUS_WAITING_APPROVAL:
		return domain.TaskStatusWaitingApproval, nil

	case workv1.TaskStatus_TASK_STATUS_COMPLETED:
		return domain.TaskStatusCompleted, nil

	case workv1.TaskStatus_TASK_STATUS_CANCELED:
		return domain.TaskStatusCanceled, nil

	default:
		return "", fmt.Errorf(
			"%w: unknown task status",
			domain.ErrInvalidArgument,
		)
	}
}

func domainTaskPriority(
	value workv1.TaskPriority,
) (domain.TaskPriority, error) {
	switch value {
	case workv1.TaskPriority_TASK_PRIORITY_LOW:
		return domain.TaskPriorityLow, nil

	case workv1.TaskPriority_TASK_PRIORITY_NORMAL:
		return domain.TaskPriorityNormal, nil

	case workv1.TaskPriority_TASK_PRIORITY_HIGH:
		return domain.TaskPriorityHigh, nil

	case workv1.TaskPriority_TASK_PRIORITY_URGENT:
		return domain.TaskPriorityUrgent, nil

	default:
		return "", fmt.Errorf(
			"%w: unknown task priority",
			domain.ErrInvalidArgument,
		)
	}
}
