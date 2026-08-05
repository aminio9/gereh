package application

import "testing"

func TestNormalizeSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{
			input: "Gereh-Team",
			want:  "gereh-team",
		},
		{
			input:   "--invalid",
			wantErr: true,
		},
		{
			input:   "ab",
			wantErr: true,
		},
		{
			input:   "invalid_underscore",
			wantErr: true,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.input,
			func(t *testing.T) {
				t.Parallel()

				actual, err := normalizeSlug(
					test.input,
				)

				if test.wantErr {
					if err == nil {
						t.Fatal(
							"normalizeSlug() error = nil",
						)
					}

					return
				}

				if err != nil {
					t.Fatalf(
						"normalizeSlug() error = %v",
						err,
					)
				}

				if actual != test.want {
					t.Fatalf(
						"normalizeSlug() = %q, want %q",
						actual,
						test.want,
					)
				}
			},
		)
	}
}
