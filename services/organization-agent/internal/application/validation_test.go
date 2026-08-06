package application

import (
	"strings"
	"testing"

	"github.com/aminio9/gereh/services/organization-agent/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSlug(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{
			name:  "lowercase",
			input: "my-company",
			want:  "my-company",
		},
		{
			name:  "uppercase normalized",
			input: "My-Company",
			want:  "my-company",
		},
		{
			name:  "single character",
			input: "a",
			want:  "a",
		},
		{
			name:      "leading dash",
			input:     "-company",
			wantError: true,
		},
		{
			name:      "trailing dash",
			input:     "company-",
			wantError: true,
		},
		{
			name:      "space separator",
			input:     "comp any",
			wantError: true,
		},
		{
			name:      "empty",
			input:     "   ",
			wantError: true,
		},
		{
			name:  "max length",
			input: strings.Repeat("a", 63),
			want:  strings.Repeat("a", 63),
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 64),
			wantError: true,
		},
	} {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := normalizeSlug(test.input)

			if test.wantError {
				require.ErrorIs(
					t,
					err,
					domain.ErrInvalidArgument,
				)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.want, result)
		})
	}
}

func TestBoundedText(t *testing.T) {
	t.Parallel()

	result, err := boundedText(
		"display_name",
		"  Example  ",
		1,
		120,
	)
	require.NoError(t, err)
	require.Equal(t, "Example", result)

	_, err = boundedText(
		"display_name",
		"",
		1,
		120,
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)

	_, err = boundedText(
		"description",
		strings.Repeat("x", 2001),
		0,
		2000,
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestNormalizeCapabilities(t *testing.T) {
	t.Parallel()

	result, err := normalizeCapabilities(
		[]string{
			"Email",
			"email",
			"  billing  ",
		},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"billing", "email"}, result)

	_, err = normalizeCapabilities(
		make([]string, 65),
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)

	_, err = normalizeCapabilities(
		[]string{strings.Repeat("x", 129)},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)

	_, err = normalizeCapabilities(
		[]string{"  "},
	)
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestValidateConfiguration(t *testing.T) {
	t.Parallel()

	require.NoError(
		t,
		validateConfiguration(nil),
	)

	require.NoError(
		t,
		validateConfiguration(
			map[string]any{
				"model": "gpt-4",
				"nested": map[string]any{
					"temperature": 0.7,
				},
			},
		),
	)

	for _, key := range []string{
		"api_key",
		"apikey",
		"access_token",
		"refresh_token",
		"password",
		"secret",
		"private_key",
		"API_KEY",
	} {
		key := key

		t.Run(key, func(t *testing.T) {
			t.Parallel()

			err := validateConfiguration(
				map[string]any{
					key: "value",
				},
			)

			require.ErrorIs(
				t,
				err,
				domain.ErrInvalidArgument,
			)
		})
	}

	require.ErrorIs(
		t,
		validateConfiguration(
			map[string]any{
				"nested": map[string]any{
					"api_key": "value",
				},
			},
		),
		domain.ErrInvalidArgument,
	)

	large := map[string]any{
		"payload": strings.Repeat("x", 66*1024),
	}

	require.ErrorIs(
		t,
		validateConfiguration(large),
		domain.ErrInvalidArgument,
	)
}

func TestValidateUUID(t *testing.T) {
	t.Parallel()

	require.NoError(
		t,
		validateUUID(
			"tenant_id",
			"018f7767-28d2-7f5c-a693-0bb4c8ee4ae1",
		),
	)

	err := validateUUID("tenant_id", "not-a-uuid")
	require.ErrorIs(t, err, domain.ErrInvalidArgument)
}
