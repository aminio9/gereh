// Package security signs policy decisions.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	"google.golang.org/protobuf/proto"
)

// Signer signs policy decisions with an HMAC-SHA256 key.
type Signer struct {
	keyID string
	key   []byte
}

// NewSigner creates the decision signer.
func NewSigner(keyID string, base64Key string) (*Signer, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf(
			"decode policy signing key: %w",
			err,
		)
	}

	if len(key) < 32 {
		return nil, fmt.Errorf(
			"policy signing key must contain at least 32 bytes",
		)
	}

	if keyID == "" {
		return nil, fmt.Errorf(
			"policy signing key ID is required",
		)
	}

	return &Signer{
		keyID: keyID,
		key:   key,
	}, nil
}

// KeyID returns the signing key identifier.
func (signer *Signer) KeyID() string {
	return signer.keyID
}

// Sign produces the deterministic signature of a decision.
func (signer *Signer) Sign(
	decision *policyv1.PolicyDecision,
) ([]byte, error) {
	unsigned := proto.Clone(
		decision,
	).(*policyv1.PolicyDecision)

	unsigned.Signature = nil

	encoded, err := proto.MarshalOptions{
		Deterministic: true,
	}.Marshal(unsigned)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal policy decision for signing: %w",
			err,
		)
	}

	mac := hmac.New(sha256.New, signer.key)
	_, _ = mac.Write(encoded)

	return mac.Sum(nil), nil
}

// Verify reports whether the decision signature is valid.
func (signer *Signer) Verify(
	decision *policyv1.PolicyDecision,
) bool {
	expected, err := signer.Sign(decision)
	if err != nil {
		return false
	}

	return hmac.Equal(expected, decision.GetSignature())
}
