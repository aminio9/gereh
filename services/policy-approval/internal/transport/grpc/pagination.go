package grpc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/aminio9/gereh/services/policy-approval/internal/ports"
)

type policyToken struct {
	PolicyID string `json:"policy_id"`
}

func encodePolicyToken(cursor *ports.PolicyCursor) string {
	if cursor == nil || cursor.PolicyID == "" {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(
		[]byte(cursor.PolicyID),
	)
}

func decodePolicyToken(value string) (*ports.PolicyCursor, error) {
	if value == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid policy page token",
			domain.ErrInvalidArgument,
		)
	}

	var token policyToken
	if err := json.Unmarshal(decoded, &token); err != nil ||
		token.PolicyID == "" {
		return nil, fmt.Errorf(
			"%w: invalid policy page token",
			domain.ErrInvalidArgument,
		)
	}

	return &ports.PolicyCursor{
		PolicyID: token.PolicyID,
	}, nil
}

type decisionToken struct {
	DecidedAt  string `json:"decided_at"`
	DecisionID string `json:"decision_id"`
}

func encodeDecisionToken(cursor *ports.DecisionCursor) string {
	if cursor == nil || cursor.DecisionID == "" {
		return ""
	}

	token := decisionToken{
		DecidedAt:  cursor.DecidedAt.UTC().Format(time.RFC3339Nano),
		DecisionID: cursor.DecisionID,
	}

	encoded, err := json.Marshal(token)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeDecisionToken(
	value string,
) (*ports.DecisionCursor, error) {
	if value == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid decision page token",
			domain.ErrInvalidArgument,
		)
	}

	var token decisionToken
	if err := json.Unmarshal(decoded, &token); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid decision page token",
			domain.ErrInvalidArgument,
		)
	}

	decidedAt, err := time.Parse(
		time.RFC3339Nano,
		token.DecidedAt,
	)
	if err != nil || token.DecisionID == "" {
		return nil, fmt.Errorf(
			"%w: invalid decision page token",
			domain.ErrInvalidArgument,
		)
	}

	return &ports.DecisionCursor{
		DecidedAt:  decidedAt,
		DecisionID: token.DecisionID,
	}, nil
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
