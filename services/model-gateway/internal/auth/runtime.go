// Package auth provides runtime execution cell JWT token validation.
package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const maxRuntimeTokenTTL = 15 * time.Minute

// Verifier validates runtime Ed25519 JWT tokens issued to runtime execution cells.
type Verifier struct {
	publicKey ed25519.PublicKey
	issuer    string
}

// customClaims represents the structured claims in a runtime execution token.
type customClaims struct {
	TenantID    string `json:"tenant_id"`
	AgentID     string `json:"agent_id"`
	ExecutionID string `json:"execution_id"`
	WorkflowID  string `json:"workflow_id"`
	RunID       string `json:"run_id"`
	StepID      string `json:"step_id"`
	jwt.RegisteredClaims
}

// NewVerifier creates a new runtime token verifier with the given Ed25519 public key.
func NewVerifier(publicKey ed25519.PublicKey, issuer string) *Verifier {
	return &Verifier{
		publicKey: publicKey,
		issuer:    issuer,
	}
}

// NewVerifierFromFile loads an Ed25519 public key from a PEM file.
func NewVerifierFromFile(keyPath string, issuer string) (*Verifier, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read runtime public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		// Try raw 32-byte key
		if len(data) == ed25519.PublicKeySize {
			return NewVerifier(ed25519.PublicKey(data), issuer), nil
		}
		return nil, errors.New("failed to parse PEM block for runtime public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}

	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("key is not an Ed25519 public key")
	}

	return NewVerifier(edPub, issuer), nil
}

// Verify extracts and strictly validates the runtime claims from a Bearer token string.
func (v *Verifier) Verify(rawToken string) (domain.RuntimeClaims, error) {
	tokenStr := strings.TrimPrefix(rawToken, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return domain.RuntimeClaims{}, domain.ErrUnauthorized
	}

	var claims customClaims
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, fmt.Errorf("unexpected signing algorithm: %v", token.Header["alg"])
			}
			return v.publicKey, nil
		},
		jwt.WithLeeway(5*time.Second),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return domain.RuntimeClaims{}, domain.ErrTokenExpired
		}
		return domain.RuntimeClaims{}, fmt.Errorf("%w: %w", domain.ErrInvalidToken, err)
	}

	if !token.Valid {
		return domain.RuntimeClaims{}, domain.ErrInvalidToken
	}

	if v.issuer != "" && claims.Issuer != v.issuer {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: issuer mismatch", domain.ErrInvalidToken)
	}

	// Validate required UUID format fields
	if _, err := uuid.Parse(claims.TenantID); err != nil {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: invalid tenant_id", domain.ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.AgentID); err != nil {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: invalid agent_id", domain.ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.ExecutionID); err != nil {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: invalid execution_id", domain.ErrInvalidToken)
	}
	if claims.WorkflowID == "" || claims.RunID == "" || claims.StepID == "" {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: missing workflow/run/step context", domain.ErrInvalidToken)
	}

	issuedAt := claims.IssuedAt.Time
	expiresAt := claims.ExpiresAt.Time

	// Enforce maximum 15 min TTL constraint
	if expiresAt.Sub(issuedAt) > maxRuntimeTokenTTL+10*time.Second {
		return domain.RuntimeClaims{}, fmt.Errorf("%w: token TTL exceeds 15 minutes", domain.ErrInvalidToken)
	}

	return domain.RuntimeClaims{
		TenantID:    claims.TenantID,
		AgentID:     claims.AgentID,
		ExecutionID: claims.ExecutionID,
		WorkflowID:  claims.WorkflowID,
		RunID:       claims.RunID,
		StepID:      claims.StepID,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
	}, nil
}
