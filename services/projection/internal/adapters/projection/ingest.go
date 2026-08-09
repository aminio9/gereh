package projection

import (
	"context"
	"fmt"
	"strings"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	workv1 "github.com/aminio9/gereh/gen/go/gereh/work/v1"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
	"google.golang.org/protobuf/proto"
)

// decodeEvent unmarshals one domain event payload and returns the projection
// apply function. Unknown event types are safely ignored so that the read
// model tolerates forward schema drift.
func decodeEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	switch eventType {
	// Tenant lifecycle
	case "tenant.created", "tenant.activated":
		return decodeTenantContextEvent(eventType, payload, meta, now)

	case "tenant.onboarding_failed",
		"tenant.updated",
		"tenant.archived":
		return decodeTenantEvent(eventType, payload, meta, now)

	// Organization
	case "company.created",
		"company.updated",
		"company.archived":
		return decodeCompanyEvent(eventType, payload, meta, now)

	case "agent.created",
		"agent.updated",
		"agent.manager_changed",
		"agent.paused",
		"agent.resumed":
		return decodeAgentEvent(eventType, payload, meta, now)

	case "agent.deleted":
		return decodeAgentDeletedEvent(payload, meta, now)

	// Work
	case "goal.created",
		"goal.updated",
		"goal.status_changed":
		return decodeGoalEvent(eventType, payload, meta, now)

	case "project.created",
		"project.updated",
		"project.status_changed":
		return decodeProjectEvent(eventType, payload, meta, now)

	case "task.created",
		"task.updated",
		"task.status_changed":
		return decodeTaskEvent(eventType, payload, meta, now)

	case "task.dependency_added":
		return decodeDependencyAddedEvent(payload, meta, now)

	case "task.dependency_removed":
		return decodeDependencyRemovedEvent(eventType, payload, meta, now)

	case "task.assigned":
		return decodeAssignedEvent(payload, meta, now)

	case "task.unassigned":
		return decodeUnassignedEvent(eventType, payload, meta, now)

	case "task.comment_added",
		"task.comment_updated",
		"task.comment_deleted":
		return decodeCommentEvent(eventType, payload, meta, now)

	case "task.artifact_added":
		return decodeArtifactAddedEvent(payload, meta, now)

	case "task.artifact_deleted":
		return decodeActivityOnlyEvent(eventType, payload, meta, now)

	case "task.checklist_changed",
		"task.schedule_changed":
		return decodeActivityOnlyEvent(eventType, payload, meta, now)

	default:
		return nil, nil
	}
}

// requireTenantMatch enforces the invariant that the Kafka envelope's
// tenant_id equals the payload's tenant_id. A mismatch means the record is
// corrupt or malicious: the consumer must fail, never project.
func requireTenantMatch(
	meta domain.EventMeta,
	payloadTenantID string,
) error {
	if strings.TrimSpace(
		payloadTenantID,
	) != strings.TrimSpace(
		meta.TenantID,
	) {
		return fmt.Errorf(
			"%s payload tenant %q does not match envelope tenant %q",
			meta.EventType,
			payloadTenantID,
			meta.TenantID,
		)
	}

	return nil
}

func decodeTenantContextEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	holder, err := decodeTenantContextHolder(
		eventType,
		payload,
	)
	if err != nil {
		return nil, err
	}

	contextValue := holder.GetContext()
	if contextValue == nil ||
		contextValue.GetTenant() == nil {
		return nil, fmt.Errorf(
			"tenant event %q carried no tenant",
			eventType,
		)
	}

	tenant := contextValue.GetTenant()

	if err := requireTenantMatch(
		meta,
		tenant.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := tenantSnapshot(tenant, meta, now)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertTenant(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return nil
	}, nil
}

func decodeTenantEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	holder, err := decodeTenantHolder(
		eventType,
		payload,
	)
	if err != nil {
		return nil, err
	}

	tenant := holder.GetTenant()
	if tenant == nil {
		return nil, fmt.Errorf(
			"tenant event %q carried no tenant",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		tenant.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := tenantSnapshot(tenant, meta, now)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		return transaction.UpsertTenant(
			ctx,
			projection,
		)
	}, nil
}

func decodeCompanyEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value organizationv1.CompanyCreated

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	company := value.GetCompany()
	if company == nil {
		return nil, fmt.Errorf(
			"company event %q carried no company",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		company.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := companySnapshot(company, meta, now)

	document := companySearchDocument(
		company,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertCompany(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.UpsertSearchDocument(
			ctx,
			document,
		)
	}, nil
}

func decodeAgentEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value organizationv1.AgentCreated

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	agent := value.GetAgent()
	if agent == nil {
		return nil, fmt.Errorf(
			"agent event %q carried no agent",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		agent.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := agentSnapshot(agent, meta, now)

	document := agentSearchDocument(
		agent,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertAgent(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.UpsertSearchDocument(
			ctx,
			document,
		)
	}, nil
}

func decodeAgentDeletedEvent(
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value organizationv1.AgentDeleted

	if err := unmarshalTyped(
		"agent.deleted",
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	agent := value.GetAgent()
	if agent == nil {
		return nil, fmt.Errorf(
			"agent.deleted carried no agent",
		)
	}

	if err := requireTenantMatch(
		meta,
		agent.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := agentSnapshot(agent, meta, now)
	projection.Status = "deleted"

	document := agentSearchDocument(
		agent,
		meta,
		now,
	)
	document.Deleted = true

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertAgent(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.UpsertSearchDocument(
			ctx,
			document,
		)
	}, nil
}

func decodeGoalEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.GoalCreated

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	goal := value.GetGoal()
	if goal == nil {
		return nil, fmt.Errorf(
			"goal event %q carried no goal",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		goal.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := goalSnapshot(goal, meta, now)

	document := goalSearchDocument(
		goal,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertGoal(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.UpsertSearchDocument(
			ctx,
			document,
		)
	}, nil
}

func decodeProjectEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.ProjectCreated

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	project := value.GetProject()
	if project == nil {
		return nil, fmt.Errorf(
			"project event %q carried no project",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		project.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := projectSnapshot(project, meta, now)

	document := projectSearchDocument(
		project,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertProject(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.UpsertSearchDocument(
			ctx,
			document,
		)
	}, nil
}

func decodeTaskEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskCreated

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	task := value.GetTask()
	if task == nil {
		return nil, fmt.Errorf(
			"task event %q carried no task",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		task.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := taskSnapshot(task, meta, now)

	document := taskSearchDocument(
		task,
		meta,
		now,
	)

	summary := taskEventSummary(eventType, task)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertTask(
			ctx,
			projection,
		); err != nil {
			return err
		}

		if err := transaction.UpsertSearchDocument(
			ctx,
			document,
		); err != nil {
			return err
		}

		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				task.GetCompanyId(),
				task.GetProjectId(),
				task.GetTaskId(),
				summary,
			),
		)
	}, nil
}

func decodeDependencyAddedEvent(
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskDependencyAdded

	if err := unmarshalTyped(
		"task.dependency_added",
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	dependency := value.GetDependency()
	if dependency == nil {
		return nil, fmt.Errorf(
			"task.dependency_added carried no dependency",
		)
	}

	if err := requireTenantMatch(
		meta,
		dependency.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := dependencySnapshot(
		dependency,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertDependency(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				"",
				dependency.GetProjectId(),
				dependency.GetTaskId(),
				"task dependency added",
			),
		)
	}, nil
}

func decodeDependencyRemovedEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskDependencyRemoved

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	dependency := value.GetDependency()
	if dependency == nil {
		return nil, fmt.Errorf(
			"task.dependency_removed carried no dependency",
		)
	}

	if err := requireTenantMatch(
		meta,
		dependency.GetTenantId(),
	); err != nil {
		return nil, err
	}

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.DeleteDependency(
			ctx,
			meta.TenantID,
			dependency.GetTaskId(),
			dependency.GetDependsOnTaskId(),
		); err != nil {
			return err
		}

		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				"",
				dependency.GetProjectId(),
				dependency.GetTaskId(),
				"task dependency removed",
			),
		)
	}, nil
}

func decodeAssignedEvent(
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskAssigned

	if err := unmarshalTyped(
		"task.assigned",
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	assignment := value.GetAssignment()
	if assignment == nil {
		return nil, fmt.Errorf(
			"task.assigned carried no assignment",
		)
	}

	if err := requireTenantMatch(
		meta,
		assignment.GetTenantId(),
	); err != nil {
		return nil, err
	}

	projection := assignmentSnapshot(
		assignment,
		meta,
		now,
	)

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.UpsertAssignment(
			ctx,
			projection,
		); err != nil {
			return err
		}

		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				"",
				"",
				assignment.GetTaskId(),
				"task assigned",
			),
		)
	}, nil
}

func decodeUnassignedEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskUnassigned

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	assignment := value.GetAssignment()
	if assignment == nil {
		return nil, fmt.Errorf(
			"task.unassigned carried no assignment",
		)
	}

	if err := requireTenantMatch(
		meta,
		assignment.GetTenantId(),
	); err != nil {
		return nil, err
	}

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		if err := transaction.DeleteAssignment(
			ctx,
			meta.TenantID,
			assignment.GetAssignmentId(),
		); err != nil {
			return err
		}

		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				"",
				"",
				assignment.GetTaskId(),
				"task unassigned",
			),
		)
	}, nil
}

func decodeCommentEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskCommentAdded

	if err := unmarshalTyped(
		eventType,
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	comment := value.GetComment()
	if comment == nil {
		return nil, fmt.Errorf(
			"comment event %q carried no comment",
			eventType,
		)
	}

	if err := requireTenantMatch(
		meta,
		comment.GetTenantId(),
	); err != nil {
		return nil, err
	}

	var actorType *string
	var actorID *string

	if comment.GetAuthorType() ==
		workv1.CommentAuthorType_COMMENT_AUTHOR_TYPE_AGENT {
		actorType = stringPointer("agent")

		if value := comment.GetAuthorAgentId(); value != "" {
			actorID = stringPointer(value)
		}
	} else {
		actorType = stringPointer("user")

		if value := comment.GetAuthorUserId(); value != "" {
			actorID = stringPointer(value)
		}
	}

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		activity := taskActivity(
			meta,
			now,
			"",
			"",
			comment.GetTaskId(),
			"task comment "+commentEventKind(eventType),
		)
		activity.ActorType = actorType
		activity.ActorID = actorID

		return transaction.AppendActivity(
			ctx,
			activity,
		)
	}, nil
}

func decodeArtifactAddedEvent(
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var value workv1.TaskArtifactAdded

	if err := unmarshalTyped(
		"task.artifact_added",
		payload,
		&value,
	); err != nil {
		return nil, err
	}

	artifact := value.GetArtifact()
	if artifact == nil {
		return nil, fmt.Errorf(
			"task.artifact_added carried no artifact",
		)
	}

	if err := requireTenantMatch(
		meta,
		artifact.GetTenantId(),
	); err != nil {
		return nil, err
	}

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				artifact.GetCompanyId(),
				"",
				artifact.GetTaskId(),
				"task artifact added",
			),
		)
	}, nil
}

func decodeActivityOnlyEvent(
	eventType string,
	payload []byte,
	meta domain.EventMeta,
	now time.Time,
) (ports.ApplyFunc, error) {
	var taskID *string
	var payloadTenantID string

	switch eventType {
	case "task.artifact_deleted":
		var value workv1.TaskArtifactDeleted

		if err := unmarshalTyped(
			eventType,
			payload,
			&value,
		); err != nil {
			return nil, err
		}

		artifact := value.GetArtifact()
		if artifact != nil {
			taskID = stringPointer(
				artifact.GetTaskId(),
			)
			payloadTenantID = artifact.GetTenantId()
		}

	case "task.checklist_changed":
		var value workv1.TaskChecklistChanged

		if err := unmarshalTyped(
			eventType,
			payload,
			&value,
		); err != nil {
			return nil, err
		}

		item := value.GetItem()
		if item != nil {
			taskID = stringPointer(
				item.GetTaskId(),
			)
			payloadTenantID = item.GetTenantId()
		}

	case "task.schedule_changed":
		var value workv1.TaskScheduleChanged

		if err := unmarshalTyped(
			eventType,
			payload,
			&value,
		); err != nil {
			return nil, err
		}

		schedule := value.GetSchedule()
		if schedule != nil {
			taskID = stringPointer(
				schedule.GetTaskId(),
			)
			payloadTenantID = schedule.GetTenantId()
		}
	}

	if err := requireTenantMatch(
		meta,
		payloadTenantID,
	); err != nil {
		return nil, err
	}

	return func(
		ctx context.Context,
		transaction ports.ProjectionTransaction,
	) error {
		return transaction.AppendActivity(
			ctx,
			taskActivity(
				meta,
				now,
				"",
				"",
				optionalValue(taskID),
				"task "+
					taskActivityEventKind(
						eventType,
					),
			),
		)
	}, nil
}

