// Package secrets provides Vault client for Model Gateway secret retrieval.
package secrets

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const maxVaultResponseBytes = 256 * 1024

// VaultConfig configures the Model Gateway Vault client.
type VaultConfig struct {
	Address           string
	Mount             string
	Namespace         string
	TokenFile         string
	StaticToken       string
	CAFile            string
	AllowInsecureHTTP bool
	Timeout           time.Duration
}

// VaultStore retrieves decrypted BYOK API keys from Vault.
type VaultStore struct {
	baseURL     *url.URL
	mount       string
	namespace   string
	tokenFile   string
	staticToken string
	client      *http.Client
}

// NewVaultStore constructs a new Model Gateway Vault client.
func NewVaultStore(config VaultConfig) (*VaultStore, error) {
	parsed, err := url.Parse(strings.TrimSpace(config.Address))
	if err != nil {
		return nil, fmt.Errorf("parse Vault address: %w", err)
	}

	switch {
	case parsed.Scheme == "https":
	case config.AllowInsecureHTTP && parsed.Scheme == "http":
	default:
		return nil, errors.New("vault requires HTTPS")
	}

	if parsed.Host == "" {
		return nil, errors.New("vault host is required")
	}

	mount := strings.Trim(strings.TrimSpace(config.Mount), "/")
	if mount == "" || strings.Contains(mount, "..") {
		return nil, errors.New("invalid Vault mount")
	}

	if strings.TrimSpace(config.TokenFile) == "" && strings.TrimSpace(config.StaticToken) == "" {
		return nil, errors.New("vault token source is required")
	}

	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Second
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate pool: %w", err)
	}

	if config.CAFile != "" {
		pemData, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read Vault CA file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(pemData) {
			return nil, errors.New("vault CA file contains no certificates")
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
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

func (s *VaultStore) token() (string, error) {
	if s.tokenFile != "" {
		raw, err := os.ReadFile(s.tokenFile)
		if err != nil {
			return "", fmt.Errorf("read Vault token file: %w", err)
		}
		t := strings.TrimSpace(string(raw))
		if t == "" {
			return "", errors.New("vault token is empty")
		}
		return t, nil
	}
	if s.staticToken == "" {
		return "", errors.New("vault static token is empty")
	}
	return s.staticToken, nil
}

// GetBYOKSecret retrieves the active API key for a tenant connection from Vault KV-v2.
func (s *VaultStore) GetBYOKSecret(
	ctx context.Context,
	tenantID string,
	connectionID string,
) ([]byte, error) {
	secretPath := fmt.Sprintf("tenants/%s/connections/%s", tenantID, connectionID)

	target := *s.baseURL
	target.Path = path.Join(target.Path, "v1", s.mount, "data", secretPath)

	token, err := s.token()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build Vault request: %w", err)
	}

	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")
	if s.namespace != "" {
		req.Header.Set("X-Vault-Namespace", s.namespace)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do Vault request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("credential secret not found in Vault")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vault returned error status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxVaultResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read Vault response: %w", err)
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal Vault response: %w", err)
	}

	apiKey := result.Data.Data["api_key"]
	if apiKey == "" {
		return nil, errors.New("api_key field missing in Vault secret data")
	}

	return []byte(apiKey), nil
}
