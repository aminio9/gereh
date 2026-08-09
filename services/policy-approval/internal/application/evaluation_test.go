package application

import (
	"context"
	"errors"
	"testing"
	"time"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/engine"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/aminio9/gereh/services/policy-approval/internal/security"
	"github.com/stretchr/testify/require"
)

const (
	testEvaluationPrincipal = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae2"
	testTenantID            = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae3"
	testSubjectID           = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae4"
	testRequestID           = "018f7767-28d2-7f5c-a693-0bb4c8ee4ae5"
	testSigningKey          = "k2cZF3qx4Ne01CBaUmvy7zBE8XB4v+WqZaIM5dBXQQw="
)

type mockRepository struct {
	findDecisionRecord func() (domain.Decision, error)
	listActiveBundles  func() ([]domain.ActiveBundle, error)
	recordDecision     func(params ports.RecordDecisionParams) (domain.Decision, error)
}

func (repository *mockRepository) CreatePolicy(
	context.Context,
	ports.CreatePolicyParams,
) (domain.Policy, error) {
	return domain.Policy{}, nil
}

func (repository *mockRepository) GetPolicy(
	_ context.Context,
	_ string,
	_ string,
	_ string,
) (domain.Policy, *domain.PolicyVersion, error) {
	return domain.Policy{}, nil, nil
}

func (repository *mockRepository) ListPolicies(
	context.Context,
	string,
	string,
	int,
	*ports.PolicyCursor,
	bool,
) ([]domain.Policy, error) {
	return nil, nil
}

func (repository *mockRepository) CreateVersion(
	context.Context,
	ports.CreateVersionParams,
) (domain.Policy, domain.PolicyVersion, error) {
	return domain.Policy{}, domain.PolicyVersion{}, nil
}

func (repository *mockRepository) ActivatePolicy(
	context.Context,
	ports.ActivatePolicyParams,
) (domain.Policy, domain.PolicyVersion, error) {
	return domain.Policy{}, domain.PolicyVersion{}, nil
}

func (repository *mockRepository) ArchivePolicy(
	context.Context,
	ports.ArchivePolicyParams,
) (domain.Policy, error) {
	return domain.Policy{}, nil
}

func (repository *mockRepository) ListActiveBundles(
	context.Context,
	string,
	string,
	*string,
	*string,
) ([]domain.ActiveBundle, error) {
	if repository.listActiveBundles != nil {
		return repository.listActiveBundles()
	}

	return nil, nil
}

func (repository *mockRepository) FindDecisionByRequestID(
	context.Context,
	string,
	string,
	string,
) (domain.Decision, error) {
	if repository.findDecisionRecord != nil {
		return repository.findDecisionRecord()
	}

	return domain.Decision{}, domain.ErrNotFound
}

func (repository *mockRepository) RecordDecision(
	_ context.Context,
	params ports.RecordDecisionParams,
) (domain.Decision, error) {
	if repository.recordDecision != nil {
		return repository.recordDecision(params)
	}

	return params.Decision, nil
}

func (repository *mockRepository) GetDecision(
	context.Context,
	string,
	string,
	string,
) (domain.Decision, error) {
	return domain.Decision{}, nil
}

func (repository *mockRepository) ListDecisions(
	context.Context,
	string,
	string,
	*string,
	int,
	*ports.DecisionCursor,
) ([]domain.Decision, error) {
	return nil, nil
}

func (repository *mockRepository) EnsureDefaults(
	context.Context,
	ports.EnsureDefaultsParams,
) ([]domain.Policy, error) {
	return nil, nil
}

func (repository *mockRepository) ClaimOutbox(
	context.Context,
	int,
	time.Duration,
) ([]domain.OutboxRecord, error) {
	return nil, nil
}

func (repository *mockRepository) MarkOutboxPublished(
	context.Context,
	int64,
) error {
	return nil
}

func (repository *mockRepository) ReleaseOutbox(
	context.Context,
	int64,
	time.Time,
	string,
) error {
	return nil
}

func testEvaluationInput() EvaluateInput {
	return EvaluateInput{
		RequestID:     testRequestID,
		TenantID:      testTenantID,
		CallerService: "work-management",
		Subject: domain.Subject{
			Type: domain.SubjectUser,
			ID:   testSubjectID,
		},
		Action:   "work.task.update",
		Resource: domain.Resource{Type: "task", ID: "task-1"},
		Risk:     domain.RiskLow,
	}
}

