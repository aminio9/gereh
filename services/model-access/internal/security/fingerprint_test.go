package security

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestFingerprinter(t *testing.T) *Fingerprinter {
	t.Helper()

	key := make([]byte, 32)

	_, err := rand.Read(key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "fingerprint.key")

	require.NoError(t, os.WriteFile(
		path,
		[]byte(base64.StdEncoding.EncodeToString(key)),
		0o400,
	))

	fingerprinter, err := NewFingerprinterFromFile(path, "test")
	require.NoError(t, err)

	return fingerprinter
}

func TestFingerprintIsStableAndNonSecret(t *testing.T) {
	t.Parallel()

	fingerprinter := newTestFingerprinter(t)

	secret := []byte("sk-example-super-secret-value")

	first := fingerprinter.Sum(secret)
	second := fingerprinter.Sum(secret)

	require.Equal(t, first.Display, second.Display)
	require.Len(t, first.Full, 32)
	require.NotContains(t, first.Display, "sk-example")
	require.NotContains(t, first.Display, "super-secret")
	require.True(t, Equal(first.Full, second.Full))
}

func TestFingerprinterRejectsShortKey(t *testing.T) {
	t.Parallel()

	key := make([]byte, 16)

	_, err := rand.Read(key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "fingerprint.key")

	require.NoError(t, os.WriteFile(
		path,
		[]byte(base64.StdEncoding.EncodeToString(key)),
		0o400,
	))

	_, err = NewFingerprinterFromFile(path, "test")
	require.Error(t, err)
}

func TestFingerprintDiffersAcrossSecrets(t *testing.T) {
	t.Parallel()

	fingerprinter := newTestFingerprinter(t)

	first := fingerprinter.Sum([]byte("credential-a-value"))
	second := fingerprinter.Sum([]byte("credential-b-value"))

	require.NotEqual(t, first.Display, second.Display)
	require.False(t, Equal(first.Full, second.Full))
}