func commentEventKind(
	eventType string,
) string {
	switch eventType {
	case "task.comment_updated":
		return "updated"

	case "task.comment_deleted":
		return "deleted"

	default:
		return "added"
	}
}

func taskActivityEventKind(
	eventType string,
) string {
	switch eventType {
	case "task.artifact_deleted":
		return "artifact deleted"

	case "task.checklist_changed":
		return "checklist changed"

	case "task.schedule_changed":
		return "schedule changed"

	default:
		return "changed"
	}
}

func taskActivity(
	meta domain.EventMeta,
	now time.Time,
	companyID string,
	projectID string,
	taskID string,
	summary string,
) domain.Activity {
	return domain.Activity{
		TenantID:  meta.TenantID,
		EventID:   meta.EventID,
		EventType: meta.EventType,

		CompanyID: optionalTaskID(companyID),
		ProjectID: optionalTaskID(projectID),
		TaskID:    optionalTaskID(taskID),

		Summary: summary,

		OccurredAt:  meta.OccurredAt,
		ProjectedAt: now,
	}
}

func optionalTaskID(
	value string,
) *string {
	if value == "" {
		return nil
	}

	return stringPointer(value)
}

func optionalValue(
	value *string,
) string {
	if value == nil {
		return ""
	}

	return *value
}

func stringPointer(value string) *string {
	cloned := value

	return &cloned
}

func unmarshalTyped(
	eventType string,
	payload []byte,
	value proto.Message,
) error {
	if err := proto.Unmarshal(payload, value); err != nil {
		return fmt.Errorf(
			"unmarshal %q payload: %w",
			eventType,
			err,
		)
	}

	return nil
}

// TenantContextHolder accommodates event payloads that wrap a TenantContext.
type tenantContextGetter interface {
	GetContext() *tenantv1.TenantContext
}

func decodeTenantContextHolder(
	eventType string,
	payload []byte,
) (tenantContextGetter, error) {
	switch eventType {
	case "tenant.created":
		var created tenantv1.TenantCreated

		if err := unmarshalTyped(
			eventType,
			payload,
			&created,
		); err != nil {
			return nil, err
		}

		return &created, nil

	case "tenant.activated":
		var activated tenantv1.TenantActivated

		if err := unmarshalTyped(
			eventType,
			payload,
			&activated,
		); err != nil {
			return nil, err
		}

		return &activated, nil

	default:
		return nil, fmt.Errorf(
			"unsupported tenant context event %q",
			eventType,
		)
	}
}

// TenantHolder accommodates event payloads that carry a Tenant directly.
type tenantGetter interface {
	GetTenant() *tenantv1.Tenant
}

func decodeTenantHolder(
	eventType string,
	payload []byte,
) (tenantGetter, error) {
	switch eventType {
	case "tenant.onboarding_failed":
		var failed tenantv1.TenantOnboardingFailed

		if err := unmarshalTyped(
			eventType,
			payload,
			&failed,
		); err != nil {
			return nil, err
		}

		return &failed, nil

	case "tenant.updated":
		var updated tenantv1.TenantUpdated

		if err := unmarshalTyped(
			eventType,
			payload,
			&updated,
		); err != nil {
			return nil, err
		}

		return &updated, nil

	case "tenant.archived":
		var archived tenantv1.TenantArchived

		if err := unmarshalTyped(
			eventType,
			payload,
			&archived,
		); err != nil {
			return nil, err
		}

		return &archived, nil

	default:
		return nil, fmt.Errorf(
			"unsupported tenant event %q",
			eventType,
		)
	}
}
