package domain

import "time"

// ScopeType identifies the scope a policy set applies to.
type ScopeType string

// ScopeType values.
const (
	ScopeTenant  ScopeType = "tenant"
	ScopeCompany ScopeType = "company"
	ScopeAgent   ScopeType = "agent"
)

// PolicyStatus is the lifecycle state of a policy set.
type PolicyStatus string

// PolicyStatus values.
const (
	PolicyStatusDraft    PolicyStatus = "draft"
	PolicyStatusActive   PolicyStatus = "active"
	PolicyStatusArchived PolicyStatus = "archived"
)

// Effect is a policy evaluation outcome.
type Effect string

// Effect values.
const (
	EffectAllow                Effect = "allow"
	EffectDeny                 Effect = "deny"
	EffectRequireApproval      Effect = "require_approval"
	EffectAllowWithConstraints Effect = "allow_with_constraints"
)

// Risk is the risk level of an evaluated action.
type Risk string

// Risk values.
const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

// SubjectType identifies the kind of evaluation subject.
type SubjectType string

// SubjectType values.
const (
	SubjectUser    SubjectType = "user"
	SubjectAgent   SubjectType = "agent"
	SubjectService SubjectType = "service"
)

// Policy is a tenant-scoped policy set with immutable versions.
type Policy struct {
	TenantID string
	ID       string

	ScopeType ScopeType
	ScopeID   *string

	Name        string
	Description string

	Status PolicyStatus

	ActivePolicyVersion *int64
	ResourceVersion     int64

	CreatedByUserID string

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}

// PolicyVersion is an immutable policy version.
type PolicyVersion struct {
	TenantID      string
	PolicyID      string
	PolicyVersion int64

	DefaultEffect Effect
	Rules         []Rule

	Notes           string
	CreatedByUserID string

	CreatedAt   time.Time
	ActivatedAt *time.Time
}

// Rule is one prioritized policy rule.
type Rule struct {
	ID       string
	Priority int32

	Name    string
	Enabled bool

	Effect Effect

	ActionPatterns []string
	ResourceTypes  []string
	RiskLevels     []Risk

	MaximumEstimatedCostMicroUSD *int64

	Condition   string
	Constraints Constraints
	Reason      string
}

// Constraints carries merged evaluation constraints.
type Constraints struct {
	MaxCostMicroUSD    *int64   `json:"max_cost_micro_usd,omitempty"`
	MaxRuntimeSeconds  *int64   `json:"max_runtime_seconds,omitempty"`
	AllowedDomains     []string `json:"allowed_domains,omitempty"`
	AllowedResourceIDs []string `json:"allowed_resource_ids,omitempty"`
	RequireHumanReview bool     `json:"require_human_review,omitempty"`
}

// Subject is the evaluation subject resolved by the Policy Service.
type Subject struct {
	Type      SubjectType
	ID        string
	CompanyID *string

	AgentAutonomy     string
	AgentStatus       string
	AgentCapabilities []string
	AgentVersion      int64
}

// Resource is the evaluation resource.
type Resource struct {
	Type       string
	ID         string
	Attributes map[string]any
}

// EvaluationInput is the normalized evaluation request.
type EvaluationInput struct {
	RequestID string
	TenantID  string

	CallerService string

	Subject Subject

	Action   string
	Resource Resource

	Risk Risk

	EstimatedCostMicroUSD int64
	Context               map[string]any
}

// Decision is a signed evaluation result.
type Decision struct {
	ID        string
	RequestID string
	TenantID  string

	CallerService string

	Subject Subject

	Action   string
	Resource Resource

	Risk Risk

	EstimatedCostMicroUSD int64

	Effect      Effect
	Constraints Constraints
	Reason      string

	MatchedPolicyID      *string
	MatchedPolicyVersion *int64
	MatchedRuleID        *string

	InputHash []byte

	DecidedAt time.Time
	ExpiresAt time.Time

	SigningKeyID string
	Signature    []byte
}

// ActiveBundle pairs an active policy set with its active version.
type ActiveBundle struct {
	Policy  Policy
	Version PolicyVersion
}
