// Package security provides a security.
package security

import (
	"testing"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"github.com/stretchr/testify/require"
)

const testSigningKey = "k2cZF3qx4Ne01CBaUmvy7zBE8XB4v+WqZaIM5dBXQQw="

func testDecision() *policyv1.PolicyDecision {
	return &policyv1.PolicyDecision{
		DecisionId:          "decision-1",
		EvaluationRequestId: "request-1",
		TenantId:            "tenant-1",
		Effect:              policyv1.PolicyEffect_POLICY_EFFECT_ALLOW,
		Reason:              "allow rotation",
	}
}

func TestNewSigner(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner("key-1", testSigningKey)
	require.NoError(t, err)
	require.Equal(t, "key-1", signer.KeyID())
}

func TestNewSignerRejectsShortKey(t *testing.T) {
	t.Parallel()

	_, err := NewSigner("key-1", "c2hvcnQ=")
	require.Error(t, err)
}

func TestNewSignerRejectsEmptyKeyID(t *testing.T) {
	t.Parallel()

	_, err := NewSigner("", testSigningKey)
	require.Error(t, err)
}

func TestSignDeterministic(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner("key-1", testSigningKey)
	require.NoError(t, err)

	first, err := signer.Sign(testDecision())
	require.NoError(t, err)

	second, err := signer.Sign(testDecision())
	require.NoError(t, err)

	require.Equal(t, first, second)
}

func TestVerify(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner("key-1", testSigningKey)
	require.NoError(t, err)

	decision := testDecision()

	signature, err := signer.Sign(decision)
	require.NoError(t, err)

	decision.Signature = signature

	require.True(t, signer.Verify(decision))
}

func TestVerifyRejectsTamperedDecision(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner("key-1", testSigningKey)
	require.NoError(t, err)

	decision := testDecision()

	signature, err := signer.Sign(decision)
	require.NoError(t, err)

	decision.Signature = signature
	decision.Effect = policyv1.PolicyEffect_POLICY_EFFECT_DENY

	require.False(t, signer.Verify(decision))
}

func TestVerifyFailsWithDifferentKey(t *testing.T) {
	t.Parallel()

	first, err := NewSigner("key-1", testSigningKey)
	require.NoError(t, err)

	second, err := NewSigner("key-1", "eBox2Y2rVkpjNETWnJMZ2HdMsvSdtSoaQVLcaP7Vf6k=")
	require.NoError(t, err)

	decision := testDecision()

	signature, err := first.Sign(decision)
	require.NoError(t, err)

	decision.Signature = signature

	require.False(t, second.Verify(decision))
}
