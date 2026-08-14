package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	commonv1 "github.com/aminio9/gereh/gen/go/gereh/common/v1"
	modelgatewayv1 "github.com/aminio9/gereh/gen/go/gereh/model/gateway/v1"
	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/aminio9/gereh/services/model-gateway/internal/ports"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ServiceConfig contains configuration for the Model Gateway service.
type ServiceConfig struct {
	EventTopic                 string
	RequireBudgetReservation   bool
	MaxContextWindowMultiplier float64
}

// Service orchestrates authentication, plan resolution, pre-stream routing, streaming lock, and usage accounting.
type Service struct {
	config         ServiceConfig
	resolver       ports.ResolverClient
	secretStore    ports.SecretStore
	budgetVerifier ports.BudgetVerifier
	journal        ports.JournalRepository
	adapters       map[string]ports.ProviderAdapter
	logger         *slog.Logger
}

// New creates a new Model Gateway application service.
func New(
	config ServiceConfig,
	resolver ports.ResolverClient,
	secretStore ports.SecretStore,
	budgetVerifier ports.BudgetVerifier,
	journal ports.JournalRepository,
	adapters []ports.ProviderAdapter,
	logger *slog.Logger,
) *Service {
	adapterMap := make(map[string]ports.ProviderAdapter, len(adapters))
	for _, a := range adapters {
		adapterMap[a.ProviderKey()] = a
	}

	return &Service{
		config:         config,
		resolver:       resolver,
		secretStore:    secretStore,
		budgetVerifier: budgetVerifier,
		journal:        journal,
		adapters:       adapterMap,
		logger:         logger,
	}
}

// Execute handles non-streaming inference requests with pre-stream fallback.
func (s *Service) Execute(
	ctx context.Context,
	claims domain.RuntimeClaims,
	req domain.InferenceRequest,
) (domain.InferenceResult, error) {
	startTime := time.Now().UTC()

	// 1. Resolve inference plan
	plan, err := s.resolver.ResolveInferencePlan(ctx, claims.TenantID, claims.AgentID)
	if err != nil {
		return domain.InferenceResult{}, fmt.Errorf("resolve inference plan: %w", err)
	}

	// 2. Build candidate route sequence (primary/fast/fallbacks)
	routes := s.buildRouteChain(plan, req.Model)
	if len(routes) == 0 {
		return domain.InferenceResult{}, domain.ErrNoAvailableRoute
	}

	// 3. Initial Admit in request journal
	initialRoute := routes[0]
	journalRecord := domain.JournalRecord{
		TenantID:        claims.TenantID,
		RequestID:       req.RequestID,
		AgentID:         claims.AgentID,
		ExecutionID:     claims.ExecutionID,
		WorkflowID:      claims.WorkflowID,
		RunID:           claims.RunID,
		StepID:          claims.StepID,
		ConnectionID:    initialRoute.ConnectionId,
		OfferingID:      initialRoute.OfferingId,
		ProviderKey:     initialRoute.ProviderKey,
		ProviderModelID: initialRoute.ProviderModelId,
		Status:          domain.RequestStatusAdmitted,
		Streamed:        false,
		AdmittedAt:      startTime,
	}

	if err := s.journal.AdmitRequest(ctx, journalRecord); err != nil {
		return domain.InferenceResult{}, err
	}

	// 4. Budget verification
	if s.config.RequireBudgetReservation {
		if err := s.budgetVerifier.CheckBudget(ctx, claims.TenantID, claims.AgentID, plan.MaxModelCostMicrousd); err != nil {
			s.recordFailure(ctx, journalRecord, domain.RequestStatusFailed, "budget_exceeded", startTime, 0, nil)
			return domain.InferenceResult{}, domain.ErrBudgetExceeded
		}
	}

	// 5. Try routes in order (pre-stream retry/fallback)
	var (
		lastErr                error
		res                    domain.InferenceResult
		selectedRoute          *modelgatewayv1.InferenceRoute
		retryCount             int
		fallbackFromOfferingID *string
	)

	for idx, route := range routes {
		if idx > 0 {
			fallbackFromOfferingID = &routes[0].OfferingId
		}

		apiKey, err := s.getRouteAPIKey(ctx, claims.TenantID, route)
		if err != nil {
			lastErr = err
			retryCount++
			continue
		}

		adapter, ok := s.adapters[route.ProviderKey]
		if !ok {
			lastErr = fmt.Errorf("no adapter for provider: %s", route.ProviderKey)
			retryCount++
			continue
		}

		result, err := adapter.Complete(ctx, route, apiKey, req)
		if err != nil {
			lastErr = err
			retryCount++
			continue
		}

		res = result
		selectedRoute = route
		break
	}

	durationMS := time.Since(startTime).Milliseconds()

	if selectedRoute == nil {
		s.recordFailure(ctx, journalRecord, domain.RequestStatusFailed, "all_routes_failed", startTime, retryCount, fallbackFromOfferingID)
		return domain.InferenceResult{}, fmt.Errorf("%w: %w", domain.ErrProviderCallFailed, lastErr)
	}

	// 6. Complete journal and emit usage event
	journalRecord.ConnectionID = selectedRoute.ConnectionId
	journalRecord.OfferingID = selectedRoute.OfferingId
	journalRecord.ProviderKey = selectedRoute.ProviderKey
	journalRecord.ProviderModelID = selectedRoute.ProviderModelId
	journalRecord.Status = domain.RequestStatusSucceeded
	journalRecord.PromptTokens = res.Usage.PromptTokens
	journalRecord.CompletionTokens = res.Usage.CompletionTokens
	journalRecord.TotalTokens = res.Usage.TotalTokens
	journalRecord.CachedPromptTokens = res.Usage.CachedPromptTokens
	journalRecord.ReasoningTokens = res.Usage.ReasoningTokens
	journalRecord.RetryCount = retryCount
	journalRecord.FallbackFromOfferingID = fallbackFromOfferingID
	journalRecord.DurationMS = durationMS
	completedNow := time.Now().UTC()
	journalRecord.CompletedAt = &completedNow

	outboxEvent := s.createUsageEvent(journalRecord, claims, req.CorrelationID, modelv1.ModelUsageOutcome_MODEL_USAGE_OUTCOME_SUCCESS, "")
	_ = s.journal.CompleteRequest(ctx, journalRecord, outboxEvent)

	return res, nil
}

