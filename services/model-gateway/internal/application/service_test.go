package application_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	"github.com/aminio9/gereh/services/model-gateway/internal/application"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/aminio9/gereh/services/model-gateway/internal/ports"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mockResolver struct {
	plan *modelgatewayv1.InferencePlan
	err  error
}

func (m *mockResolver) ResolveInferencePlan(context.Context, string, string) (*modelgatewayv1.InferencePlan, error) {
	return m.plan, m.err
}

type mockSecretStore struct {
	secret []byte
	err    error
}

func (m *mockSecretStore) GetBYOKSecret(context.Context, string, string) ([]byte, error) {
	return m.secret, m.err
}

type mockJournal struct {
	admitted  []domain.JournalRecord
	completed []domain.JournalRecord
	outbox    []*domain.OutboxEvent
}

func (m *mockJournal) AdmitRequest(_ context.Context, rec domain.JournalRecord) error {
	m.admitted = append(m.admitted, rec)
	return nil
}

func (m *mockJournal) CompleteRequest(_ context.Context, rec domain.JournalRecord, outbox *domain.OutboxEvent) error {
	m.completed = append(m.completed, rec)
	if outbox != nil {
		m.outbox = append(m.outbox, outbox)
	}
	return nil
}

type mockAdapter struct {
	providerKey string
	completeRes domain.InferenceResult
	completeErr error
	streamUsage domain.TokenUsage
	streamErr   error
	chunks      []domain.StreamChunk
}

func (m *mockAdapter) ProviderKey() string {
	return m.providerKey
}

func (m *mockAdapter) Complete(
	_ context.Context,
	_ *modelgatewayv1.InferenceRoute,
	_ []byte,
	_ domain.InferenceRequest,
) (domain.InferenceResult, error) {
	return m.completeRes, m.completeErr
}

func (m *mockAdapter) Stream(
	ctx context.Context,
	_ *modelgatewayv1.InferenceRoute,
	_ []byte,
	_ domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) (domain.TokenUsage, error) {
	if m.streamErr != nil && len(m.chunks) == 0 {
		return domain.TokenUsage{}, m.streamErr
	}

	for _, chunk := range m.chunks {
		if ctx.Err() != nil {
			return m.streamUsage, ctx.Err()
		}
		if err := onChunk(chunk); err != nil {
			return m.streamUsage, err
		}
	}

	return m.streamUsage, m.streamErr
}

