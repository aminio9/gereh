// Package config loads and validates identity-access runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	identityoidc "github.com/aminio9/gereh/services/identity-access/internal/adapters/oidc"
)

// Config defines identity-access runtime configuration.
type Config struct {
	ServiceName      string
	ServiceVersion   string
	Environment      string
	GRPCAddress      string
	DatabaseURL      string
	RedisURL         string
	RedisPrefix      string
	TransactionTTL   time.Duration
	SessionTTL       time.Duration
	ShutdownTimeout  time.Duration
	PostgresMaxConns int32
	PostgresMinConns int32
	OIDC             identityoidc.Config
}

// FromEnv loads identity-access configuration.
func FromEnv() (Config, error) {
	environment := envOrDefault(
		"GEREH_ENVIRONMENT",
		"development",
	)

	allowInsecureDefault := environment != "production"

	allowInsecure, err := boolEnvironment(
		"OIDC_ALLOW_INSECURE_HTTP",
		allowInsecureDefault,
	)
	if err != nil {
		return Config{}, err
	}

	requireVerifiedEmail, err := boolEnvironment(
		"OIDC_REQUIRE_VERIFIED_EMAIL",
		true,
	)
	if err != nil {
		return Config{}, err
	}

	fetchUserInfo, err := boolEnvironment(
		"OIDC_FETCH_USERINFO",
		true,
	)
	if err != nil {
		return Config{}, err
	}

	requirePKCE, err := boolEnvironment(
		"OIDC_REQUIRE_PKCE_ADVERTISED",
		true,
	)
	if err != nil {
		return Config{}, err
	}

	transactionTTL, err := durationEnvironment(
		"OIDC_TRANSACTION_TTL",
		10*time.Minute,
	)
	if err != nil {
		return Config{}, err
	}

	sessionTTL, err := durationEnvironment(
		"AUTH_SESSION_TTL",
		8*time.Hour,
	)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationEnvironment(
		"SHUTDOWN_TIMEOUT",
		15*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	httpTimeout, err := durationEnvironment(
		"OIDC_HTTP_TIMEOUT",
		10*time.Second,
	)
	if err != nil {
		return Config{}, err
	}

	maxConns, err := int32Environment(
		"POSTGRES_MAX_CONNS",
		10,
	)
	if err != nil {
		return Config{}, err
	}

	minConns, err := int32Environment(
		"POSTGRES_MIN_CONNS",
		2,
	)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		ServiceName: "identity-access",
		ServiceVersion: envOrDefault(
			"SERVICE_VERSION",
			"dev",
		),
		Environment: environment,
		GRPCAddress: envOrDefault(
			"IDENTITY_GRPC_ADDRESS",
			":18081",
		),
		DatabaseURL: strings.TrimSpace(
			os.Getenv("DATABASE_URL"),
		),
		RedisURL: strings.TrimSpace(
			os.Getenv("REDIS_URL"),
		),
		RedisPrefix: envOrDefault(
			"IDENTITY_REDIS_PREFIX",
			"gereh:iam",
		),
		TransactionTTL:   transactionTTL,
		SessionTTL:       sessionTTL,
		ShutdownTimeout:  shutdownTimeout,
		PostgresMaxConns: maxConns,
		PostgresMinConns: minConns,
		OIDC: identityoidc.Config{
			IssuerURL: strings.TrimSpace(
				os.Getenv("OIDC_ISSUER_URL"),
			),
			ClientID: strings.TrimSpace(
				os.Getenv("OIDC_CLIENT_ID"),
			),
			ClientSecret: os.Getenv(
				"OIDC_CLIENT_SECRET",
			),
			RedirectURL: strings.TrimSpace(
				os.Getenv("OIDC_REDIRECT_URL"),
			),
			Scopes: splitCommaSeparated(
				envOrDefault(
					"OIDC_SCOPES",
					"openid,profile,email",
				),
			),
			SupportedSigningAlgs: splitCommaSeparated(
				envOrDefault(
					"OIDC_SUPPORTED_SIGNING_ALGS",
					"RS256,ES256",
				),
			),
			RequireVerifiedEmail:  requireVerifiedEmail,
			FetchUserInfo:         fetchUserInfo,
			RequirePKCEAdvertised: requirePKCE,
			AllowInsecureHTTP:     allowInsecure,
			HTTPTimeout:           httpTimeout,
		},
	}

	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf(
			"DATABASE_URL is required",
		)
	}

	if config.RedisURL == "" {
		return Config{}, fmt.Errorf(
			"REDIS_URL is required",
		)
	}

	if config.PostgresMinConns > config.PostgresMaxConns {
		return Config{}, fmt.Errorf(
			"POSTGRES_MIN_CONNS cannot exceed POSTGRES_MAX_CONNS",
		)
	}

	if err := config.OIDC.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}

	return fallback
}

func splitCommaSeparated(value string) []string {
	var values []string

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)

		if item != "" {
			values = append(values, item)
		}
	}

	return values
}

func durationEnvironment(
	name string,
	fallback time.Duration,
) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return duration, nil
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

func int32Environment(
	name string,
	fallback int32,
) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}

	return int32(result), nil
}
