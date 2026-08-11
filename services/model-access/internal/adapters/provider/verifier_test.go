package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestVerifier(t *testing.T, handler http.HandlerFunc, _ string) *Verifier {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	verifier.specs = map[string]verifierSpec{
		"test": {
			URL: server.URL,
			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set("Authorization", "Bearer "+string(key))
			},
			ValidateResponse: validateDataArray,
		},
	}

	return verifier
}

func TestVerifierAcceptsValidCredential(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "Bearer secret-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": [{"id": "gpt-4"}]}`))
		},
		"",
	)

	result, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.HTTPStatus)
	require.Equal(t, 1, result.DiscoveredModelCount)
}

func TestVerifierRejectsUnauthorized(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		},
		"",
	)

	_, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialRejected)
}

func TestVerifierRejectsForbidden(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		},
		"",
	)

	_, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialRejected)
}

func TestVerifierTreats429AsTransient(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "too many", http.StatusTooManyRequests)
		},
		"",
	)

	_, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialVerificationUnavailable)
}

func TestVerifierTreats500AsTransient(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
		"",
	)

	_, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialVerificationUnavailable)
}

func TestVerifierMalformed2xxIsTransient(t *testing.T) {
	verifier := newTestVerifier(
		t,
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not json"))
		},
		"",
	)

	_, err := verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialVerificationUnavailable)
}

func TestVerifierUnsupportedProvider(t *testing.T) {
	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	_, err = verifier.Verify(
		context.Background(),
		"unknown",
		[]byte("secret-key"),
	)
	require.ErrorIs(t, err, domain.ErrCredentialVerificationUnsupported)
}

func TestVerifierDoesNotFollowRedirect(t *testing.T) {
	targetHit := false

	target := httptest.NewServer(
		http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {
				targetHit = true
			},
		),
	)
	defer target.Close()

	source := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusFound)
			},
		),
	)
	defer source.Close()

	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	verifier.specs = map[string]verifierSpec{
		"test": {
			URL: source.URL,
			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set("Authorization", "Bearer "+string(key))
			},
			ValidateResponse: validateDataArray,
		},
	}

	_, err = verifier.Verify(
		context.Background(),
		"test",
		[]byte("secret-key"),
	)
	require.Error(t, err)
	require.False(t, targetHit)
}

func TestVerifierAnthropicHeaders(t *testing.T) {
	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	var captured *http.Request

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				captured = r
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data": [{"id": "claude-3"}]}`))
			},
		),
	)
	defer server.Close()

	verifier.specs = map[string]verifierSpec{
		"anthropic": {
			URL: server.URL,
			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set("X-Api-Key", string(key))
				request.Header.Set("anthropic-version", "2023-06-01")
			},
			ValidateResponse: validateDataArray,
		},
	}

	_, err = verifier.Verify(
		context.Background(),
		"anthropic",
		[]byte("anthropic-key-value"),
	)
	require.NoError(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "anthropic-key-value", captured.Header.Get("X-Api-Key"))
	require.Equal(t, "2023-06-01", captured.Header.Get("anthropic-version"))
}

func TestVerifierGoogleHeader(t *testing.T) {
	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	var captured *http.Request

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				captured = r
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"models": [{"name": "gemini"}]}`))
			},
		),
	)
	defer server.Close()

	verifier.specs = map[string]verifierSpec{
		"google": {
			URL: server.URL,
			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set("x-goog-api-key", string(key))
			},
			ValidateResponse: validateModelsArray,
		},
	}

	_, err = verifier.Verify(
		context.Background(),
		"google",
		[]byte("google-key-value"),
	)
	require.NoError(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "google-key-value", captured.Header.Get("x-goog-api-key"))
}

func TestVerifierOpenRouterDataObject(t *testing.T) {
	verifier, err := NewVerifier(5 * time.Second)
	require.NoError(t, err)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data": {"label": "key"}}`))
			},
		),
	)
	defer server.Close()

	verifier.specs = map[string]verifierSpec{
		"openrouter": {
			URL: server.URL,
			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set("Authorization", "Bearer "+string(key))
			},
			ValidateResponse: validateDataObject,
		},
	}

	_, err = verifier.Verify(
		context.Background(),
		"openrouter",
		[]byte("openrouter-key-value"),
	)
	require.NoError(t, err)
}

func TestDefaultSpecsUseHTTPS(t *testing.T) {
	for key, spec := range defaultSpecs() {
		require.True(
			t,
			strings.HasPrefix(spec.URL, "https://"),
			"provider %s must use HTTPS",
			key,
		)
	}
}
