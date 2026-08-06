package application

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(
	`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`,
)

func validateUUID(
	name string,
	value string,
) error {
	if _, err := uuid.Parse(
		strings.TrimSpace(value),
	); err != nil {
		return fmt.Errorf(
			"%w: %s must be a UUID",
			domain.ErrInvalidArgument,
			name,
		)
	}

	return nil
}

func normalizeSlug(
	value string,
) (string, error) {
	value = strings.ToLower(
		strings.TrimSpace(value),
	)

	if !slugPattern.MatchString(value) {
		return "", fmt.Errorf(
			"%w: invalid slug",
			domain.ErrInvalidArgument,
		)
	}

	return value, nil
}

func boundedText(
	name string,
	value string,
	minimum int,
	maximum int,
) (string, error) {
	value = strings.TrimSpace(value)
	length := len([]rune(value))

	if length < minimum ||
		length > maximum {
		return "", fmt.Errorf(
			"%w: %s must contain %d-%d characters",
			domain.ErrInvalidArgument,
			name,
			minimum,
			maximum,
		)
	}

	return value, nil
}

func normalizeCapabilities(
	values []string,
) ([]string, error) {
	if len(values) > 64 {
		return nil, fmt.Errorf(
			"%w: no more than 64 capabilities are permitted",
			domain.ErrInvalidArgument,
		)
	}

	result := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.ToLower(
			strings.TrimSpace(value),
		)

		if value == "" ||
			len(value) > 128 {
			return nil, fmt.Errorf(
				"%w: invalid capability",
				domain.ErrInvalidArgument,
			)
		}

		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}

	slices.Sort(result)

	return result, nil
}

func validateConfiguration(
	value map[string]any,
) error {
	if value == nil {
		return nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf(
			"%w: configuration is not valid JSON",
			domain.ErrInvalidArgument,
		)
	}

	if len(encoded) > 64*1024 {
		return fmt.Errorf(
			"%w: configuration exceeds 64 KiB",
			domain.ErrInvalidArgument,
		)
	}

	forbiddenKeys := []string{
		"api_key",
		"apikey",
		"access_token",
		"refresh_token",
		"password",
		"secret",
		"private_key",
	}

	var inspect func(map[string]any) error

	inspect = func(current map[string]any) error {
		for key, raw := range current {
			normalized := strings.ToLower(key)

			if slices.Contains(
				forbiddenKeys,
				normalized,
			) {
				return fmt.Errorf(
					"%w: configuration must not contain secrets",
					domain.ErrInvalidArgument,
				)
			}

			switch nested := raw.(type) {
			case map[string]any:
				if err := inspect(nested); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return inspect(value)
}
