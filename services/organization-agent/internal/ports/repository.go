package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
)

// CompanyCursor is the pagination position for companies.
type CompanyCursor struct {
	CompanyID string
}

// AgentCursor is the pagination position for agents.
type AgentCursor struct {
	AgentID string
}

// CreateCompanyParams carries a company creation and its committed event.
type CreateCompanyParams struct {
	ActorUserID string
	Company     domain.Company
	Event       domain.OutboxEvent
}

// EnsureDefaultCompanyParams carries the idempotent default-company bootstrap.
type EnsureDefaultCompanyParams struct {
	ServicePrincipalID    string
	OnboardingOperationID string
	Company               domain.Company
	Event                 domain.OutboxEvent
}

// UpdateCompanyParams carries an optimistic-concurrency company update.
type UpdateCompanyParams struct {
	ActorUserID     string
	Company         domain.Company
	ExpectedVersion int64
	Event           domain.OutboxEvent
}

// CreateAgentParams carries an agent creation and its committed event.
type CreateAgentParams struct {
	ActorUserID string
	Agent       domain.Agent
	Event       domain.OutboxEvent
}

// UpdateAgentParams carries an optimistic-concurrency agent mutation.
type UpdateAgentParams struct {
	ActorUserID     string
	Agent           domain.Agent
	ExpectedVersion int64
	ChangeKind      string
	Event           domain.OutboxEvent
}

// Repository is the Company and Agent Service persistence boundary.
type Repository interface {
	CreateCompany(
		context.Context,
		CreateCompanyParams,
	) (domain.Company, error)

	EnsureDefaultCompany(
		context.Context,
		EnsureDefaultCompanyParams,
	) (domain.Company, error)

	GetCompany(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
	) (domain.Company, error)

	ListCompanies(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		limit int,
		cursor *CompanyCursor,
		includeArchived bool,
	) ([]domain.Company, error)

	UpdateCompany(
		context.Context,
		UpdateCompanyParams,
	) (domain.Company, error)

	ArchiveCompany(
		context.Context,
		UpdateCompanyParams,
	) (domain.Company, error)

	CreateAgent(
		context.Context,
		CreateAgentParams,
	) (domain.Agent, error)

	GetAgent(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		agentID string,
	) (domain.Agent, error)

	ListAgents(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
		limit int,
		cursor *AgentCursor,
		includeDeleted bool,
	) ([]domain.Agent, error)

	UpdateAgent(
		context.Context,
		UpdateAgentParams,
	) (domain.Agent, error)

	SetAgentManager(
		context.Context,
		UpdateAgentParams,
	) (domain.Agent, error)

	ChangeAgentStatus(
		context.Context,
		UpdateAgentParams,
	) (domain.Agent, error)

	DeleteAgent(
		context.Context,
		UpdateAgentParams,
	) (domain.Agent, error)

	GetHierarchy(
		ctx context.Context,
		actorUserID string,
		tenantID string,
		companyID string,
	) ([]domain.HierarchyNode, error)

	ClaimOutbox(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]domain.OutboxRecord, error)

	MarkOutboxPublished(
		ctx context.Context,
		outboxID int64,
	) error

	ReleaseOutbox(
		ctx context.Context,
		outboxID int64,
		retryAt time.Time,
		publishError string,
	) error
}
