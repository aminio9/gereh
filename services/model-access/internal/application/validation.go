package application

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/google/uuid"
)

var providerKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func validateUUID(field string, value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return fmt.Errorf(
			"%w: %s must be a UUID",
			domain.ErrInvalidArgument,
			field,
		)
	}

	return nil
}

func normalizeProviderKey(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))

	if !providerKeyPattern.MatchString(value) {
		return "", fmt.Errorf(
			"%w: invalid provider key",
			domain.ErrInvalidArgument,
		)
	}

	return value, nil
}

func normalizeDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)

	length := utf8.RuneCountInString(value)

	if length < 1 || length > 120 {
		return "", fmt.Errorf(
			"%w: display name must contain 1-120 characters",
			domain.ErrInvalidArgument,
		)
	}

	return value, nil
}

func validateIdempotencyKey(value string) error {
	return validateUUID("idempotency_key", value)
}

// hashCanonical fingerprints the canonical request shape.
//
// Fingerprints deliberately contain no context maps or arbitrary JSON so a
// later secret field cannot be included accidentally.
func hashCanonical(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal request fingerprint: %w", err)
	}

	sum := sha256.Sum256(encoded)

	return sum[:], nil
}

type createFingerprint struct {
	ProviderKey    string `json:"providerKey"`
	ConnectionType string `json:"connectionType"`
	DisplayName    string `json:"displayName"`
}

type updateFingerprint struct {
	ConnectionID    string `json:"connectionId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	DisplayName     string `json:"displayName"`
}

type archiveFingerprint struct {
	ConnectionID    string `json:"connectionId"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
