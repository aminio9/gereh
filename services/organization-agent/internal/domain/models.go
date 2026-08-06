// Package domain contains Company and Agent Service business types.
package domain

import (
	"encoding/json"
	"time"
)

// CompanyStatus is the lifecycle state of a company.
type CompanyStatus string

// CompanyStatus values.
const (
	CompanyStatusActive   CompanyStatus = "active"
	CompanyStatusArchived CompanyStatus = "archived"
)

// AgentStatus is the lifecycle state of an agent.
type AgentStatus string

// AgentStatus values.
const (
	AgentStatusDraft              AgentStatus = "draft"
	AgentStatusProvisioning       AgentStatus = "provisioning"
	AgentStatusConfiguringRuntime AgentStatus = "configuring_runtime"
	AgentStatusHealthChecking     AgentStatus = "health_checking"
	AgentStatusReady              AgentStatus = "ready"
	AgentStatusDegraded           AgentStatus = "degraded"
	AgentStatusPaused             AgentStatus = "paused"
	AgentStatusFailed             AgentStatus = "failed"
	AgentStatusDeleting           AgentStatus = "deleting"
	AgentStatusDeleted            AgentStatus = "deleted"
)

// ExecutionProfile is a provider-neutral agent execution shape.
type ExecutionProfile string

// ExecutionProfile values.
const (
	ExecutionProfileBalanced        ExecutionProfile = "balanced"
	ExecutionProfilePersistent      ExecutionProfile = "persistent"
	ExecutionProfileTechnicalWorker ExecutionProfile = "technical_worker"
)

// AutonomyLevel controls how much decision authority an agent has.
type AutonomyLevel string

// AutonomyLevel values.
const (
	AutonomyObserveOnly      AutonomyLevel = "observe_only"
	AutonomySuggest          AutonomyLevel = "suggest"
	AutonomyApprovalRequired AutonomyLevel = "approval_required"
	AutonomyPolicyBounded    AutonomyLevel = "policy_bounded"
)

// Company is a tenant-owned organizational unit.
type Company struct {
	TenantID        string
	ID              string
	Slug            string
	DisplayName     string
	Description     string
	Status          CompanyStatus
	IsDefault       bool
	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

// Agent is a provider-neutral agent business identity.
type Agent struct {
	TenantID    string
	CompanyID   string
	ID          string
	Slug        string
	DisplayName string
	RoleTitle   string
	Objective   string

	ManagerAgentID *string

	Status           AgentStatus
	ExecutionProfile ExecutionProfile
	AutonomyLevel    AutonomyLevel

	Capabilities  []string
	Configuration map[string]any

	Version         int64
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// AgentRevision is an immutable snapshot of an agent after a change.
type AgentRevision struct {
	TenantID    string
	AgentID     string
	Version     int64
	ChangeKind  string
	Snapshot    json.RawMessage
	ActorUserID string
	OccurredAt  time.Time
}

// HierarchyNode is one agent with its reporting depth in a company tree.
type HierarchyNode struct {
	Agent Agent
	Depth int32
}
