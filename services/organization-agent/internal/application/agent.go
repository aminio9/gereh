package application

import (
	"context"
	"fmt"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/aminio9/gereh/services/organization-agent/internal/ports"
	"github.com/aminio9/gereh/services/organization-agent/internal/protoutil"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateAgentInput is the input to agent creation.
type CreateAgentInput struct {
	ActorUserID string
	TenantID    string
	CompanyID   string

	Slug        string
	DisplayName string
	RoleTitle   string
	Objective   string

	ManagerAgentID *string

	ExecutionProfile domain.ExecutionProfile
	AutonomyLevel    domain.AutonomyLevel

	Capabilities  []string
	Configuration map[string]any
}

// UpdateAgentInput is the input to an agent update.
type UpdateAgentInput struct {
	ActorUserID     string
	TenantID        string
	AgentID         string
	ExpectedVersion int64

	DisplayName      *string
	RoleTitle        *string
	Objective        *string
	ExecutionProfile *domain.ExecutionProfile
	AutonomyLevel    *domain.AutonomyLevel
	Capabilities     *[]string
	Configuration    map[string]any
}

// SetAgentManagerInput reassigns an agent's manager.
type SetAgentManagerInput struct {
	ActorUserID     string
	TenantID        string
	AgentID         string
	ExpectedVersion int64

	// ManagerAgentID nil clears the manager and makes the agent a root.
	ManagerAgentID *string
}

// LifecycleInput is the input to pause, resume, and delete operations.
type LifecycleInput struct {
	ActorUserID     string
	TenantID        string
	AgentID         string
	ExpectedVersion int64
}

// CreateAgent validates, authorizes, and commits a new agent.
func (service *Service) CreateAgent(
	ctx context.Context,
	input CreateAgentInput,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"company_id":    input.CompanyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if input.ManagerAgentID != nil {
		if err := validateUUID(
			"manager_agent_id",
			*input.ManagerAgentID,
		); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_AGENT_CREATE,
	); err != nil {
		return domain.Agent{}, err
	}

	slug, err := normalizeSlug(input.Slug)
	if err != nil {
		return domain.Agent{}, err
	}

	displayName, err := boundedText(
		"display_name",
		input.DisplayName,
		1,
		120,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	roleTitle, err := boundedText(
		"role_title",
		input.RoleTitle,
		1,
		120,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	objective, err := boundedText(
		"objective",
		input.Objective,
		1,
		4000,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	executionProfile, err := normalizeExecutionProfile(
		input.ExecutionProfile,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	autonomyLevel, err := normalizeAutonomyLevel(
		input.AutonomyLevel,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	capabilities, err := normalizeCapabilities(
		input.Capabilities,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if err := validateConfiguration(
		input.Configuration,
	); err != nil {
		return domain.Agent{}, err
	}

	agentID, err := uuid.NewV7()
	if err != nil {
		return domain.Agent{}, fmt.Errorf(
			"generate agent ID: %w",
			err,
		)
	}

	now := service.now().UTC()

	agent := domain.Agent{
		TenantID:         input.TenantID,
		CompanyID:        input.CompanyID,
		ID:               agentID.String(),
		Slug:             slug,
		DisplayName:      displayName,
		RoleTitle:        roleTitle,
		Objective:        objective,
		ManagerAgentID:   input.ManagerAgentID,
		Status:           domain.AgentStatusDraft,
		ExecutionProfile: executionProfile,
		AutonomyLevel:    autonomyLevel,
		Capabilities:     capabilities,
		Configuration:    input.Configuration,
		Version:          1,
		CreatedByUserID:  input.ActorUserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	event, err := newOutboxEvent(
		ctx,
		service.config.AgentEventTopic,
		agent.ID,
		"agent.created",
		agent.TenantID,
		"agent",
		agent.ID,
		agent.Version,
		&organizationv1.AgentCreated{
			Agent: protoutil.Agent(agent),
		},
		now,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return service.repository.CreateAgent(
		ctx,
		ports.CreateAgentParams{
			ActorUserID: input.ActorUserID,
			Agent:       agent,
			Event:       event,
		},
	)
}

// GetAgent returns one agent by identity.
func (service *Service) GetAgent(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	agentID string,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"agent_id":      agentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_AGENT_READ,
	); err != nil {
		return domain.Agent{}, err
	}

	return service.repository.GetAgent(
		ctx,
		actorUserID,
		tenantID,
		agentID,
	)
}

// ListAgents returns a paginated agent page for one company.
func (service *Service) ListAgents(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
	pageSize int32,
	pageToken string,
	includeDeleted bool,
) ([]domain.Agent, string, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, "", err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_AGENT_READ,
	); err != nil {
		return nil, "", err
	}

	limit := normalizePageSize(pageSize)

	cursorValue, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	var cursor *ports.AgentCursor

	if cursorValue != "" {
		if err := validateUUID(
			"page_token",
			cursorValue,
		); err != nil {
			return nil, "", err
		}

		cursor = &ports.AgentCursor{
			AgentID: cursorValue,
		}
	}

	agents, err := service.repository.ListAgents(
		ctx,
		actorUserID,
		tenantID,
		companyID,
		limit,
		cursor,
		includeDeleted,
	)
	if err != nil {
		return nil, "", err
	}

	nextToken := ""

	if len(agents) == limit && len(agents) > 0 {
		nextToken = encodePageToken(
			agents[len(agents)-1].ID,
		)
	}

	return agents, nextToken, nil
}

// UpdateAgent validates and commits a versioned agent update.
func (service *Service) UpdateAgent(
	ctx context.Context,
	input UpdateAgentInput,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"agent_id":      input.AgentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_AGENT_UPDATE,
	); err != nil {
		return domain.Agent{}, err
	}

	current, err := service.repository.GetAgent(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.AgentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if current.Status == domain.AgentStatusDeleted {
		return domain.Agent{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	if input.DisplayName != nil {
		displayName, err := boundedText(
			"display_name",
			*input.DisplayName,
			1,
			120,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.DisplayName = displayName
	}

	if input.RoleTitle != nil {
		roleTitle, err := boundedText(
			"role_title",
			*input.RoleTitle,
			1,
			120,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.RoleTitle = roleTitle
	}

	if input.Objective != nil {
		objective, err := boundedText(
			"objective",
			*input.Objective,
			1,
			4000,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.Objective = objective
	}

	if input.ExecutionProfile != nil {
		profile, err := normalizeExecutionProfile(
			*input.ExecutionProfile,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.ExecutionProfile = profile
	}

	if input.AutonomyLevel != nil {
		level, err := normalizeAutonomyLevel(
			*input.AutonomyLevel,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.AutonomyLevel = level
	}

	if input.Capabilities != nil {
		capabilities, err := normalizeCapabilities(
			*input.Capabilities,
		)
		if err != nil {
			return domain.Agent{}, err
		}

		current.Capabilities = capabilities
	}

	if input.Configuration != nil {
		if err := validateConfiguration(
			input.Configuration,
		); err != nil {
			return domain.Agent{}, err
		}

		current.Configuration = input.Configuration
	}

	current.Version++
	current.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.AgentEventTopic,
		current.ID,
		"agent.updated",
		current.TenantID,
		"agent",
		current.ID,
		current.Version,
		&organizationv1.AgentUpdated{
			Agent:           protoutil.Agent(current),
			UpdatedByUserId: input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return service.repository.UpdateAgent(
		ctx,
		ports.UpdateAgentParams{
			ActorUserID:     input.ActorUserID,
			Agent:           current,
			ExpectedVersion: input.ExpectedVersion,
			ChangeKind:      "updated",
			Event:           event,
		},
	)
}

// SetAgentManager reassigns an agent's manager after cycle detection.
func (service *Service) SetAgentManager(
	ctx context.Context,
	input SetAgentManagerInput,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"agent_id":      input.AgentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if input.ManagerAgentID != nil {
		if err := validateUUID(
			"manager_agent_id",
			*input.ManagerAgentID,
		); err != nil {
			return domain.Agent{}, err
		}

		if *input.ManagerAgentID == input.AgentID {
			return domain.Agent{}, domain.ErrHierarchyCycle
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_AGENT_HIERARCHY_MANAGE,
	); err != nil {
		return domain.Agent{}, err
	}

	current, err := service.repository.GetAgent(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.AgentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if current.Status == domain.AgentStatusDeleted {
		return domain.Agent{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	previousManager := current.ManagerAgentID
	current.ManagerAgentID = input.ManagerAgentID
	current.Version++
	current.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.AgentEventTopic,
		current.ID,
		"agent.manager_changed",
		current.TenantID,
		"agent",
		current.ID,
		current.Version,
		&organizationv1.AgentManagerChanged{
			Agent:                  protoutil.Agent(current),
			PreviousManagerAgentId: previousManager,
			ChangedByUserId:        input.ActorUserID,
		},
		now,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return service.repository.SetAgentManager(
		ctx,
		ports.UpdateAgentParams{
			ActorUserID:     input.ActorUserID,
			Agent:           current,
			ExpectedVersion: input.ExpectedVersion,
			ChangeKind:      "manager_changed",
			Event:           event,
		},
	)
}

// PauseAgent pauses an agent that is ready, degraded, or failed.
func (service *Service) PauseAgent(
	ctx context.Context,
	input LifecycleInput,
) (domain.Agent, error) {
	return service.changeAgentStatus(
		ctx,
		input,
		domain.AgentStatusPaused,
		"paused",
		"agent.paused",
	)
}

// ResumeAgent resumes a paused agent.
func (service *Service) ResumeAgent(
	ctx context.Context,
	input LifecycleInput,
) (domain.Agent, error) {
	return service.changeAgentStatus(
		ctx,
		input,
		domain.AgentStatusReady,
		"resumed",
		"agent.resumed",
	)
}

// DeleteAgent soft-deletes an agent with no direct reports.
func (service *Service) DeleteAgent(
	ctx context.Context,
	input LifecycleInput,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"agent_id":      input.AgentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_AGENT_DELETE,
	); err != nil {
		return domain.Agent{}, err
	}

	current, err := service.repository.GetAgent(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.AgentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if !canDelete(current.Status) {
		return domain.Agent{}, domain.ErrInvalidTransition
	}

	now := service.now().UTC()

	current.Status = domain.AgentStatusDeleted
	current.DeletedAt = &now
	current.Version++
	current.UpdatedAt = now

	event, err := newOutboxEvent(
		ctx,
		service.config.AgentEventTopic,
		current.ID,
		"agent.deleted",
		current.TenantID,
		"agent",
		current.ID,
		current.Version,
		&organizationv1.AgentDeleted{
			Agent:           protoutil.Agent(current),
			DeletedByUserId: input.ActorUserID,
			DeletedAt:       timestamppb.New(now),
		},
		now,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return service.repository.DeleteAgent(
		ctx,
		ports.UpdateAgentParams{
			ActorUserID:     input.ActorUserID,
			Agent:           current,
			ExpectedVersion: input.ExpectedVersion,
			ChangeKind:      "deleted",
			Event:           event,
		},
	)
}

// GetHierarchy returns the reporting tree of a company.
func (service *Service) GetHierarchy(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) ([]domain.HierarchyNode, error) {
	for name, value := range map[string]string{
		"actor_user_id": actorUserID,
		"tenant_id":     tenantID,
		"company_id":    companyID,
	} {
		if err := validateUUID(name, value); err != nil {
			return nil, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		actorUserID,
		tenantID,
		tenantv1.Permission_PERMISSION_AGENT_READ,
	); err != nil {
		return nil, err
	}

	return service.repository.GetHierarchy(
		ctx,
		actorUserID,
		tenantID,
		companyID,
	)
}

func (service *Service) changeAgentStatus(
	ctx context.Context,
	input LifecycleInput,
	target domain.AgentStatus,
	changeKind string,
	eventType string,
) (domain.Agent, error) {
	for name, value := range map[string]string{
		"actor_user_id": input.ActorUserID,
		"tenant_id":     input.TenantID,
		"agent_id":      input.AgentID,
	} {
		if err := validateUUID(name, value); err != nil {
			return domain.Agent{}, err
		}
	}

	if err := service.authorizer.Require(
		ctx,
		input.ActorUserID,
		input.TenantID,
		tenantv1.Permission_PERMISSION_AGENT_LIFECYCLE_MANAGE,
	); err != nil {
		return domain.Agent{}, err
	}

	current, err := service.repository.GetAgent(
		ctx,
		input.ActorUserID,
		input.TenantID,
		input.AgentID,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	if target == domain.AgentStatusPaused &&
		!canPause(current.Status) {
		return domain.Agent{}, domain.ErrInvalidTransition
	}

	if target == domain.AgentStatusReady &&
		!canResume(current.Status) {
		return domain.Agent{}, domain.ErrInvalidTransition
	}

	previousStatus := current.Status
	current.Status = target
	current.Version++
	current.UpdatedAt = service.now().UTC()

	event, err := newOutboxEvent(
		ctx,
		service.config.AgentEventTopic,
		current.ID,
		eventType,
		current.TenantID,
		"agent",
		current.ID,
		current.Version,
		&organizationv1.AgentLifecycleChanged{
			Agent:           protoutil.Agent(current),
			PreviousStatus:  protoutil.AgentStatus(previousStatus),
			ChangedByUserId: input.ActorUserID,
		},
		current.UpdatedAt,
	)
	if err != nil {
		return domain.Agent{}, err
	}

	return service.repository.ChangeAgentStatus(
		ctx,
		ports.UpdateAgentParams{
			ActorUserID:     input.ActorUserID,
			Agent:           current,
			ExpectedVersion: input.ExpectedVersion,
			ChangeKind:      changeKind,
			Event:           event,
		},
	)
}

func normalizeExecutionProfile(
	value domain.ExecutionProfile,
) (domain.ExecutionProfile, error) {
	if value == "" {
		return domain.ExecutionProfileBalanced, nil
	}

	switch value {
	case domain.ExecutionProfileBalanced,
		domain.ExecutionProfilePersistent,
		domain.ExecutionProfileTechnicalWorker:
		return value, nil

	default:
		return "", fmt.Errorf(
			"%w: unsupported execution profile",
			domain.ErrInvalidArgument,
		)
	}
}

func normalizeAutonomyLevel(
	value domain.AutonomyLevel,
) (domain.AutonomyLevel, error) {
	if value == "" {
		return domain.AutonomyApprovalRequired, nil
	}

	switch value {
	case domain.AutonomyObserveOnly,
		domain.AutonomySuggest,
		domain.AutonomyApprovalRequired,
		domain.AutonomyPolicyBounded:
		return value, nil

	default:
		return "", fmt.Errorf(
			"%w: unsupported autonomy level",
			domain.ErrInvalidArgument,
		)
	}
}

func canPause(status domain.AgentStatus) bool {
	switch status {
	case domain.AgentStatusReady,
		domain.AgentStatusDegraded,
		domain.AgentStatusFailed:
		return true

	default:
		return false
	}
}

func canResume(status domain.AgentStatus) bool {
	return status == domain.AgentStatusPaused
}

func canDelete(status domain.AgentStatus) bool {
	switch status {
	case domain.AgentStatusDraft,
		domain.AgentStatusPaused,
		domain.AgentStatusFailed:
		return true

	default:
		return false
	}
}