// ExecuteStream handles streaming inference requests with pre-stream fallback, streaming lock, and client disconnect handling.
func (s *Service) ExecuteStream(
	ctx context.Context,
	claims domain.RuntimeClaims,
	req domain.InferenceRequest,
	onChunk func(domain.StreamChunk) error,
) error {
	startTime := time.Now().UTC()

	// 1. Resolve inference plan
	plan, err := s.resolver.ResolveInferencePlan(ctx, claims.TenantID, claims.AgentID)
	if err != nil {
		return fmt.Errorf("resolve inference plan: %w", err)
	}

	routes := s.buildRouteChain(plan, req.Model)
	if len(routes) == 0 {
		return domain.ErrNoAvailableRoute
	}

	initialRoute := routes[0]
	journalRecord := domain.JournalRecord{
		TenantID:        claims.TenantID,
		RequestID:       req.RequestID,
		AgentID:         claims.AgentID,
		ExecutionID:     claims.ExecutionID,
		WorkflowID:      claims.WorkflowID,
		RunID:           claims.RunID,
		StepID:          claims.StepID,
		ConnectionID:    initialRoute.ConnectionId,
		OfferingID:      initialRoute.OfferingId,
		ProviderKey:     initialRoute.ProviderKey,
		ProviderModelID: initialRoute.ProviderModelId,
		Status:          domain.RequestStatusAdmitted,
		Streamed:        true,
		AdmittedAt:      startTime,
	}

	if err := s.journal.AdmitRequest(ctx, journalRecord); err != nil {
		return err
	}

	if s.config.RequireBudgetReservation {
		if err := s.budgetVerifier.CheckBudget(ctx, claims.TenantID, claims.AgentID, plan.MaxModelCostMicrousd); err != nil {
			s.recordFailure(ctx, journalRecord, domain.RequestStatusFailed, "budget_exceeded", startTime, 0, nil)
			return domain.ErrBudgetExceeded
		}
	}

	var (
		lastErr                error
		selectedRoute          *modelgatewayv1.InferenceRoute
		retryCount             int
		fallbackFromOfferingID *string
		timeToFirstTokenMS     int64
		firstChunkSent         bool
		finalUsage             domain.TokenUsage
		clientDisconnected     bool
	)

	for idx, route := range routes {
		if idx > 0 {
			fallbackFromOfferingID = &routes[0].OfferingId
		}

		apiKey, err := s.getRouteAPIKey(ctx, claims.TenantID, route)
		if err != nil {
			lastErr = err
			retryCount++
			continue
		}

		adapter, ok := s.adapters[route.ProviderKey]
		if !ok {
			lastErr = fmt.Errorf("no adapter for provider: %s", route.ProviderKey)
			retryCount++
			continue
		}

		// Stream proxy callback
		usage, streamErr := adapter.Stream(ctx, route, apiKey, req, func(chunk domain.StreamChunk) error {
			if !firstChunkSent {
				firstChunkSent = true
				timeToFirstTokenMS = time.Since(startTime).Milliseconds()
			}

			// Forward chunk downstream to agent runtime cell
			if err := onChunk(chunk); err != nil {
				clientDisconnected = true
				return err
			}
			return nil
		})

		if firstChunkSent {
			// Once streaming started, route is locked — do not fallback
			selectedRoute = route
			finalUsage = usage
			if streamErr != nil && !clientDisconnected {
				lastErr = streamErr
			}
			break
		}

		// If no chunk was sent yet, we can safely try the next fallback route
		if streamErr != nil {
			lastErr = streamErr
			retryCount++
			continue
		}

		selectedRoute = route
		finalUsage = usage
		break
	}

	durationMS := time.Since(startTime).Milliseconds()

	if selectedRoute == nil {
		s.recordFailure(ctx, journalRecord, domain.RequestStatusFailed, "all_routes_failed", startTime, retryCount, fallbackFromOfferingID)
		return fmt.Errorf("%w: %w", domain.ErrProviderCallFailed, lastErr)
	}

	status := domain.RequestStatusSucceeded
	outcome := modelv1.ModelUsageOutcome_MODEL_USAGE_OUTCOME_SUCCESS
	errorCode := ""

	if clientDisconnected {
		status = domain.RequestStatusClientDisconnected
		outcome = modelv1.ModelUsageOutcome_MODEL_USAGE_OUTCOME_CLIENT_DISCONNECTED
		errorCode = "client_disconnected"
	} else if lastErr != nil {
		status = domain.RequestStatusFailed
		outcome = modelv1.ModelUsageOutcome_MODEL_USAGE_OUTCOME_PROVIDER_ERROR
		errorCode = "stream_interrupted"
	}

	journalRecord.ConnectionID = selectedRoute.ConnectionId
	journalRecord.OfferingID = selectedRoute.OfferingId
	journalRecord.ProviderKey = selectedRoute.ProviderKey
	journalRecord.ProviderModelID = selectedRoute.ProviderModelId
	journalRecord.Status = status
	journalRecord.PromptTokens = finalUsage.PromptTokens
	journalRecord.CompletionTokens = finalUsage.CompletionTokens
	journalRecord.TotalTokens = finalUsage.TotalTokens
	journalRecord.CachedPromptTokens = finalUsage.CachedPromptTokens
	journalRecord.ReasoningTokens = finalUsage.ReasoningTokens
	journalRecord.ErrorCode = errorCode
	journalRecord.RetryCount = retryCount
	journalRecord.FallbackFromOfferingID = fallbackFromOfferingID
	journalRecord.DurationMS = durationMS
	journalRecord.TimeToFirstTokenMS = timeToFirstTokenMS
	completedNow := time.Now().UTC()
	journalRecord.CompletedAt = &completedNow

	outboxEvent := s.createUsageEvent(journalRecord, claims, req.CorrelationID, outcome, errorCode)
	_ = s.journal.CompleteRequest(ctx, journalRecord, outboxEvent)

	if clientDisconnected {
		return domain.ErrClientDisconnected
	}
	if lastErr != nil {
		return fmt.Errorf("%w: %w", domain.ErrStreamingInterrupted, lastErr)
	}

	return nil
}

