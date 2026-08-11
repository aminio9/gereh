package grpc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/google/uuid"
)

type connectionToken struct {
	ConnectionID string `json:"connectionId"`
}

func decodeConnectionToken(value string) (*ports.ConnectionCursor, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid page token",
			domain.ErrInvalidArgument,
		)
	}

	var token connectionToken

	if err := json.Unmarshal(decoded, &token); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid page token",
			domain.ErrInvalidArgument,
		)
	}

	if _, err := uuid.Parse(token.ConnectionID); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid page token",
			domain.ErrInvalidArgument,
		)
	}

	return &ports.ConnectionCursor{
		ConnectionID: token.ConnectionID,
	}, nil
}

func encodeConnectionToken(cursor *ports.ConnectionCursor) string {
	if cursor == nil {
		return ""
	}

	value, _ := json.Marshal(connectionToken{
		ConnectionID: cursor.ConnectionID,
	})

	return base64.RawURLEncoding.EncodeToString(value)
}

func normalizePageSize(value int32) int {
	switch {
	case value <= 0:
		return 25

	case value > 100:
		return 100

	default:
		return int(value)
	}
}