type stubAuthorizer struct{}

func (stubAuthorizer) Require(
	context.Context,
	string,
	string,
	tenantv1.Permission,
) error {
	return nil
}

type stubOrganizationClient struct{}

func (stubOrganizationClient) GetAgentPolicyContext(
	context.Context,
	string,
	string,
) (domain.Subject, error) {
	return domain.Subject{}, nil
}

func newMockService(
	t *testing.T,
	repository *mockRepository,
) *Service {
	t.Helper()

	celEngine, err := engine.NewCEL()
	require.NoError(t, err)

	signer, err := security.NewSigner("key-1", testSigningKey)
	require.NoError(t, err)

	service, err := New(
		repository,
		stubAuthorizer{},
		stubOrganizationClient{},
		engine.NewEvaluator(celEngine),
		signer,
		Config{
			EventTopic:                   "gereh.policy.events.v1",
			EvaluationServicePrincipalID: testEvaluationPrincipal,
			BootstrapServicePrincipalID:  "018f7767-28d2-7f5c-a693-0bb4c8ee4a01",
			DecisionTTL:                  time.Minute,
		},
	)
	require.NoError(t, err)

	return service
}

func TestEvaluateRecordsSignedDecision(t *testing.T) {
	t.Parallel()

	repository := &mockRepository{}

	callCount := 0

	repository.recordDecision = func(params ports.RecordDecisionParams) (domain.Decision, error) {
		callCount++
		require.Equal(t, domain.EffectDeny, params.Decision.Effect)
		require.NotEmpty(t, params.Decision.Signature)
		require.Equal(t, "key-1", params.Decision.SigningKeyID)
		require.NotEmpty(t, params.Decision.InputHash)
		require.NotEmpty(t, params.Event.Envelope)

		return params.Decision, nil
	}

	decision, err := newMockService(t, repository).Evaluate(
		context.Background(),
		testEvaluationInput(),
	)

	require.NoError(t, err)
	require.Equal(t, callCount, 1)
	require.Equal(t, domain.EffectDeny, decision.Effect)
	require.NotEmpty(t, decision.Signature)
}

func TestEvaluateRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	input := testEvaluationInput()
	input.RequestID = "not-a-uuid"

	_, err := newMockService(
		t,
		&mockRepository{},
	).Evaluate(context.Background(), input)

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestEvaluateReturnsExistingDecision(t *testing.T) {
	t.Parallel()

	existing := domain.Decision{
		ID:        "decision-1",
		RequestID: testRequestID,
		TenantID:  testTenantID,
		Effect:    domain.EffectAllow,
		DecidedAt: time.Now().UTC(),
	}

	input := testEvaluationInput()

	hashed, err := hashEvaluationInput(domain.EvaluationInput{
		RequestID:     input.RequestID,
		TenantID:      input.TenantID,
		CallerService: input.CallerService,
		Subject:       input.Subject,
		Action:        input.Action,
		Resource:      input.Resource,
		Risk:          input.Risk,
		Context:       input.Context,
	})
	require.NoError(t, err)

	existing.InputHash = hashed

	repository := &mockRepository{
		findDecisionRecord: func() (domain.Decision, error) {
			return existing, nil
		},
	}

	decision, err := newMockService(t, repository).Evaluate(
		context.Background(),
		input,
	)

	require.NoError(t, err)
	require.Equal(t, "decision-1", decision.ID)
}

func TestEvaluateMismatchedExistingDecisionRejected(t *testing.T) {
	t.Parallel()

	existing := domain.Decision{
		ID:        "decision-1",
		RequestID: testRequestID,
		TenantID:  testTenantID,
		Effect:    domain.EffectAllow,
		InputHash: []byte("tampered"),
	}

	repository := &mockRepository{
		findDecisionRecord: func() (domain.Decision, error) {
			return existing, nil
		},
	}

	_, err := newMockService(t, repository).Evaluate(
		context.Background(),
		testEvaluationInput(),
	)

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrDecisionMismatch)
}

func TestEvaluateTransportErrorFailsClosed(t *testing.T) {
	t.Parallel()

	repository := &mockRepository{
		findDecisionRecord: func() (domain.Decision, error) {
			return domain.Decision{}, errors.New("database unavailable")
		},
	}

	_, err := newMockService(t, repository).Evaluate(
		context.Background(),
		testEvaluationInput(),
	)

	require.Error(t, err)
	require.NotErrorIs(t, err, nil)
}