func (s *Service) buildRouteChain(
	plan *modelgatewayv1.InferencePlan,
	modelAlias string,
) []*modelgatewayv1.InferenceRoute {
	var routes []*modelgatewayv1.InferenceRoute

	if modelAlias == "fast" && plan.FastRoute != nil {
		routes = append(routes, plan.FastRoute)
		if plan.PrimaryRoute != nil {
			routes = append(routes, plan.PrimaryRoute)
		}
	} else {
		if plan.PrimaryRoute != nil {
			routes = append(routes, plan.PrimaryRoute)
		}
		if plan.FastRoute != nil {
			routes = append(routes, plan.FastRoute)
		}
	}

	routes = append(routes, plan.FallbackRoutes...)
	return routes
}

func (s *Service) getRouteAPIKey(
	ctx context.Context,
	tenantID string,
	route *modelgatewayv1.InferenceRoute,
) ([]byte, error) {
	if route.ConnectionType == "platform_managed" {
		// Platform keys retrieved internally if applicable
		return []byte("platform-managed-key"), nil
	}
	return s.secretStore.GetBYOKSecret(ctx, tenantID, route.ConnectionId)
}

func (s *Service) recordFailure(
	ctx context.Context,
	record domain.JournalRecord,
	status domain.RequestStatus,
	errorCode string,
	startTime time.Time,
	retryCount int,
	fallbackFrom *string,
) {
	record.Status = status
	record.ErrorCode = errorCode
	record.RetryCount = retryCount
	record.FallbackFromOfferingID = fallbackFrom
	record.DurationMS = time.Since(startTime).Milliseconds()
	now := time.Now().UTC()
	record.CompletedAt = &now
	_ = s.journal.CompleteRequest(ctx, record, nil)
}

