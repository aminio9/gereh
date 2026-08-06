package application

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/aminio9/gereh/services/tenant/internal/domain"
)

var slugExpression = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])$`,
)

func validateUUID(
	name string,
	value string,
) error {
	value = strings.TrimSpace(value)

	parsed, err := uuid.Parse(value)
	if err != nil || parsed.String() != strings.ToLower(value) {
		return fmt.Errorf(
			"%w: %s must be a canonical UUID",
			domain.ErrInvalidArgument,
			name,
		)
	}

	return nil
}

func normalizeSlug(value string) (string, error) {
	value = strings.ToLower(
		strings.TrimSpace(value),
	)

	if !slugExpression.MatchString(value) {
		return "", fmt.Errorf(
			"%w: slug must contain 3-63 lowercase letters, digits, or internal hyphens",
			domain.ErrInvalidArgument,
		)
	}

	return value, nil
}

func validateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)

	if len(value) < 1 || len(value) > 120 {
		return "", fmt.Errorf(
			"%w: display name must contain 1-120 bytes",
			domain.ErrInvalidArgument,
		)
	}

	return value, nil
}

func validateRegion(
	value string,
	allowed map[string]struct{},
) (string, error) {
	value = strings.ToLower(
		strings.TrimSpace(value),
	)

	if _, ok := allowed[value]; !ok {
		return "", fmt.Errorf(
			"%w: unsupported tenant region %q",
			domain.ErrInvalidArgument,
			value,
		)
	}

	return value, nil
}

func validateRetentionDays(value int32) error {
	if value < 1 || value > 3650 {
		return fmt.Errorf(
			"%w: retention days must be between 1 and 3650",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}

func validateOperationError(value domain.OperationError) error {
	if len(value.Code) < 1 || len(value.Code) > 64 {
		return fmt.Errorf(
			"%w: operation error code must contain 1-64 bytes",
			domain.ErrInvalidArgument,
		)
	}

	if len(value.Message) > 512 {
		return fmt.Errorf(
			"%w: operation error message must not exceed 512 bytes",
			domain.ErrInvalidArgument,
		)
	}

	return nil
}
