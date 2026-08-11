// Package provider implements credential verification against
// fixed, first-party provider APIs.
package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
)

const maxProviderResponseBytes = 1024 * 1024

type verifierSpec struct {
	URL string

	ApplyAuthentication func(
		*http.Request,
		[]byte,
	)

	ValidateResponse func(
		[]byte,
	) (int, error)
}

// Verifier verifies credentials against fixed Gereh-owned endpoints.
type Verifier struct {
	client *http.Client

	specs map[string]verifierSpec
}

// NewVerifier constructs a credential verifier.
func NewVerifier(
	timeout time.Duration,
) (*Verifier, error) {
	if timeout <= 0 ||
		timeout > 30*time.Second {
		return nil,
			fmt.Errorf(
				"provider verification timeout must be between 0 and 30 seconds",
			)
	}

	transport :=
		http.DefaultTransport.(*http.Transport).
			Clone()

	transport.TLSClientConfig =
		&tls.Config{
			MinVersion: tls.VersionTLS12,
		}

	return &Verifier{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,

			// Never allow Authorization/API-key headers to follow
			// redirects onto another host.
			CheckRedirect: func(
				_ *http.Request,
				_ []*http.Request,
			) error {
				return http.ErrUseLastResponse
			},
		},
		specs: defaultSpecs(),
	}, nil
}

func defaultSpecs() map[string]verifierSpec {
	return map[string]verifierSpec{
		"openai": {
			URL: "https://api.openai.com/v1/models",

			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set(
					"Authorization",
					"Bearer "+string(key),
				)
			},

			ValidateResponse: validateDataArray,
		},

		"anthropic": {
			URL: "https://api.anthropic.com/v1/models?limit=1",

			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set(
					"X-Api-Key",
					string(key),
				)

				request.Header.Set(
					"anthropic-version",
					"2023-06-01",
				)
			},

			ValidateResponse: validateDataArray,
		},

		"google": {
			URL: "https://generativelanguage.googleapis.com/v1beta/models?pageSize=1",

			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set(
					"x-goog-api-key",
					string(key),
				)
			},

			ValidateResponse: validateModelsArray,
		},

		"openrouter": {
			URL: "https://openrouter.ai/api/v1/key",

			ApplyAuthentication: func(
				request *http.Request,
				key []byte,
			) {
				request.Header.Set(
					"Authorization",
					"Bearer "+string(key),
				)
			},

			ValidateResponse: validateDataObject,
		},
	}
}

// Verify checks a credential against the provider's fixed endpoint.
func (verifier *Verifier) Verify(
	ctx context.Context,
	providerKey string,
	credential []byte,
) (
	ports.ProviderVerification,
	error,
) {
	spec, ok := verifier.specs[providerKey]

	if !ok {
		return ports.ProviderVerification{},
			domain.ErrCredentialVerificationUnsupported
	}

	request, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			spec.URL,
			nil,
		)
	if err != nil {
		return ports.ProviderVerification{},
			fmt.Errorf(
				"build provider verification request: %w",
				err,
			)
	}

	request.Header.Set(
		"Accept",
		"application/json",
	)

	spec.ApplyAuthentication(
		request,
		credential,
	)

	response, err :=
		verifier.client.Do(request)
	if err != nil {
		return ports.ProviderVerification{},
			domain.ErrCredentialVerificationUnavailable
	}

	defer func() { _ = response.Body.Close() }()

	raw, err :=
		io.ReadAll(
			io.LimitReader(
				response.Body,
				maxProviderResponseBytes,
			),
		)
	if err != nil {
		return ports.ProviderVerification{},
			domain.ErrCredentialVerificationUnavailable
	}

	switch {
	case response.StatusCode ==
		http.StatusUnauthorized,
		response.StatusCode ==
			http.StatusForbidden:
		return ports.ProviderVerification{
				HTTPStatus: response.StatusCode,
			},
			domain.ErrCredentialRejected

	// A 402 at a credential/introspection endpoint demonstrates that
	// authentication succeeded but provider billing/credits block use.
	// Keep the connection credential-valid; budget/billing state is a
	// separate concern.
	case response.StatusCode ==
		http.StatusPaymentRequired:
		return ports.ProviderVerification{
			HTTPStatus: response.StatusCode,
		}, nil

	case response.StatusCode ==
		http.StatusRequestTimeout,
		response.StatusCode ==
			http.StatusTooManyRequests,
		response.StatusCode >= 500:
		return ports.ProviderVerification{
				HTTPStatus: response.StatusCode,
			},
			domain.ErrCredentialVerificationUnavailable

	case response.StatusCode <
		200 ||
		response.StatusCode >=
			300:
		// The request format is fixed by Gereh. A new unexpected 4xx
		// therefore indicates provider/API-contract drift, not proof
		// that the customer's key is bad.
		return ports.ProviderVerification{
				HTTPStatus: response.StatusCode,
			},
			domain.ErrCredentialVerificationUnavailable
	}

	count, err :=
		spec.ValidateResponse(raw)
	if err != nil {
		return ports.ProviderVerification{
				HTTPStatus: response.StatusCode,
			},
			domain.ErrCredentialVerificationUnavailable
	}

	return ports.ProviderVerification{
		HTTPStatus:           response.StatusCode,
		DiscoveredModelCount: count,
	}, nil
}

func validateDataArray(
	raw []byte,
) (int, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&response,
		); err != nil {
		return 0, err
	}

	return len(response.Data), nil
}

func validateModelsArray(
	raw []byte,
) (int, error) {
	var response struct {
		Models []json.RawMessage `json:"models"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&response,
		); err != nil {
		return 0, err
	}

	return len(response.Models), nil
}

func validateDataObject(
	raw []byte,
) (int, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&response,
		); err != nil {
		return 0, err
	}

	if len(response.Data) == 0 ||
		string(response.Data) ==
			"null" {
		return 0,
			fmt.Errorf(
				"provider returned no key metadata",
			)
	}

	return 1, nil
}
