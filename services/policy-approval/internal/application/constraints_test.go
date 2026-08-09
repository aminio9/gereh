package application

import (
	"testing"

	"github.com/aminio9/gereh/services/policy-approval/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func mustStruct(t *testing.T, fields map[string]any) *structpb.Struct {
	t.Helper()

	value, err := structpb.NewStruct(fields)
	require.NoError(t, err)

	return value
}

func TestDomainConstraintsEmpty(t *testing.T) {
	t.Parallel()

	constraints, err := domainConstraints(nil)
	require.NoError(t, err)
	require.Empty(t, constraints.MaxCostMicroUSD)
	require.Empty(t, constraints.MaxRuntimeSeconds)
	require.Empty(t, constraints.AllowedDomains)
	require.Empty(t, constraints.AllowedResourceIDs)
	require.False(t, constraints.RequireHumanReview)
}

func TestDomainConstraintsAllFields(t *testing.T) {
	t.Parallel()

	value := mustStruct(t, map[string]any{
		"max_cost_micro_usd":   float64(1000),
		"max_runtime_seconds":  float64(120),
		"allowed_domains":      []any{"a.test", "b.test"},
		"allowed_resource_ids": []any{"1"},
		"require_human_review": true,
	})

	constraints, err := domainConstraints(value)
	require.NoError(t, err)

	require.Equal(t, int64(1000), *constraints.MaxCostMicroUSD)
	require.Equal(t, int64(120), *constraints.MaxRuntimeSeconds)
	require.Equal(t, []string{"a.test", "b.test"}, constraints.AllowedDomains)
	require.Equal(t, []string{"1"}, constraints.AllowedResourceIDs)
	require.True(t, constraints.RequireHumanReview)
}

func TestDomainConstraintsUnknownField(t *testing.T) {
	t.Parallel()

	value := mustStruct(t, map[string]any{
		"shadow_mode": true,
	})

	_, err := domainConstraints(value)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidConstraint)
}

func TestDomainConstraintsInvalidTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fields map[string]any
	}{
		{
			name:   "negative cost",
			fields: map[string]any{"max_cost_micro_usd": float64(-1)},
		},
		{
			name:   "runtime as string",
			fields: map[string]any{"max_runtime_seconds": "fast"},
		},
		{
			name:   "domains not a list",
			fields: map[string]any{"allowed_domains": "a.test"},
		},
		{
			name:   "resource ids with non-string",
			fields: map[string]any{"allowed_resource_ids": []any{1}},
		},
		{
			name:   "review not bool",
			fields: map[string]any{"require_human_review": "yes"},
		},
	}

	for _, test := range cases {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				_, err := domainConstraints(mustStruct(t, test.fields))
				require.Error(t, err)
				require.ErrorIs(t, err, domain.ErrInvalidConstraint)
			},
		)
	}
}
