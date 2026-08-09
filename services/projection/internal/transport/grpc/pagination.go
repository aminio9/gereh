package grpc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	projectionv1 "github.com/aminio9/gereh/gen/go/gereh/projection/v1"
	"github.com/aminio9/gereh/services/projection/internal/domain"
	"github.com/aminio9/gereh/services/projection/internal/ports"
)

const (
	paginationKindAgent    = "agent-v1"
	paginationKindActivity = "activity-v1"
	paginationKindSearch   = "search-v1"
)

type encodedCursor struct {
	Kind string  `json:"k"`
	Time string  `json:"t,omitempty"`
	ID   string  `json:"i,omitempty"`
	Rank float64 `json:"r,omitempty"`
	Type string  `json:"y,omitempty"`
}

func encodeCursor(cursor encodedCursor) string {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}

	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(token string) (*encodedCursor, error) {
	if token == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, domain.ErrInvalidArgument
	}

	var cursor encodedCursor

	if err := json.Unmarshal(raw, &cursor); err != nil {
		return nil, domain.ErrInvalidArgument
	}

	return &cursor, nil
}

func parseAgentCursor(token string) (*ports.AgentCursor, error) {
	cursor, err := decodeCursor(token)
	if err != nil {
		return nil, err
	}

	if cursor == nil {
		return nil, nil
	}

	if cursor.Kind != paginationKindAgent {
		return nil, domain.ErrInvalidArgument
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, cursor.Time)
	if err != nil {
		return nil, domain.ErrInvalidArgument
	}

	return &ports.AgentCursor{
		UpdatedAt: updatedAt,
		AgentID:   cursor.ID,
	}, nil
}

func encodeAgentCursor(cursor *ports.AgentCursor) string {
	if cursor == nil {
		return ""
	}

	return encodeCursor(encodedCursor{
		Kind: paginationKindAgent,
		Time: cursor.UpdatedAt.Format(time.RFC3339Nano),
		ID:   cursor.AgentID,
	})
}

func parseActivityCursor(token string) (*ports.ActivityCursor, error) {
	cursor, err := decodeCursor(token)
	if err != nil {
		return nil, err
	}

	if cursor == nil {
		return nil, nil
	}

	if cursor.Kind != paginationKindActivity {
		return nil, domain.ErrInvalidArgument
	}

	occurredAt, err := time.Parse(time.RFC3339Nano, cursor.Time)
	if err != nil {
		return nil, domain.ErrInvalidArgument
	}

	return &ports.ActivityCursor{
		OccurredAt: occurredAt,
		EventID:    cursor.ID,
	}, nil
}

func encodeActivityCursor(cursor *ports.ActivityCursor) string {
	if cursor == nil {
		return ""
	}

	return encodeCursor(encodedCursor{
		Kind: paginationKindActivity,
		Time: cursor.OccurredAt.Format(time.RFC3339Nano),
		ID:   cursor.EventID,
	})
}

func parseSearchCursor(token string) (*ports.SearchCursor, error) {
	cursor, err := decodeCursor(token)
	if err != nil {
		return nil, err
	}

	if cursor == nil {
		return nil, nil
	}

	if cursor.Kind != paginationKindSearch {
		return nil, domain.ErrInvalidArgument
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, cursor.Time)
	if err != nil {
		return nil, domain.ErrInvalidArgument
	}

	return &ports.SearchCursor{
		Rank:      cursor.Rank,
		UpdatedAt: updatedAt,
		Type:      cursor.Type,
		ID:        cursor.ID,
	}, nil
}

func encodeSearchCursor(cursor *ports.SearchCursor) string {
	if cursor == nil {
		return ""
	}

	if cursor.Type == "" {
		cursor.Type = "task"
	}

	return encodeCursor(encodedCursor{
		Kind: paginationKindSearch,
		Time: cursor.UpdatedAt.Format(time.RFC3339Nano),
		ID:   cursor.ID,
		Rank: cursor.Rank,
		Type: cursor.Type,
	})
}

func normalizePageSize(value int32) int {
	const (
		defaultPageSize = 25
		maxPageSize     = 100
	)

	pageSize := int(value)

	switch {
	case pageSize <= 0:
		return defaultPageSize

	case pageSize > maxPageSize:
		return maxPageSize

	default:
		return pageSize
	}
}

func mapSearchTypes(
	values []projectionv1.SearchDocumentType,
) ([]string, error) {
	documentTypes := make([]string, 0, len(values))

	for _, value := range values {
		documentType, err := documentTypeName(value)
		if err != nil {
			return nil, err
		}

		documentTypes = append(documentTypes, documentType)
	}

	return documentTypes, nil
}

func documentTypeName(
	value projectionv1.SearchDocumentType,
) (string, error) {
	switch value {
	case projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_COMPANY:
		return "company", nil

	case projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_AGENT:
		return "agent", nil

	case projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_GOAL:
		return "goal", nil

	case projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_PROJECT:
		return "project", nil

	case projectionv1.SearchDocumentType_SEARCH_DOCUMENT_TYPE_TASK:
		return "task", nil

	default:
		return "", fmt.Errorf(
			"unsupported search document type %v",
			value,
		)
	}
}
