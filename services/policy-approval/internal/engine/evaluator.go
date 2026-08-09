package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// Result is the restrictive outcome of an evaluation.
type Result struct {
	Effect      domain.Effect
	Constraints domain.Constraints
	Reason      string

	MatchedPolicyID      *string
	MatchedPolicyVersion *int64
	MatchedRuleID        *string
}

// Evaluator applies priority ordering and restrictive combination.
type Evaluator struct {
	cel *CEL
}

// NewEvaluator creates the policy evaluator.
func NewEvaluator(celEngine *CEL) *Evaluator {
	return &Evaluator{
		cel: celEngine,
	}
}

// Evaluate evaluates every active policy bundle and returns the restrictive
// outcome.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	input domain.EvaluationInput,
	bundles []domain.ActiveBundle,
) (Result, error) {
	result := autonomyBaseline(input)

	for _, bundle := range bundles {
		defaultResult := Result{
			Effect: bundle.Version.DefaultEffect,
			Reason: "Policy default effect applied",
			MatchedPolicyID: stringPointer(
				bundle.Policy.ID,
			),
			MatchedPolicyVersion: int64Pointer(
				bundle.Version.PolicyVersion,
			),
		}

		result = combine(result, defaultResult)

		rules := slices.Clone(bundle.Version.Rules)

		slices.SortFunc(
			rules,
			func(left, right domain.Rule) int {
				switch {
				case left.Priority < right.Priority:
					return -1
				case left.Priority > right.Priority:
					return 1
				case left.ID < right.ID:
					return -1
				case left.ID > right.ID:
					return 1
				default:
					return 0
				}
			},
		)

		for _, rule := range rules {
			if !rule.Enabled ||
				!RuleMatchesPrefilter(rule, input) {
				continue
			}

			matched, err := evaluator.cel.Evaluate(
				ctx,
				fmt.Sprintf(
					"%s:%d:%s:%s",
					bundle.Policy.ID,
					bundle.Version.PolicyVersion,
					rule.ID,
					rule.Condition,
				),
				rule.Condition,
				celVariables(input),
			)
			if err != nil {
				return Result{}, err
			}

			if !matched {
				continue
			}

			ruleResult := Result{
				Effect:      rule.Effect,
				Constraints: rule.Constraints,
				Reason:      rule.Reason,

				MatchedPolicyID: stringPointer(
					bundle.Policy.ID,
				),
				MatchedPolicyVersion: int64Pointer(
					bundle.Version.PolicyVersion,
				),
				MatchedRuleID: stringPointer(rule.ID),
			}

			result = combine(result, ruleResult)

			if result.Effect == domain.EffectDeny {
				return result, nil
			}
		}
	}

	if len(bundles) == 0 && result.Effect == "" {
		return Result{
			Effect: domain.EffectDeny,
			Reason: "No active policy allows this action",
		}, nil
	}

	if result.Effect == "" {
		result.Effect = domain.EffectDeny
		result.Reason = "No matching policy rule"
	}

	return result, nil
}

// autonomyBaseline applies the built-in agent autonomy guardrail.
func autonomyBaseline(input domain.EvaluationInput) Result {
	if input.Subject.Type != domain.SubjectAgent {
		return Result{}
	}

	switch input.Subject.AgentAutonomy {
	case "observe_only":
		if isReadAction(input.Action) {
			return Result{
				Effect: domain.EffectAllowWithConstraints,
				Constraints: domain.Constraints{
					RequireHumanReview: false,
				},
				Reason: "Agent autonomy permits read actions only",
			}
		}

		return Result{
			Effect: domain.EffectDeny,
			Reason: "Agent autonomy is observe-only",
		}

	case "suggest":
		return Result{
			Effect: domain.EffectRequireApproval,
			Constraints: domain.Constraints{
				RequireHumanReview: true,
			},
			Reason: "Agent autonomy permits suggestions only",
		}

	case "approval_required":
		return Result{
			Effect: domain.EffectRequireApproval,
			Constraints: domain.Constraints{
				RequireHumanReview: true,
			},
			Reason: "Agent autonomy requires approval",
		}

	case "policy_bounded":
		return Result{}

	default:
		return Result{
			Effect: domain.EffectDeny,
			Reason: "Agent autonomy is unknown",
		}
	}
}

func isReadAction(action string) bool {
	return strings.HasPrefix(action, "read.") ||
		strings.HasSuffix(action, ".read") ||
		strings.HasPrefix(action, "observe.") ||
		strings.HasPrefix(action, "research.")
}

func combine(current Result, incoming Result) Result {
	if incoming.Effect == "" {
		return current
	}

	if current.Effect == "" {
		return incoming
	}

	if severity(incoming.Effect) > severity(current.Effect) {
		incoming.Constraints = MergeConstraints(
			current.Constraints,
			incoming.Constraints,
		)

		return incoming
	}

	current.Constraints = MergeConstraints(
		current.Constraints,
		incoming.Constraints,
	)

	return current
}

func severity(effect domain.Effect) int {
	switch effect {
	case domain.EffectDeny:
		return 4

	case domain.EffectRequireApproval:
		return 3

	case domain.EffectAllowWithConstraints:
		return 2

	case domain.EffectAllow:
		return 1

	default:
		return 0
	}
}

func celVariables(input domain.EvaluationInput) map[string]any {
	return map[string]any{
		"subject": map[string]any{
			"type":         string(input.Subject.Type),
			"id":           input.Subject.ID,
			"company_id":   valueOrEmpty(input.Subject.CompanyID),
			"autonomy":     input.Subject.AgentAutonomy,
			"status":       input.Subject.AgentStatus,
			"capabilities": input.Subject.AgentCapabilities,
			"version":      input.Subject.AgentVersion,
		},
		"resource": map[string]any{
			"type":       input.Resource.Type,
			"id":         input.Resource.ID,
			"attributes": input.Resource.Attributes,
		},
		"request": map[string]any{
			"action":                   input.Action,
			"risk":                     string(input.Risk),
			"estimated_cost_micro_usd": input.EstimatedCostMicroUSD,
		},
		"context": input.Context,
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func stringPointer(value string) *string {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
