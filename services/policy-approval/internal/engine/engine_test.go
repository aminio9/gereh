package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestCEL(t *testing.T) *CEL {
	t.Helper()

	engine, err := NewCEL()
	require.NoError(t, err)

	return engine
}

func newTestEvaluator(t *testing.T) *Evaluator {
	t.Helper()

	return NewEvaluator(newTestCEL(t))
}

func TestMergeConstraints(t *testing.T) {
	t.Parallel()

	leftCost := int64(200)
	rightCost := int64(50)
	leftRuntime := int64(300)

	left := domain.Constraints{
		MaxCostMicroUSD:    &leftCost,
		MaxRuntimeSeconds:  &leftRuntime,
		AllowedDomains:     []string{"a.test", "b.test"},
		AllowedResourceIDs: []string{"1", "2"},
		RequireHumanReview: false,
	}

	right := domain.Constraints{
		MaxCostMicroUSD:    &rightCost,
		MaxRuntimeSeconds:  nil,
		AllowedDomains:     []string{"b.test", "c.test"},
		AllowedResourceIDs: []string{"2", "3"},
		RequireHumanReview: true,
	}

	merged := MergeConstraints(left, right)

	require.Equal(t, int64(50), *merged.MaxCostMicroUSD)
	require.Equal(t, int64(300), *merged.MaxRuntimeSeconds)
	require.Equal(t, []string{"b.test"}, merged.AllowedDomains)
	require.Equal(t, []string{"2"}, merged.AllowedResourceIDs)
	require.True(t, merged.RequireHumanReview)
}

func TestMergeConstraintsEmptyLeft(t *testing.T) {
	t.Parallel()

	leftCost := int64(50)

	merged := MergeConstraints(
		domain.Constraints{},
		domain.Constraints{MaxCostMicroUSD: &leftCost},
	)

	require.Equal(t, int64(50), *merged.MaxCostMicroUSD)
}

func testUserInput() domain.EvaluationInput {
	return domain.EvaluationInput{
		Action:   "work.task.update",
		Subject:  domain.Subject{Type: domain.SubjectUser, ID: "u0000"},
		Resource: domain.Resource{Type: "task"},
		Risk:     domain.RiskLow,
	}
}

func agentInput(autonomy string, action string) domain.EvaluationInput {
	input := testUserInput()

	input.Action = action

	input.Subject = domain.Subject{
		Type:          domain.SubjectAgent,
		ID:            "018f7767-28d2-7f5c-a693-0bb4c8ee4ae1",
		AgentAutonomy: autonomy,
		AgentStatus:   "active",
	}

	return input
}

func bundle(defaultEffect domain.Effect, rules ...domain.Rule) []domain.ActiveBundle {
	return []domain.ActiveBundle{
		{
			Policy: domain.Policy{
				ID:        "policy-1",
				ScopeType: domain.ScopeTenant,
			},
			Version: domain.PolicyVersion{
				PolicyVersion: 1,
				DefaultEffect: defaultEffect,
				Rules:         rules,
			},
		},
	}
}

func lowAllowRule(priority int32) domain.Rule {
	return domain.Rule{
		ID:             "rule-allow",
		Priority:       priority,
		Enabled:        true,
		ActionPatterns: []string{"*"},
		Effect:         domain.EffectAllow,
		Condition:      "true",
		Reason:         "default allow",
	}
}

func denyRule(priority int32, id string) domain.Rule {
	return domain.Rule{
		ID:             id,
		Priority:       priority,
		Enabled:        true,
		ActionPatterns: []string{"*"},
		Effect:         domain.EffectDeny,
		Condition:      "true",
		Reason:         "hard stop",
	}
}

func TestCELRejectsInvalidExpression(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		expression string
	}{
		{
			name:       "syntax error",
			expression: `request.owner <<<`,
		},
		{
			name:       "non boolean result",
			expression: "request.action",
		},
		{
			name:       "unknown identifier",
			expression: "military.value == 1",
		},
		{
			name:       "oversized expression",
			expression: strings.Repeat("a && b ", 2000) + "c",
		},
	}

	for _, test := range cases {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				err := newTestCEL(t).Validate(test.expression)

				require.Error(t, err)
			},
		)
	}
}

func TestCELAcceptsValidExpression(t *testing.T) {
	t.Parallel()

	err := newTestCEL(t).Validate(
		`request.estimated_cost_micro_usd < 100`,
	)

	require.NoError(t, err)
}

func TestCELEvaluatesTrueCondition(t *testing.T) {
	t.Parallel()

	result, err := newTestCEL(t).Evaluate(
		context.Background(),
		"key-true",
		`request.estimated_cost_micro_usd < 100 && request.risk == "low"`,
		map[string]any{
			"request": map[string]any{
				"estimated_cost_micro_usd": int64(50),
				"risk":                     "low",
			},
		},
	)

	require.NoError(t, err)
	require.True(t, result)
}

func TestCELEvaluatesFalseCondition(t *testing.T) {
	t.Parallel()

	result, err := newTestCEL(t).Evaluate(
		context.Background(),
		"key-false",
		`request.estimated_cost_micro_usd < 100`,
		map[string]any{
			"request": map[string]any{
				"estimated_cost_micro_usd": int64(150),
			},
		},
	)

	require.NoError(t, err)
	require.False(t, result)
}

func TestCELMissingVariableFailsLoudly(t *testing.T) {
	t.Parallel()

	// validation succeeds because resource is a declared variable
	require.NoError(t, newTestCEL(t).Validate(`resource.owner == "admin"`))

	// evaluation without the variable raised returns an error
	_, err := newTestCEL(t).Evaluate(
		context.Background(),
		"key-missing",
		`resource.owner == "admin"`,
		map[string]any{},
	)

	require.Error(t, err)
}

