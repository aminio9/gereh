// Package security contains Model Access cryptographic helpers.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const fingerprintKeyBytes = 32

// Fingerprint is a keyed HMAC of a credential. Full is the 256-bit digest;
// Display is a safe truncated representation for public use.
type Fingerprint struct {
	Full    []byte
	Display string
	KeyID   string
}

// Fingerprinter computes keyed HMAC fingerprints.
type Fingerprinter struct {
	key   []byte
	keyID string
}

// NewFingerprinterFromFile loads a base64-encoded 32-byte HMAC key from a file.
func NewFingerprinterFromFile(
	path string,
	keyID string,
) (*Fingerprinter, error) {
	path = strings.TrimSpace(path)
	keyID = strings.TrimSpace(keyID)

	if path == "" {
		return nil, fmt.Errorf(
			"fingerprint key file is required",
		)
	}

	if keyID == "" ||
		len(keyID) > 32 {
		return nil, fmt.Errorf(
			"fingerprint key ID is invalid",
		)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read fingerprint key: %w",
			err,
		)
	}

	encoded :=
		strings.TrimSpace(
			string(raw),
		)

	key, err :=
		base64.StdEncoding.
			DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf(
			"decode fingerprint key: %w",
			err,
		)
	}

	if len(key) != fingerprintKeyBytes {
		return nil, fmt.Errorf(
			"fingerprint key must decode to %d bytes",
			fingerprintKeyBytes,
		)
	}

	return &Fingerprinter{
		key:   key,
		keyID: keyID,
	}, nil
}

// Sum computes the keyed fingerprint of a credential.
func (fingerprinter *Fingerprinter) Sum(
	credential []byte,
) Fingerprint {
	mac :=
		hmac.New(
			sha256.New,
			fingerprinter.key,
		)

	_, _ = mac.Write(credential)

	full := mac.Sum(nil)

	// 128 bits of keyed fingerprint are sufficient for a UI-safe
	// identity marker; PostgreSQL retains the full 256-bit HMAC.
	display :=
		fmt.Sprintf(
			"fp-%s:%s",
			fingerprinter.keyID,
			hex.EncodeToString(
				full[:16],
			),
		)

	return Fingerprint{
		Full:    full,
		Display: display,
		KeyID:   fingerprinter.keyID,
	}
}

// Equal compares two digests in constant time.
func Equal(
	left []byte,
	right []byte,
) bool {
	return hmac.Equal(
		left,
		right,
	)
}