func (s *Service) createUsageEvent(
	rec domain.JournalRecord,
	claims domain.RuntimeClaims,
	correlationID string,
	outcome modelv1.ModelUsageOutcome,
	errorCode string,
) *domain.OutboxEvent {
	eventID := uuid.NewString()

	fallbackID := ""
	if rec.FallbackFromOfferingID != nil {
		fallbackID = *rec.FallbackFromOfferingID
	}

	usageEvent := &modelv1.ModelUsageRecorded{
		TenantId:               rec.TenantID,
		RequestId:              rec.RequestID,
		CorrelationId:          correlationID,
		AgentId:                claims.AgentID,
		ExecutionId:            claims.ExecutionID,
		WorkflowId:             claims.WorkflowID,
		RunId:                  claims.RunID,
		StepId:                 claims.StepID,
		ConnectionId:           rec.ConnectionID,
		OfferingId:             rec.OfferingID,
		ProviderKey:            rec.ProviderKey,
		ProviderModelId:        rec.ProviderModelID,
		PromptTokens:           rec.PromptTokens,
		CompletionTokens:       rec.CompletionTokens,
		TotalTokens:            rec.TotalTokens,
		CachedPromptTokens:     rec.CachedPromptTokens,
		ReasoningTokens:        rec.ReasoningTokens,
		EstimatedCostMicrousd:  rec.EstimatedCostMicroUSD,
		Outcome:                outcome,
		ErrorCode:              errorCode,
		DurationMs:             rec.DurationMS,
		TimeToFirstTokenMs:     rec.TimeToFirstTokenMS,
		Streamed:               rec.Streamed,
		RetryCount:             int32(rec.RetryCount),
		FallbackFromOfferingId: fallbackID,
		RecordedAt:             timestamppb.New(time.Now().UTC()),
	}

	payloadBytes, err := proto.Marshal(usageEvent)
	if err != nil {
		return nil
	}

	now := time.Now().UTC()
	envelope := &commonv1.EventEnvelope{
		EventId:          eventID,
		EventType:        "model.usage.recorded",
		EventVersion:     1,
		TenantId:         rec.TenantID,
		AggregateType:    "model_usage",
		AggregateId:      rec.RequestID,
		AggregateVersion: 1,
		OccurredAt:       timestamppb.New(now),
		Producer:         "model-gateway",
		CorrelationId:    correlationID,
		CausationId:      rec.RequestID,
		Payload:          payloadBytes,
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return nil
	}

	return &domain.OutboxEvent{
		EventID:      eventID,
		Topic:        s.config.EventTopic,
		PartitionKey: rec.TenantID,
		Payload:      envelopeBytes,
		OccurredAt:   now,
	}
}
