// Package secrets implements encrypted external secret storage.
package secrets

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

const (
	maxVaultResponseBytes = 256 * 1024
)

// VaultConfig configures a KV-v2 Vault client.
type VaultConfig struct {
	Address string
	Mount   string

	Namespace string

	TokenFile   string
	StaticToken string

	CAFile string

	AllowInsecureHTTP bool

	Timeout time.Duration
}

// VaultStore implements ports.SecretStore over Vault KV v2.
type VaultStore struct {
	baseURL *url.URL

	mount string

	namespace string

	tokenFile   string
	staticToken string

	client *http.Client
}

// NewVaultStore constructs a KV-v2 Vault client.
func NewVaultStore(
	config VaultConfig,
) (*VaultStore, error) {
	parsed, err :=
		url.Parse(
			strings.TrimSpace(
				config.Address,
			),
		)
	if err != nil {
		return nil,
			fmt.Errorf(
				"parse Vault address: %w",
				err,
			)
	}

	switch {
	case parsed.Scheme == "https":
	case config.AllowInsecureHTTP && parsed.Scheme == "http":
	default:
		return nil,
			fmt.Errorf(
				"vault requires HTTPS",
			)
	}

	if parsed.Host == "" {
		return nil,
			fmt.Errorf(
				"vault host is required",
			)
	}

	mount :=
		strings.Trim(
			strings.TrimSpace(
				config.Mount,
			),
			"/",
		)

	if mount == "" ||
		strings.Contains(
			mount,
			"..",
		) {
		return nil,
			fmt.Errorf(
				"invalid Vault mount",
			)
	}

	if strings.TrimSpace(
		config.TokenFile,
	) == "" &&
		strings.TrimSpace(
			config.StaticToken,
		) == "" {
		return nil,
			fmt.Errorf(
				"vault token source is required",
			)
	}

	if config.Timeout <= 0 {
		config.Timeout =
			5 * time.Second
	}

	rootCAs, err :=
		x509.SystemCertPool()
	if err != nil {
		return nil,
			fmt.Errorf(
				"load system certificate pool: %w",
				err,
			)
	}

	if config.CAFile != "" {
		pemData, err :=
			os.ReadFile(
				config.CAFile,
			)
		if err != nil {
			return nil,
				fmt.Errorf(
					"read Vault CA file: %w",
					err,
				)
		}

		if !rootCAs.
			AppendCertsFromPEM(
				pemData,
			) {
			return nil,
				fmt.Errorf(
					"vault CA file contains no certificates",
				)
		}
	}

	transport :=
		http.DefaultTransport.(*http.Transport).
			Clone()

	transport.TLSClientConfig =
		&tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
		}

	client :=
		&http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
			CheckRedirect: func(
				_ *http.Request,
				_ []*http.Request,
			) error {
				return http.ErrUseLastResponse
			},
		}

	return &VaultStore{
		baseURL:     parsed,
		mount:       mount,
		namespace:   strings.TrimSpace(config.Namespace),
		tokenFile:   strings.TrimSpace(config.TokenFile),
		staticToken: strings.TrimSpace(config.StaticToken),
		client:      client,
	}, nil
}

// Reference builds the opaque secret reference for a connection.
func (store *VaultStore) Reference(
	tenantID string,
	connectionID string,
) string {
	return fmt.Sprintf(
		"vault://%s/tenants/%s/connections/%s",
		store.mount,
		tenantID,
		connectionID,
	)
}

func (store *VaultStore) token() (
	string,
	error,
) {
	if store.tokenFile != "" {
		raw, err :=
			os.ReadFile(
				store.tokenFile,
			)
		if err != nil {
			return "",
				fmt.Errorf(
					"%w: read Vault token file",
					domain.ErrSecretStoreUnavailable,
				)
		}

		token :=
			strings.TrimSpace(
				string(raw),
			)

		if token == "" {
			return "",
				domain.ErrSecretStoreUnavailable
		}

		return token, nil
	}

	if store.staticToken == "" {
		return "",
			domain.ErrSecretStoreUnavailable
	}

	return store.staticToken, nil
}

