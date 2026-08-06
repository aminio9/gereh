package postgres

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ScopeKind tells PostgreSQL RLS policies how to interpret the transaction.
type ScopeKind string

const (
	// ScopeKindTenant allows access to exactly one selected tenant.
	ScopeKindTenant ScopeKind = "tenant"

	// ScopeKindPrincipal allows an authenticated principal to discover only
	// their own tenant memberships before selecting a tenant.
	ScopeKindPrincipal ScopeKind = "principal"
)

// PrincipalType identifies the authenticated workload represented by the scope.
type PrincipalType string

// Supported principal types.
const (
	PrincipalTypeUser    PrincipalType = "user"
	PrincipalTypeService PrincipalType = "service"
	PrincipalTypeAgent   PrincipalType = "agent"
)

// ErrInvalidScope rejects malformed or internally inconsistent security scope.
var ErrInvalidScope = errors.New("invalid PostgreSQL security scope")

// Scope is trusted security context copied into transaction-local PostgreSQL
// settings. It must be constructed from authenticated server-side state, not
// directly from arbitrary request-body fields.
type Scope struct {
	Kind          ScopeKind
	TenantID      string
	PrincipalID   string
	PrincipalType PrincipalType
	RequestID     string
	CorrelationID string
}

// TenantScope creates a tenant-bound human-user scope.
func TenantScope(
	tenantID string,
	principalID string,
	requestID string,
	correlationID string,
) Scope {
	return Scope{
		Kind:          ScopeKindTenant,
		TenantID:      strings.TrimSpace(tenantID),
		PrincipalID:   strings.TrimSpace(principalID),
		PrincipalType: PrincipalTypeUser,
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(correlationID),
	}
}

// PrincipalScope creates a pre-tenant user-discovery scope.
func PrincipalScope(
	principalID string,
	requestID string,
	correlationID string,
) Scope {
	return Scope{
		Kind:          ScopeKindPrincipal,
		PrincipalID:   strings.TrimSpace(principalID),
		PrincipalType: PrincipalTypeUser,
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(correlationID),
	}
}

// ServiceTenantScope creates a tenant-bound service workload scope.
//
// Never accept principalID for this function from an incoming request. It
// comes from trusted service configuration.
func ServiceTenantScope(
	tenantID string,
	principalID string,
	requestID string,
	correlationID string,
) Scope {
	return Scope{
		Kind:          ScopeKindTenant,
		TenantID:      strings.TrimSpace(tenantID),
		PrincipalID:   strings.TrimSpace(principalID),
		PrincipalType: PrincipalTypeService,
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(correlationID),
	}
}

// Validate rejects malformed or internally inconsistent security context.
func (scope Scope) Validate() error {
	if _, err := uuid.Parse(scope.PrincipalID); err != nil {
		return fmt.Errorf(
			"%w: malformed principal ID: %w",
			ErrInvalidScope,
			err,
		)
	}

	switch scope.PrincipalType {
	case PrincipalTypeUser,
		PrincipalTypeService,
		PrincipalTypeAgent:
	default:
		return fmt.Errorf(
			"%w: unsupported principal type %q",
			ErrInvalidScope,
			scope.PrincipalType,
		)
	}

	switch scope.Kind {
	case ScopeKindTenant:
		if _, err := uuid.Parse(scope.TenantID); err != nil {
			return fmt.Errorf(
				"%w: malformed tenant ID: %w",
				ErrInvalidScope,
				err,
			)
		}

	case ScopeKindPrincipal:
		if scope.TenantID != "" {
			return fmt.Errorf(
				"%w: principal scope must not contain a tenant ID",
				ErrInvalidScope,
			)
		}

	default:
		return fmt.Errorf(
			"%w: unsupported scope kind %q",
			ErrInvalidScope,
			scope.Kind,
		)
	}

	if len(scope.RequestID) > 256 {
		return fmt.Errorf(
			"%w: request ID exceeds 256 characters",
			ErrInvalidScope,
		)
	}

	if len(scope.CorrelationID) > 256 {
		return fmt.Errorf(
			"%w: correlation ID exceeds 256 characters",
			ErrInvalidScope,
		)
	}

	return nil
}
