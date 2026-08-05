package oidc

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
)

// Config defines the external OpenID Provider client.
type Config struct {
	IssuerURL             string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	Scopes                []string
	SupportedSigningAlgs  []string
	RequireVerifiedEmail  bool
	FetchUserInfo         bool
	RequirePKCEAdvertised bool
	AllowInsecureHTTP     bool
	HTTPTimeout           time.Duration
}

// Validate validates provider configuration.
func (config Config) Validate() error {
	if strings.TrimSpace(config.IssuerURL) == "" {
		return fmt.Errorf("OIDC issuer URL is required")
	}

	if strings.TrimSpace(config.ClientID) == "" {
		return fmt.Errorf("OIDC client ID is required")
	}

	if strings.TrimSpace(config.ClientSecret) == "" {
		return fmt.Errorf("OIDC client secret is required")
	}

	if strings.TrimSpace(config.RedirectURL) == "" {
		return fmt.Errorf("OIDC redirect URL is required")
	}

	if config.HTTPTimeout <= 0 {
		return fmt.Errorf("OIDC HTTP timeout must be greater than zero")
	}

	issuer, err := url.Parse(config.IssuerURL)
	if err != nil {
		return fmt.Errorf("parse OIDC issuer URL: %w", err)
	}

	redirect, err := url.Parse(config.RedirectURL)
	if err != nil {
		return fmt.Errorf("parse OIDC redirect URL: %w", err)
	}

	if !config.AllowInsecureHTTP {
		if issuer.Scheme != "https" {
			return fmt.Errorf("OIDC issuer must use HTTPS")
		}

		if redirect.Scheme != "https" {
			return fmt.Errorf("OIDC redirect URL must use HTTPS")
		}
	}

	if len(config.Scopes) == 0 {
		return fmt.Errorf("at least one OIDC scope is required")
	}

	if !slices.Contains(config.Scopes, gooidc.ScopeOpenID) {
		return fmt.Errorf("OIDC scopes must include openid")
	}

	return nil
}