func (store *VaultStore) secretPath(
	secretRef string,
) (
	string,
	error,
) {
	prefix :=
		"vault://" +
			store.mount +
			"/"

	if !strings.HasPrefix(
		secretRef,
		prefix,
	) {
		return "",
			fmt.Errorf(
				"%w: invalid secret reference",
				domain.ErrInvalidArgument,
			)
	}

	relative :=
		strings.TrimPrefix(
			secretRef,
			prefix,
		)

	clean :=
		path.Clean(relative)

	if clean == "." ||
		clean == ".." ||
		strings.HasPrefix(
			clean,
			"../",
		) ||
		clean != relative {
		return "",
			fmt.Errorf(
				"%w: invalid secret reference",
				domain.ErrInvalidArgument,
			)
	}

	return clean, nil
}

func (store *VaultStore) apiURL(
	operation string,
	secretPath string,
) string {
	value :=
		*store.baseURL

	value.Path =
		path.Join(
			value.Path,
			"v1",
			store.mount,
			operation,
			secretPath,
		)

	return value.String()
}

func (store *VaultStore) request(
	ctx context.Context,
	method string,
	targetURL string,
	body []byte,
) (*http.Response, error) {
	request, err :=
		http.NewRequestWithContext(
			ctx,
			method,
			targetURL,
			bytes.NewReader(body),
		)
	if err != nil {
		return nil,
			fmt.Errorf(
				"build Vault request: %w",
				err,
			)
	}

	token, err :=
		store.token()
	if err != nil {
		return nil, err
	}

	request.Header.Set(
		"X-Vault-Token",
		token,
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	if store.namespace != "" {
		request.Header.Set(
			"X-Vault-Namespace",
			store.namespace,
		)
	}

	response, err :=
		store.client.Do(request)
	if err != nil {
		return nil,
			fmt.Errorf(
				"%w: Vault request failed",
				domain.ErrSecretStoreUnavailable,
			)
	}

	return response, nil
}

func readBounded(
	response *http.Response,
) ([]byte, error) {
	defer func() { _ = response.Body.Close() }()

	return io.ReadAll(
		io.LimitReader(
			response.Body,
			maxVaultResponseBytes,
		),
	)
}

// WriteCAS writes a credential version with KV-v2 Check-And-Set semantics.
func (store *VaultStore) WriteCAS(
	ctx context.Context,
	secretRef string,
	credential []byte,
	expectedVersion int64,
) (int64, error) {
	secretPath, err :=
		store.secretPath(
			secretRef,
		)
	if err != nil {
		return 0, err
	}

	payload :=
		struct {
			Options struct {
				CAS int64 `json:"cas"`
			} `json:"options"`

			Data map[string]string `json:"data"`
		}{}

	payload.Options.CAS =
		expectedVersion

	payload.Data =
		map[string]string{
			"api_key": string(credential),
		}

	body, err :=
		json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	response, err :=
		store.request( //nolint:bodyclose // readBounded closes the body
			ctx,
			http.MethodPost,
			store.apiURL(
				"data",
				secretPath,
			),
			body,
		)
	if err != nil {
		return 0, err
	}

	raw, err :=
		readBounded(response)
	if err != nil {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	if response.StatusCode ==
		http.StatusBadRequest {
		return 0,
			domain.ErrSecretStoreConflict
	}

	if response.StatusCode <
		200 ||
		response.StatusCode >=
			300 {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	var result struct {
		Data struct {
			Version int64 `json:"version"`
		} `json:"data"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&result,
		); err != nil {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	if result.Data.Version <= 0 {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	return result.Data.Version,
		nil
}

// ReadVersion reads a specific credential version from Vault.
func (store *VaultStore) ReadVersion(
	ctx context.Context,
	secretRef string,
	version int64,
) ([]byte, error) {
	secretPath, err :=
		store.secretPath(
			secretRef,
		)
	if err != nil {
		return nil, err
	}

	target, err :=
		url.Parse(
			store.apiURL(
				"data",
				secretPath,
			),
		)
	if err != nil {
		return nil, err
	}

	query :=
		target.Query()

	query.Set(
		"version",
		strconv.FormatInt(
			version,
			10,
		),
	)

	target.RawQuery =
		query.Encode()

	response, err :=
		store.request( //nolint:bodyclose // readBounded closes the body
			ctx,
			http.MethodGet,
			target.String(),
			nil,
		)
	if err != nil {
		return nil, err
	}

	raw, err :=
		readBounded(response)
	if err != nil {
		return nil,
			domain.ErrSecretStoreUnavailable
	}

	if response.StatusCode ==
		http.StatusNotFound {
		return nil,
			domain.ErrSecretNotFound
	}

	if response.StatusCode <
		200 ||
		response.StatusCode >=
			300 {
		return nil,
			domain.ErrSecretStoreUnavailable
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&result,
		); err != nil {
		return nil,
			domain.ErrSecretStoreUnavailable
	}

	apiKey := result.Data.Data["api_key"]

	if apiKey == "" {
		return nil,
			domain.ErrSecretNotFound
	}

	return []byte(apiKey), nil
}

// CurrentVersion returns the current KV-v2 version for a secret.
func (store *VaultStore) CurrentVersion(
	ctx context.Context,
	secretRef string,
) (int64, error) {
	secretPath, err :=
		store.secretPath(
			secretRef,
		)
	if err != nil {
		return 0, err
	}

	response, err :=
		store.request( //nolint:bodyclose // readBounded closes the body
			ctx,
			http.MethodGet,
			store.apiURL(
				"metadata",
				secretPath,
			),
			nil,
		)
	if err != nil {
		return 0, err
	}

	raw, err :=
		readBounded(response)
	if err != nil {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	if response.StatusCode ==
		http.StatusNotFound {
		return 0,
			domain.ErrSecretNotFound
	}

	if response.StatusCode <
		200 ||
		response.StatusCode >=
			300 {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	var result struct {
		Data struct {
			CurrentVersion int64 `json:"current_version"`
		} `json:"data"`
	}

	if err :=
		json.Unmarshal(
			raw,
			&result,
		); err != nil {
		return 0,
			domain.ErrSecretStoreUnavailable
	}

	return result.Data.CurrentVersion,
		nil
}

// DestroyVersion permanently destroys a KV-v2 version.
func (store *VaultStore) DestroyVersion(
	ctx context.Context,
	secretRef string,
	version int64,
) error {
	secretPath, err :=
		store.secretPath(
			secretRef,
		)
	if err != nil {
		return err
	}

	body, err :=
		json.Marshal(
			map[string]any{
				"versions": []int64{version},
			},
		)
	if err != nil {
		return err
	}

	response, err :=
		store.request( //nolint:bodyclose // readBounded closes the body
			ctx,
			http.MethodPost,
			store.apiURL(
				"destroy",
				secretPath,
			),
			body,
		)
	if err != nil {
		return err
	}

	_, _ = readBounded(response)

	if response.StatusCode ==
		http.StatusNotFound {
		return nil
	}

	if response.StatusCode <
		200 ||
		response.StatusCode >=
			300 {
		return domain.ErrSecretStoreUnavailable
	}

	return nil
}

// Purge permanently removes all versions of a secret.
func (store *VaultStore) Purge(
	ctx context.Context,
	secretRef string,
) error {
	secretPath, err :=
		store.secretPath(
			secretRef,
		)
	if err != nil {
		return err
	}

	response, err :=
		store.request( //nolint:bodyclose // readBounded closes the body
			ctx,
			http.MethodDelete,
			store.apiURL(
				"metadata",
				secretPath,
			),
			nil,
		)
	if err != nil {
		return err
	}

	_, _ = readBounded(response)

	if response.StatusCode ==
		http.StatusNotFound {
		return nil
	}

	if response.StatusCode <
		200 ||
		response.StatusCode >=
			300 {
		return domain.ErrSecretStoreUnavailable
	}

	return nil
}
