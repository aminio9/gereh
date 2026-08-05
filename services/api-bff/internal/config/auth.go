// Package config loads and validates API BFF runtime configuration.
package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// AuthConfig defines browser and identity-service authentication settings.
type AuthConfig struct {
	IdentityTarget        string
	IdentityInsecure      bool
	WebOrigin             string
	SessionCookieName     string
	TransactionCookieName string
	CSRFCookieName        string
	CookieSecure          bool
	CookieSameSite        http.SameSite
}

// AuthConfigFromEnv loads BFF authentication configuration.
func AuthConfigFromEnv() (AuthConfig, error) {
	insecure, err := boolEnvironment(
		"IDENTITY_ACCESS_GRPC_INSECURE",
		true,
	)
	if err != nil {
		return AuthConfig{}, err
	}

	cookieSecure, err := boolEnvironment(
		"AUTH_COOKIE_SECURE",
		false,
	)
	if err != nil {
		return AuthConfig{}, err
	}

	config := AuthConfig{
		IdentityTarget: envOrDefault(
			"IDENTITY_ACCESS_GRPC_TARGET",
			"passthrough:///127.0.0.1:18081",
		),
		IdentityInsecure: insecure,
		WebOrigin: envOrDefault(
			"GEREH_WEB_ORIGIN",
			"http://localhost:5173",
		),
		SessionCookieName: envOrDefault(
			"AUTH_SESSION_COOKIE_NAME",
			"gereh_session",
		),
		TransactionCookieName: envOrDefault(
			"AUTH_TRANSACTION_COOKIE_NAME",
			"gereh_oidc_transaction",
		),
		CSRFCookieName: envOrDefault(
			"AUTH_CSRF_COOKIE_NAME",
			"gereh_csrf",
		),
		CookieSecure:   cookieSecure,
		CookieSameSite: http.SameSiteLaxMode,
	}

	webOrigin, err := url.Parse(config.WebOrigin)
	if err != nil {
		return AuthConfig{}, fmt.Errorf(
			"parse GEREH_WEB_ORIGIN: %w",
			err,
		)
	}

	if webOrigin.Scheme == "" || webOrigin.Host == "" {
		return AuthConfig{}, fmt.Errorf(
			"GEREH_WEB_ORIGIN must be an absolute origin",
		)
	}

	if webOrigin.Path != "" && webOrigin.Path != "/" {
		return AuthConfig{}, fmt.Errorf(
			"GEREH_WEB_ORIGIN must not contain a path",
		)
	}

	if strings.HasPrefix(
		config.SessionCookieName,
		"__Host-",
	) && !config.CookieSecure {
		return AuthConfig{}, fmt.Errorf(
			"__Host- cookies require AUTH_COOKIE_SECURE=true",
		)
	}

	return config, nil
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}

	return fallback
}

func boolEnvironment(
	name string,
	fallback bool,
) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}

	return result, nil
}
