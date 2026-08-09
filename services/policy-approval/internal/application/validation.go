package application

import (
	"fmt"
	"strings"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/google/uuid"
)

func newUUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf(
			"generate UUIDv7: %w",
			err,
		)
	}

	return id.String(), nil
}

func validateUUID(
	field string,
	value string,
) error {
	value = strings.TrimSpace(value)

	if value == "" {
		return fmt.Errorf(
			"%w: %s is required",
			domain.ErrInvalidArgument,
			field,
		)
	}

	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf(
			"%w: %s must be a UUID",
			domain.ErrInvalidArgument,
			field,
		)
	}

	return nil
}

func boundedText(
	field string,
	value string,
	maxLength int,
) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf(
			"%w: %s is required",
			domain.ErrInvalidArgument,
			field,
		)
	}

	if len(value) > maxLength {
		return fmt.Errorf(
			"%w: %s exceeds %d characters",
			domain.ErrInvalidArgument,
			field,
			maxLength,
		)
	}

	return nil
}

func optionalBoundedText(
	field string,
	value string,
	maxLength int,
) error {
	if value == "" {
		return nil
	}

	if len(value) > maxLength {
		return fmt.Errorf(
			"%w: %s exceeds %d characters",
			domain.ErrInvalidArgument,
			field,
			maxLength,
		)
	}

	return nil
}
