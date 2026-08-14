// Package domain contains domain models, types, and errors for Model Gateway.
package domain

import (
	"errors"
	"time"
)

var (
	// ErrUnauthorized indicates missing or invalid authorization.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrTokenExpired indicates the runtime token has expired.
	ErrTokenExpired = errors.New("runtime token has expired")
	// ErrInvalidToken indicates the runtime token is malformed or invalid.
	ErrInvalidToken = errors.New("invalid runtime token")
	// ErrBudgetExceeded indicates the tenant budget limit has been reached.
	ErrBudgetExceeded = errors.New("budget limit exceeded")
	// ErrNoAvailableRoute indicates no model route could be resolved.
	ErrNoAvailableRoute = errors.New("no available model route found")
	// ErrDuplicateRequestID indicates a repeated request ID for the tenant.
	ErrDuplicateRequestID = errors.New("duplicate request ID")
	// ErrRequestAdmitted indicates the request was already recorded.
	ErrRequestAdmitted = errors.New("request already admitted")
	// ErrProviderCallFailed indicates the provider API call returned an error.
	ErrProviderCallFailed = errors.New("provider call failed")
	// ErrProviderRateLimit indicates provider rate limit was hit.
	ErrProviderRateLimit = errors.New("provider rate limit exceeded")
	// ErrContextLengthExceeded indicates input exceeded model context window.
	ErrContextLengthExceeded = errors.New("context length exceeded")
	// ErrOutputLimitExceeded indicates output exceeded max tokens.
	ErrOutputLimitExceeded = errors.New("output limit exceeded")
	// ErrClientDisconnected indicates client closed the HTTP connection during stream.
	ErrClientDisconnected = errors.New("client disconnected")
	// ErrStreamingInterrupted indicates stream broke after starting.
	ErrStreamingInterrupted = errors.New("streaming connection interrupted")
)

// RequestStatus is the status of an inference request in the journal.
type RequestStatus string

const (
	// RequestStatusAdmitted indicates the request was admitted into the journal.
	RequestStatusAdmitted RequestStatus = "admitted"
	// RequestStatusSucceeded indicates the request completed successfully.
	RequestStatusSucceeded RequestStatus = "succeeded"
	// RequestStatusFailed indicates the request failed.
	RequestStatusFailed RequestStatus = "failed"
	// RequestStatusClientDisconnected indicates the client disconnected during streaming.
	RequestStatusClientDisconnected RequestStatus = "client_disconnected"
)

// RuntimeClaims represents claims extracted from a valid runtime Ed25519 JWT.
type RuntimeClaims struct {
	TenantID    string
	AgentID     string
	ExecutionID string
	WorkflowID  string
	RunID       string
	StepID      string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// InferenceRequest represents an incoming chat completion request from an agent runtime cell.
type InferenceRequest struct {
	RequestID      string
	CorrelationID  string
	Model          string // Optional model alias: "primary", "fast", or specific offering ID
	Messages       []Message
	Temperature    *float64
	TopP           *float64
	MaxTokens      *int64
	Stream         bool
	Stop           []string
	Tools          []ToolDefinition
	ToolChoice     any
	ResponseFormat any
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a model-requested tool invocation.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes the function to call.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolFunctionSpec describes the function schema.
type ToolFunctionSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolDefinition defines a tool callable by the model.
type ToolDefinition struct {
	Type     string            `json:"type"`
	Function *ToolFunctionSpec `json:"function,omitempty"`
	Name     string            `json:"name,omitempty"`
	Desc     string            `json:"description,omitempty"`
	Params   any               `json:"parameters,omitempty"`
}

// TokenUsage tracks normalized token accounting across providers.
type TokenUsage struct {
	PromptTokens       int64
	CompletionTokens   int64
	TotalTokens        int64
	CachedPromptTokens int64
	ReasoningTokens    int64
}

// InferenceResult is the complete non-streamed response from a provider.
type InferenceResult struct {
	ID           string
	Model        string
	Created      int64
	Message      Message
	FinishReason string
	Usage        TokenUsage
}

// StreamChunk represents a single server-sent event (SSE) data chunk.
type StreamChunk struct {
	ID           string
	Model        string
	Created      int64
	DeltaRole    string
	DeltaContent string
	DeltaTool    *ToolCall
	FinishReason *string
	Usage        *TokenUsage
}

// JournalRecord tracks an admitted or completed request in the gateway database.
type JournalRecord struct {
	TenantID               string
	RequestID              string
	AgentID                string
	ExecutionID            string
	WorkflowID             string
	RunID                  string
	StepID                 string
	ConnectionID           string
	OfferingID             string
	ProviderKey            string
	ProviderModelID        string
	Status                 RequestStatus
	PromptTokens           int64
	CompletionTokens       int64
	TotalTokens            int64
	CachedPromptTokens     int64
	ReasoningTokens        int64
	EstimatedCostMicroUSD  int64
	ErrorCode              string
	Streamed               bool
	RetryCount             int
	FallbackFromOfferingID *string
	DurationMS             int64
	TimeToFirstTokenMS     int64
	AdmittedAt             time.Time
	CompletedAt            *time.Time
}

// OutboxEvent is a domain event persisted to the transactional outbox.
type OutboxEvent struct {
	EventID      string
	Topic        string
	PartitionKey string
	Payload      []byte
	OccurredAt   time.Time
}
