// Package oidc integrates identity-access with an OpenID Connect provider.
package oidc

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
)

// Client is an OpenID Connect relying-party client.
type Client struct {
	config       Config
	httpClient   *http.Client
	provider     *gooidc.Provider
	verifier     *gooidc.IDTokenVerifier
	oauth2Config oauth2.Config
}

// New creates a discovered and validated OIDC client.
func New(
	ctx context.Context,
	config Config,
	transport http.RoundTripper,
) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if transport == nil {
		transport = http.DefaultTransport
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.HTTPTimeout,
	}

	providerContext := gooidc.ClientContext(
		ctx,
		httpClient,
	)

	provider, err := gooidc.NewProvider(
		providerContext,
		config.IssuerURL,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"discover OpenID Provider: %w",
			err,
		)
	}

	var metadata struct {
		CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
		ResponseTypesSupported        []string `json:"response_types_supported"`
	}

	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf(
			"decode OpenID Provider metadata: %w",
			err,
		)
	}

	if !slices.Contains(
		metadata.ResponseTypesSupported,
		"code",
	) {
		return nil, fmt.Errorf(
			"OpenID Provider does not advertise authorization-code support",
		)
	}

	if config.RequirePKCEAdvertised &&
		!slices.Contains(
			metadata.CodeChallengeMethodsSupported,
			"S256",
		) {
		return nil, fmt.Errorf(
			"OpenID Provider does not advertise PKCE S256 support",
		)
	}

	verifierConfig := &gooidc.Config{
		ClientID: config.ClientID,
	}

	if len(config.SupportedSigningAlgs) > 0 {
		verifierConfig.SupportedSigningAlgs = append(
			[]string(nil),
			config.SupportedSigningAlgs...,
		)
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		provider:   provider,
		verifier: provider.Verifier(
			verifierConfig,
		),
		oauth2Config: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			RedirectURL:  config.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       append([]string(nil), config.Scopes...),
		},
	}, nil
}

// AuthorizationURL returns a provider authorization URL.
func (client *Client) AuthorizationURL(
	state string,
	nonce string,
	pkceVerifier string,
) string {
	return client.oauth2Config.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(pkceVerifier),
		gooidc.Nonce(nonce),
	)
}

// Exchange exchanges an authorization code and verifies the resulting identity.
func (client *Client) Exchange(
	ctx context.Context,
	code string,
	pkceVerifier string,
	expectedNonce string,
) (domain.ExternalIdentity, error) {
	tokenContext := gooidc.ClientContext(
		ctx,
		client.httpClient,
	)

	oauthToken, err := client.oauth2Config.Exchange(
		tokenContext,
		code,
		oauth2.VerifierOption(pkceVerifier),
	)
	if err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: exchange authorization code",
			domain.ErrAuthenticationFailed,
		)
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: token response has no ID Token",
			domain.ErrAuthenticationFailed,
		)
	}

	idToken, err := client.verifier.Verify(
		tokenContext,
		rawIDToken,
	)
	if err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: verify ID Token",
			domain.ErrAuthenticationFailed,
		)
	}

	if subtle.ConstantTimeCompare(
		[]byte(idToken.Nonce),
		[]byte(expectedNonce),
	) != 1 {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: nonce mismatch",
			domain.ErrAuthenticationFailed,
		)
	}

	if idToken.AccessTokenHash != "" {
		if err := idToken.VerifyAccessToken(
			oauthToken.AccessToken,
		); err != nil {
			return domain.ExternalIdentity{}, fmt.Errorf(
				"%w: access-token hash mismatch",
				domain.ErrAuthenticationFailed,
			)
		}
	}

	var claims struct {
		Subject           string `json:"sub"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		Picture           string `json:"picture"`
		PreferredUsername string `json:"preferred_username"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: decode ID Token claims",
			domain.ErrAuthenticationFailed,
		)
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: subject claim is missing",
			domain.ErrAuthenticationFailed,
		)
	}

	rawClaims, err := rawTokenClaims(rawIDToken)
	if err != nil {
		return domain.ExternalIdentity{}, fmt.Errorf(
			"%w: decode raw ID Token claims",
			domain.ErrAuthenticationFailed,
		)
	}

	identity := domain.ExternalIdentity{
		Issuer:        idToken.Issuer,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName: firstNonEmpty(
			claims.Name,
			claims.PreferredUsername,
		),
		PictureURL: claims.Picture,
		RawClaims:  rawClaims,
	}

	if client.config.FetchUserInfo {
		if err := client.mergeUserInfo(
			tokenContext,
			oauthToken,
			&identity,
		); err != nil {
			return domain.ExternalIdentity{}, err
		}
	}

	if client.config.RequireVerifiedEmail {
		if strings.TrimSpace(identity.Email) == "" ||
			!identity.EmailVerified {
			return domain.ExternalIdentity{}, fmt.Errorf(
				"%w: a verified email is required",
				domain.ErrAuthenticationFailed,
			)
		}
	}

	return identity, nil
}

func (client *Client) mergeUserInfo(
	ctx context.Context,
	token *oauth2.Token,
	identity *domain.ExternalIdentity,
) error {
	userInfo, err := client.provider.UserInfo(
		ctx,
		oauth2.StaticTokenSource(token),
	)
	if err != nil {
		return fmt.Errorf(
			"%w: retrieve UserInfo",
			domain.ErrAuthenticationFailed,
		)
	}

	if userInfo.Subject != identity.Subject {
		return fmt.Errorf(
			"%w: UserInfo subject mismatch",
			domain.ErrAuthenticationFailed,
		)
	}

	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		Picture           string `json:"picture"`
		PreferredUsername string `json:"preferred_username"`
	}

	if err := userInfo.Claims(&claims); err != nil {
		return fmt.Errorf(
			"%w: decode UserInfo claims",
			domain.ErrAuthenticationFailed,
		)
	}

	if identity.Email == "" {
		identity.Email = claims.Email
		identity.EmailVerified = claims.EmailVerified
	}

	if identity.DisplayName == "" {
		identity.DisplayName = firstNonEmpty(
			claims.Name,
			claims.PreferredUsername,
		)
	}

	if identity.PictureURL == "" {
		identity.PictureURL = claims.Picture
	}

	return nil
}

func rawTokenClaims(rawToken string) (json.RawMessage, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("ID Token must contain three segments")
	}

	payload, err := oauth2JWTDecode(parts[1])
	if err != nil {
		return nil, err
	}

	if !json.Valid(payload) {
		return nil, fmt.Errorf("ID Token payload is not valid JSON")
	}

	return json.RawMessage(payload), nil
}

func oauth2JWTDecode(value string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf(
			"decode ID Token payload: %w",
			err,
		)
	}

	return payload, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}

	return ""
}
