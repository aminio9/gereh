package application

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
	"github.com/aminio9/gereh/services/policy-approval/internal/protoutil"
	"google.golang.org/protobuf/proto"
)

// EvaluateInput is the normalized evaluation request.
type EvaluateInput struct {
	RequestID string
	TenantID  string

	CallerService string

	Subject domain.Subject

	Action   string
	Resource domain.Resource

	Risk domain.Risk

	EstimatedCostMicroUSD int64
	Context               map[string]any
}

// Evaluate evaluates one action and returns a signed decision. Enforcement
// points must fail closed: transport failures surface as gRPC UNAVAILABLE.
func (service *Service) Evaluate(
	ctx context.Context,
	input EvaluateInput,
) (domain.Decision, error) {
	if err := validateEvaluation(input); err != nil {
		return domain.Decision{}, err
	}

	resolvedSubject, err := service.resolveSubject(
		ctx,
		input.TenantID,
		input.Subject,
	)
	if err != nil {
		return domain.Decision{}, err
	}

	evaluation := domain.EvaluationInput{
		RequestID: input.RequestID,
		TenantID:  input.TenantID,

		CallerService: input.CallerService,

		Subject: resolvedSubject,

		Action:   input.Action,
		Resource: input.Resource,

		Risk: input.Risk,

		EstimatedCostMicroUSD: input.EstimatedCostMicroUSD,

		Context: input.Context,
	}

	inputHash, err := hashEvaluationInput(evaluation)
	if err != nil {
		return domain.Decision{}, err
	}

	existing, err := service.repository.FindDecisionByRequestID(
		ctx,
		service.config.EvaluationServicePrincipalID,
		input.TenantID,
		input.RequestID,
	)

	switch {
	case err == nil:
		if !equalBytes(existing.InputHash, inputHash) {
			return domain.Decision{}, domain.ErrDecisionMismatch
		}

		return existing, nil

	case !errors.Is(err, domain.ErrNotFound):
		return domain.Decision{}, err
	}

	bundles, err := service.repository.ListActiveBundles(
		ctx,
		service.config.EvaluationServicePrincipalID,
		input.TenantID,
		resolvedSubject.CompanyID,
		agentIDForScope(resolvedSubject),
	)
	if err != nil {
		return domain.Decision{}, err
	}

	result, err := service.evaluator.Evaluate(
		ctx,
		evaluation,
		bundles,
	)
	if err != nil {
		return domain.Decision{}, err
	}

	decisionID, err := newUUID()
	if err != nil {
		return domain.Decision{}, err
	}

	now := service.now().UTC()

	decision := domain.Decision{
		ID:        decisionID,
		RequestID: input.RequestID,
		TenantID:  input.TenantID,

		CallerService: input.CallerService,

		Subject: resolvedSubject,

		Action:   input.Action,
		Resource: input.Resource,

		Risk: input.Risk,

		EstimatedCostMicroUSD: input.EstimatedCostMicroUSD,

		Effect:      result.Effect,
		Constraints: result.Constraints,
		Reason:      result.Reason,

		MatchedPolicyID:      result.MatchedPolicyID,
		MatchedPolicyVersion: result.MatchedPolicyVersion,
		MatchedRuleID:        result.MatchedRuleID,

		InputHash: inputHash,

		DecidedAt: now,
		ExpiresAt: now.Add(service.config.DecisionTTL),

		SigningKeyID: service.signer.KeyID(),
	}

	protoDecision := protoutil.Decision(decision)

	signature, err := service.signer.Sign(protoDecision)
	if err != nil {
		return domain.Decision{}, err
	}

	decision.Signature = signature

	event, err := service.newDecisionEvent(ctx, decision)
	if err != nil {
		return domain.Decision{}, err
	}

	return service.repository.RecordDecision(
		ctx,
		ports.RecordDecisionParams{
			ServicePrincipalID: service.config.EvaluationServicePrincipalID,
			Decision:           decision,
			Event:              event,
		},
	)
}

// resolveSubject replaces caller-supplied agent context with trusted context
// resolved from the Organization Service.
func (service *Service) resolveSubject(
	ctx context.Context,
	tenantID string,
	subject domain.Subject,
) (domain.Subject, error) {
	if subject.Type != domain.SubjectAgent {
		return subject, nil
	}

	resolved, err := service.organizationClient.GetAgentPolicyContext(
		ctx,
		tenantID,
		subject.ID,
	)
	if err != nil {
		return domain.Subject{}, err
	}

	return resolved, nil
}

func agentIDForScope(subject domain.Subject) *string {
	if subject.Type != domain.SubjectAgent {
		return nil
	}

	id := subject.ID
	return &id
}

func hashEvaluationInput(
	input domain.EvaluationInput,
) ([]byte, error) {
	message := protoutil.EvaluationInput(input)

	encoded, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal evaluation input: %w",
			err,
		)
	}

	sum := sha256.Sum256(encoded)

	return sum[:], nil
}

func equalBytes(left []byte, right []byte) bool {
	return subtle.ConstantTimeCompare(left, right) == 1
}

func validateEvaluation(input EvaluateInput) error {
	if err := validateUUID("evaluation_request_id", input.RequestID); err != nil {
		return err
	}

	if err := validateUUID("tenant_id", input.TenantID); err != nil {
		return err
	}

	if input.CallerService == "" {
		return fmt.Errorf(
			"%w: caller service is required",
			domain.ErrInvalidArgument,
		)
	}

	if input.Subject.Type == "" {
		return fmt.Errorf(
			"%w: subject type is required",
			domain.ErrInvalidArgument,
		)
	}

	if err := validateUUID("subject_id", input.Subject.ID); err != nil {
		return err
	}

	if err := boundedText("action", input.Action, 256); err != nil {
		return err
	}

	if err := boundedText("resource_type", input.Resource.Type, 256); err != nil {
		return err
	}

	if err := boundedText("resource_id", input.Resource.ID, 512); err != nil {
		return err
	}

	if input.Risk == "" {
		return fmt.Errorf(
			"%w: risk is required",
			domain.ErrInvalidArgument,
		)
	}

	if input.EstimatedCostMicroUSD < 0 {
		return fmt.Errorf(
			"%w: estimated_cost_micro_usd must be non-negative",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}
