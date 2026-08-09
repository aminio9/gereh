package application

import (
	"fmt"
	"slices"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"google.golang.org/protobuf/types/known/structpb"
)

var knownConstraintNames = []string{
	"max_cost_micro_usd",
	"max_runtime_seconds",
	"allowed_domains",
	"allowed_resource_ids",
	"require_human_review",
}

func domainConstraints(
	value *structpb.Struct,
) (domain.Constraints, error) {
	result := domain.Constraints{}

	if value == nil || len(value.GetFields()) == 0 {
		return result, nil
	}

	for name := range value.GetFields() {
		if !slices.Contains(knownConstraintNames, name) {
			return domain.Constraints{}, fmt.Errorf(
				"%w: unknown constraint %q",
				domain.ErrInvalidConstraint,
				name,
			)
		}
	}

	var maxCost *int64
	if raw, ok := value.GetFields()["max_cost_micro_usd"]; ok {
		number, err := structInteger(raw)
		if err != nil || number < 0 {
			return domain.Constraints{}, fmt.Errorf(
				"%w: max_cost_micro_usd must be a non-negative integer",
				domain.ErrInvalidConstraint,
			)
		}

		maxCost = &number
	}

	var maxRuntime *int64
	if raw, ok := value.GetFields()["max_runtime_seconds"]; ok {
		number, err := structInteger(raw)
		if err != nil || number < 0 {
			return domain.Constraints{}, fmt.Errorf(
				"%w: max_runtime_seconds must be a non-negative integer",
				domain.ErrInvalidConstraint,
			)
		}

		maxRuntime = &number
	}

	if raw, ok := value.GetFields()["allowed_domains"]; ok {
		domains, err := structStringList(raw)
		if err != nil {
			return domain.Constraints{}, fmt.Errorf(
				"%w: allowed_domains must be a string list",
				domain.ErrInvalidConstraint,
			)
		}

		result.AllowedDomains = domains
	}

	if raw, ok := value.GetFields()["allowed_resource_ids"]; ok {
		resourceIDs, err := structStringList(raw)
		if err != nil {
			return domain.Constraints{}, fmt.Errorf(
				"%w: allowed_resource_ids must be a string list",
				domain.ErrInvalidConstraint,
			)
		}

		result.AllowedResourceIDs = resourceIDs
	}

	if raw, ok := value.GetFields()["require_human_review"]; ok {
		boolean, err := structBoolean(raw)
		if err != nil {
			return domain.Constraints{}, fmt.Errorf(
				"%w: require_human_review must be a boolean",
				domain.ErrInvalidConstraint,
			)
		}

		result.RequireHumanReview = boolean
	}

	result.MaxCostMicroUSD = maxCost
	result.MaxRuntimeSeconds = maxRuntime

	return result, nil
}

func structInteger(
	value *structpb.Value,
) (int64, error) {
	number, ok := value.GetKind().(*structpb.Value_NumberValue)
	if !ok {
		return 0, fmt.Errorf("expected number")
	}

	return int64(number.NumberValue), nil
}

func structBoolean(
	value *structpb.Value,
) (bool, error) {
	boolean, ok := value.GetKind().(*structpb.Value_BoolValue)
	if !ok {
		return false, fmt.Errorf("expected boolean")
	}

	return boolean.BoolValue, nil
}

func structStringList(
	value *structpb.Value,
) ([]string, error) {
	list, ok := value.GetKind().(*structpb.Value_ListValue)
	if !ok {
		return nil, fmt.Errorf("expected list")
	}

	result := make([]string, 0, len(list.ListValue.GetValues()))

	for _, element := range list.ListValue.GetValues() {
		text, ok := element.GetKind().(*structpb.Value_StringValue)
		if !ok {
			return nil, fmt.Errorf("expected string element")
		}

		result = append(result, text.StringValue)
	}

	return result, nil
}
