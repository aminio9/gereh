package application

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
)

func encodePageToken(value string) string {
	if value == "" {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(
		[]byte(value),
	)
}

func decodePageToken(
	value string,
) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf(
			"%w: invalid page token",
			domain.ErrInvalidArgument,
		)
	}

	return string(decoded), nil
}

func normalizePageSize(value int32) int {
	switch {
	case value <= 0:
		return 50
	case value > 100:
		return 100
	default:
		return int(value)
	}
}
