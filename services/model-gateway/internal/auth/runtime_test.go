package auth_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/model-gateway/internal/auth"
	"github.com/aminio9/gereh/services/model-gateway/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestVerifier(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	issuer := "gereh-runtime"
	verifier := auth.NewVerifier(pub, issuer)

	tenantID := uuid.NewString()
	agentID := uuid.NewString()
	executionID := uuid.NewString()

	t.Run("valid token within 15min TTL", func(t *testing.T) {
		now := time.Now().UTC()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss":          issuer,
			"tenant_id":    tenantID,
			"agent_id":     agentID,
			"execution_id": executionID,
			"workflow_id":  "wf-123",
			"run_id":       "run-456",
			"step_id":      "step-789",
			"iat":          now.Unix(),
			"exp":          now.Add(10 * time.Minute).Unix(),
		})

		signed, err := token.SignedString(priv)
		require.NoError(t, err)

		claims, err := verifier.Verify("Bearer " + signed)
		require.NoError(t, err)
		require.Equal(t, tenantID, claims.TenantID)
		require.Equal(t, agentID, claims.AgentID)
		require.Equal(t, executionID, claims.ExecutionID)
		require.Equal(t, "wf-123", claims.WorkflowID)
		require.Equal(t, "run-456", claims.RunID)
		require.Equal(t, "step-789", claims.StepID)
	})

	t.Run("token TTL exceeding 15 minutes rejected", func(t *testing.T) {
		now := time.Now().UTC()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss":          issuer,
			"tenant_id":    tenantID,
			"agent_id":     agentID,
			"execution_id": executionID,
			"workflow_id":  "wf-123",
			"run_id":       "run-456",
			"step_id":      "step-789",
			"iat":          now.Unix(),
			"exp":          now.Add(1 * time.Hour).Unix(),
		})

		signed, err := token.SignedString(priv)
		require.NoError(t, err)

		_, err = verifier.Verify("Bearer " + signed)
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrInvalidToken)
	})

	t.Run("expired token rejected", func(t *testing.T) {
		past := time.Now().UTC().Add(-30 * time.Minute)
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss":          issuer,
			"tenant_id":    tenantID,
			"agent_id":     agentID,
			"execution_id": executionID,
			"workflow_id":  "wf-123",
			"run_id":       "run-456",
			"step_id":      "step-789",
			"iat":          past.Unix(),
			"exp":          past.Add(5 * time.Minute).Unix(),
		})

		signed, err := token.SignedString(priv)
		require.NoError(t, err)

		_, err = verifier.Verify("Bearer " + signed)
		require.Error(t, err)
		require.ErrorIs(t, err, domain.ErrTokenExpired)
	})

	t.Run("invalid issuer rejected", func(t *testing.T) {
		now := time.Now().UTC()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"iss":          "untrusted-issuer",
			"tenant_id":    tenantID,
			"agent_id":     agentID,
			"execution_id": executionID,
			"workflow_id":  "wf-123",
			"run_id":       "run-456",
			"step_id":      "step-789",
			"iat":          now.Unix(),
			"exp":          now.Add(10 * time.Minute).Unix(),
		})

		signed, err := token.SignedString(priv)
		require.NoError(t, err)

		_, err = verifier.Verify("Bearer " + signed)
		require.Error(t, err)
	})
}
