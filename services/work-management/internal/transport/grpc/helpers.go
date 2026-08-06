package grpc

import (
	"fmt"
	"time"

	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func protoTimestampValue(
	value *timestamppb.Timestamp,
) *time.Time {
	if value == nil {
		return nil
	}

	result := value.AsTime()
	return &result
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