func TestApplicationService_ExecuteFallback(t *testing.T) {
	tenantID := uuid.NewString()
	agentID := uuid.NewString()
	executionID := uuid.NewString()

	claims := domain.RuntimeClaims{
		TenantID:    tenantID,
		AgentID:     agentID,
		ExecutionID: executionID,
		WorkflowID:  "wf-1",
		RunID:       "run-1",
		StepID:      "step-1",
	}

	plan := &modelgatewayv1.InferencePlan{
		TenantId:       tenantID,
		AgentId:        agentID,
		BindingVersion: 1,
		PrimaryRoute: &modelgatewayv1.InferenceRoute{
			OfferingId:      "primary-offering",
			ConnectionId:    "primary-conn",
			ProviderKey:     "openai",
			ProviderModelId: "gpt-4o",
			ConnectionType:  "byok",
		},
		FallbackRoutes: []*modelgatewayv1.InferenceRoute{
			{
				OfferingId:      "fallback-offering",
				ConnectionId:    "fallback-conn",
				ProviderKey:     "anthropic",
				ProviderModelId: "claude-3-5-sonnet",
				ConnectionType:  "byok",
			},
		},
	}

	openaiAdapter := &mockAdapter{
		providerKey: "openai",
		completeErr: errors.New("rate limited 429"),
	}

	anthropicAdapter := &mockAdapter{
		providerKey: "anthropic",
		completeRes: domain.InferenceResult{
			ID:    "resp-123",
			Model: "claude-3-5-sonnet",
			Message: domain.Message{
				Role:    "assistant",
				Content: "Fallback success response",
			},
			Usage: domain.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
	}

	journal := &mockJournal{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	service := application.New(
		application.ServiceConfig{EventTopic: "gereh.model.usage.v1"},
		&mockResolver{plan: plan},
		&mockSecretStore{secret: []byte("test-api-key")},
		ports.DeferredBudgetVerifier{},
		journal,
		[]ports.ProviderAdapter{openaiAdapter, anthropicAdapter},
		logger,
	)

	req := domain.InferenceRequest{
		RequestID:     "req-001",
		CorrelationID: "corr-001",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello model"},
		},
	}

	result, err := service.Execute(context.Background(), claims, req)
	require.NoError(t, err)
	require.Equal(t, "Fallback success response", result.Message.Content)

	require.Len(t, journal.completed, 1)
	completed := journal.completed[0]
	require.Equal(t, domain.RequestStatusSucceeded, completed.Status)
	require.Equal(t, "fallback-offering", completed.OfferingID)
	require.Equal(t, "anthropic", completed.ProviderKey)
	require.Equal(t, 1, completed.RetryCount)
	require.NotNil(t, completed.FallbackFromOfferingID)
	require.Equal(t, "primary-offering", *completed.FallbackFromOfferingID)
	require.Len(t, journal.outbox, 1)
}

func TestApplicationService_StreamLocking(t *testing.T) {
	tenantID := uuid.NewString()
	agentID := uuid.NewString()
	executionID := uuid.NewString()

	claims := domain.RuntimeClaims{
		TenantID:    tenantID,
		AgentID:     agentID,
		ExecutionID: executionID,
		WorkflowID:  "wf-1",
		RunID:       "run-1",
		StepID:      "step-1",
	}

	plan := &modelgatewayv1.InferencePlan{
		TenantId:       tenantID,
		AgentId:        agentID,
		BindingVersion: 1,
		PrimaryRoute: &modelgatewayv1.InferenceRoute{
			OfferingId:      "primary-offering",
			ConnectionId:    "primary-conn",
			ProviderKey:     "openai",
			ProviderModelId: "gpt-4o",
			ConnectionType:  "byok",
		},
		FallbackRoutes: []*modelgatewayv1.InferenceRoute{
			{
				OfferingId:      "fallback-offering",
				ConnectionId:    "fallback-conn",
				ProviderKey:     "anthropic",
				ProviderModelId: "claude-3-5-sonnet",
				ConnectionType:  "byok",
			},
		},
	}

	// OpenAI delivers 1 chunk then fails mid-stream
	openaiAdapter := &mockAdapter{
		providerKey: "openai",
		chunks: []domain.StreamChunk{
			{ID: "chunk-1", DeltaContent: "Partial "},
		},
		streamErr: errors.New("connection reset by peer mid-stream"),
		streamUsage: domain.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 2,
			TotalTokens:      12,
		},
	}

	anthropicAdapter := &mockAdapter{
		providerKey: "anthropic",
		chunks: []domain.StreamChunk{
			{ID: "chunk-2", DeltaContent: "Should not be called once locked"},
		},
	}

	journal := &mockJournal{}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	service := application.New(
		application.ServiceConfig{EventTopic: "gereh.model.usage.v1"},
		&mockResolver{plan: plan},
		&mockSecretStore{secret: []byte("test-api-key")},
		ports.DeferredBudgetVerifier{},
		journal,
		[]ports.ProviderAdapter{openaiAdapter, anthropicAdapter},
		logger,
	)

	req := domain.InferenceRequest{
		RequestID:     "req-002",
		CorrelationID: "corr-002",
		Stream:        true,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello streaming"},
		},
	}

	var receivedChunks []string
	err := service.ExecuteStream(context.Background(), claims, req, func(chunk domain.StreamChunk) error {
		receivedChunks = append(receivedChunks, chunk.DeltaContent)
		return nil
	})

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrStreamingInterrupted)
	require.Equal(t, []string{"Partial "}, receivedChunks)

	require.Len(t, journal.completed, 1)
	completed := journal.completed[0]
	// Stream route was locked to primary-offering; did NOT switch to anthropic mid-stream
	require.Equal(t, "primary-offering", completed.OfferingID)
	require.Equal(t, domain.RequestStatusFailed, completed.Status)
	require.Equal(t, "stream_interrupted", completed.ErrorCode)
}
