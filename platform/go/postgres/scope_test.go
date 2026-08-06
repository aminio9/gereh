package postgres

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTenantScopeValidation(t *testing.T) {
	t.Parallel()

	scope := TenantScope(
		uuid.NewString(),
		uuid.NewString(),
		"request-1",
		"correlation-1",
	)

	if err := scope.Validate(); err != nil {
		t.Fatalf("validate tenant scope: %v", err)
	}
}

func TestPrincipalScopeRejectsTenantID(t *testing.T) {
	t.Parallel()

	scope := PrincipalScope(
		uuid.NewString(),
		"",
		"",
	)

	scope.TenantID = uuid.NewString()

	err := scope.Validate()
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf(
			"error = %v, want ErrInvalidScope",
			err,
		)
	}
}

func TestScopeRejectsMalformedPrincipal(t *testing.T) {
	t.Parallel()

	scope := PrincipalScope(
		"not-a-uuid",
		"",
		"",
	)

	err := scope.Validate()
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf(
			"error = %v, want ErrInvalidScope",
			err,
		)
	}
}

func TestScopeRejectsUnsupportedPrincipalType(t *testing.T) {
	t.Parallel()

	scope := TenantScope(
		uuid.NewString(),
		uuid.NewString(),
		"",
		"",
	)

	scope.PrincipalType = "robot"

	err := scope.Validate()
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf(
			"error = %v, want ErrInvalidScope",
			err,
		)
	}
}

func TestScopeRejectsOversizedIdentifiers(t *testing.T) {
	t.Parallel()

	scope := TenantScope(
		uuid.NewString(),
		uuid.NewString(),
		string(make([]byte, 257)),
		"",
	)

	err := scope.Validate()
	if !errors.Is(err, ErrInvalidScope) {
		t.Fatalf(
			"error = %v, want ErrInvalidScope",
			err,
		)
	}
}