func TestValidateActionPattern(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateActionPattern("work.task.*"))
	require.NoError(t, ValidateActionPattern("read"))

	require.Error(t, ValidateActionPattern(""))
	require.Error(t, ValidateActionPattern("work.*.*"))
	require.Error(t, ValidateActionPattern("work*task"))
}

func TestActionMatches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pattern string
		action  string
		matched bool
	}{
		{
			name:    "exact match",
			pattern: "work.task.update",
			action:  "work.task.update",
			matched: true,
		},
		{
			name:    "exact mismatch",
			pattern: "work.task.update",
			action:  "work.task.delete",
			matched: false,
		},
		{
			name:    "prefix wildcard",
			pattern: "work.task.*",
			action:  "work.task.status_update",
			matched: true,
		},
		{
			name:    "wildcard mismatch",
			pattern: "work.task.*",
			action:  "work.project.update",
			matched: false,
		},
	}

	for _, test := range cases {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				require.Equal(
					t,
					test.matched,
					ActionMatches(test.pattern, test.action),
				)
			},
		)
	}
}

func TestRuleMatchesPrefilter(t *testing.T) {
	t.Parallel()

	cost := int64(100)

	rule := domain.Rule{
		ActionPatterns:               []string{"work.task.*"},
		ResourceTypes:                []string{"task"},
		RiskLevels:                   []domain.Risk{domain.RiskLow},
		MaximumEstimatedCostMicroUSD: &cost,
	}

	matching := domain.EvaluationInput{
		Action:                "work.task.update",
		Resource:              domain.Resource{Type: "task"},
		Risk:                  domain.RiskLow,
		EstimatedCostMicroUSD: 50,
	}

	require.True(t, RuleMatchesPrefilter(rule, matching))
	require.True(t, RuleMatchesPrefilter(rule, matching))

	overCost := matching
	overCost.EstimatedCostMicroUSD = 101
	require.False(t, RuleMatchesPrefilter(rule, overCost))

	wrongRisk := matching
	wrongRisk.Risk = domain.RiskCritical
	require.False(t, RuleMatchesPrefilter(rule, wrongRisk))

	wrongResource := matching
	wrongResource.Resource.Type = "project"
	require.False(t, RuleMatchesPrefilter(rule, wrongResource))

	wrongAction := matching
	wrongAction.Action = "tenant.archive"
	require.False(t, RuleMatchesPrefilter(rule, wrongAction))
}

func TestEvaluateEmptyBundlesDenies(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		testUserInput(),
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectDeny, result.Effect)
}

func TestEvaluateDefaultEffectApplies(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		testUserInput(),
		bundle(domain.EffectRequireApproval),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectRequireApproval, result.Effect)
}

func TestEvaluateDenyBeatsAllow(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		testUserInput(),
		bundle(
			domain.EffectAllow,
			lowAllowRule(1),
			denyRule(500, "rule-deny"),
		),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectDeny, result.Effect)
	require.Equal(t, "rule-deny", *result.MatchedRuleID)
}

func TestEvaluateLowerPriorityDenyCannotOverrideHigherAllow(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		testUserInput(),
		bundle(
			domain.EffectDeny,
			lowAllowRule(10),
			denyRule(5, "rule-deny-low"),
		),
	)

	require.NoError(t, err)

	// Restrictive combination applies: deny from the default effect wins.
	require.Equal(t, domain.EffectDeny, result.Effect)
}

func TestEvaluateConditionalRule(t *testing.T) {
	t.Parallel()

	evaluator := newTestEvaluator(t)

	costRule := domain.Rule{
		ID:             "rule-cost",
		Priority:       10,
		Enabled:        true,
		ActionPatterns: []string{"work.task.*"},
		Effect:         domain.EffectDeny,
		Condition:      `request.estimated_cost_micro_usd > 100`,
		Reason:         "over budget",
	}

	over := testUserInput()
	over.EstimatedCostMicroUSD = 500

	result, err := evaluator.Evaluate(
		context.Background(),
		over,
		bundle(domain.EffectRequireApproval, costRule),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectDeny, result.Effect)

	under := testUserInput()
	under.EstimatedCostMicroUSD = 10

	result, err = evaluator.Evaluate(
		context.Background(),
		under,
		bundle(domain.EffectRequireApproval, costRule),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectRequireApproval, result.Effect)
}

func TestEvaluateRequireApprovalBeatsAllowWithConstraints(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		testUserInput(),
		bundle(
			domain.EffectAllowWithConstraints,
			lowAllowRule(1),
		),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectAllowWithConstraints, result.Effect)
}

func TestEvaluateAutonomyBaselineObserveOnly(t *testing.T) {
	t.Parallel()

	readResult, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		agentInput("observe_only", "work.task.read"),
		bundle(domain.EffectAllow),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectAllowWithConstraints, readResult.Effect)

	writeResult, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		agentInput("observe_only", "work.task.update"),
		bundle(domain.EffectAllow),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectDeny, writeResult.Effect)
}

func TestEvaluateAutonomySuggestsApproval(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		agentInput("suggest", "work.task.update"),
		bundle(domain.EffectAllow),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectRequireApproval, result.Effect)
	require.True(t, result.Constraints.RequireHumanReview)
}

func TestEvaluateUnknownAutonomyDenies(t *testing.T) {
	t.Parallel()

	result, err := newTestEvaluator(t).Evaluate(
		context.Background(),
		agentInput("rogue", "work.task.update"),
		bundle(domain.EffectAllow),
	)

	require.NoError(t, err)
	require.Equal(t, domain.EffectDeny, result.Effect)
}
