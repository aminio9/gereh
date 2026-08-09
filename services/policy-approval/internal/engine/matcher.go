package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// ValidateActionPattern validates an action pattern.
func ValidateActionPattern(pattern string) error {
	pattern = strings.TrimSpace(pattern)

	if pattern == "" || len(pattern) > 256 {
		return fmt.Errorf(
			"%w: invalid action pattern",
			domain.ErrInvalidArgument,
		)
	}

	if strings.Count(pattern, "*") > 1 {
		return fmt.Errorf(
			"%w: action pattern supports at most one wildcard",
			domain.ErrInvalidArgument,
		)
	}

	if strings.Contains(pattern, "*") &&
		!strings.HasSuffix(pattern, "*") {
		return fmt.Errorf(
			"%w: wildcard must be the final character",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

// ActionMatches reports whether an action matches an action pattern.
func ActionMatches(pattern string, action string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(
			action,
			strings.TrimSuffix(pattern, "*"),
		)
	}

	return pattern == action
}

// RuleMatchesPrefilter checks cheap, non-CEL rule criteria.
func RuleMatchesPrefilter(
	rule domain.Rule,
	input domain.EvaluationInput,
) bool {
	actionMatched := false

	for _, pattern := range rule.ActionPatterns {
		if ActionMatches(pattern, input.Action) {
			actionMatched = true
			break
		}
	}

	if !actionMatched {
		return false
	}

	if len(rule.ResourceTypes) > 0 &&
		!slices.Contains(rule.ResourceTypes, input.Resource.Type) {
		return false
	}

	if len(rule.RiskLevels) > 0 &&
		!slices.Contains(rule.RiskLevels, input.Risk) {
		return false
	}

	if rule.MaximumEstimatedCostMicroUSD != nil &&
		input.EstimatedCostMicroUSD > *rule.MaximumEstimatedCostMicroUSD {
		return false
	}

	return true
}
