package engine

import (
	"slices"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
)

// MergeConstraints merges two constraint sets using restrictive precedence.
func MergeConstraints(
	current domain.Constraints,
	incoming domain.Constraints,
) domain.Constraints {
	result := current

	result.MaxCostMicroUSD = minimumInt64(
		current.MaxCostMicroUSD,
		incoming.MaxCostMicroUSD,
	)

	result.MaxRuntimeSeconds = minimumInt64(
		current.MaxRuntimeSeconds,
		incoming.MaxRuntimeSeconds,
	)

	result.AllowedDomains = intersection(
		current.AllowedDomains,
		incoming.AllowedDomains,
	)

	result.AllowedResourceIDs = intersection(
		current.AllowedResourceIDs,
		incoming.AllowedResourceIDs,
	)

	result.RequireHumanReview =
		current.RequireHumanReview || incoming.RequireHumanReview

	return result
}

func minimumInt64(left *int64, right *int64) *int64 {
	switch {
	case left == nil:
		return cloneInt64(right)

	case right == nil:
		return cloneInt64(left)

	case *left <= *right:
		return cloneInt64(left)

	default:
		return cloneInt64(right)
	}
}

func intersection(left []string, right []string) []string {
	if len(left) == 0 {
		return slices.Clone(right)
	}

	if len(right) == 0 {
		return slices.Clone(left)
	}

	result := make([]string, 0)

	for _, value := range left {
		if slices.Contains(right, value) &&
			!slices.Contains(result, value) {
			result = append(result, value)
		}
	}

	slices.Sort(result)

	return result
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}

	copyValue := *value
	return &copyValue
}
