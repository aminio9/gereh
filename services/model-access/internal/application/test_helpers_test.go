package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/aminio9/gereh/services/model-access/internal/ports"
	"github.com/aminio9/gereh/services/model-access/internal/security"
)

// newTestFingerprinter writes a throwaway HMAC key file and returns a
// fingerprinter bound to it.
func newTestFingerprinter(t *testing.T) *security.Fingerprinter {
	t.Helper()

	key := make([]byte, 32)

	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("generate test fingerprint key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "fingerprint.key")

	if err := os.WriteFile(
		path,
		[]byte(base64.StdEncoding.EncodeToString(key)),
		0o400,
	); err != nil {
		t.Fatalf("write test fingerprint key: %v", err)
	}

	fingerprinter, err := security.NewFingerprinterFromFile(path, "test")
	if err != nil {
		t.Fatalf("create test fingerprinter: %v", err)
	}

	return fingerprinter
}

// fakeSecretStore is an in-memory SecretStore for application tests.
type fakeSecretStore struct {
	versions map[string][]byte
	latest   map[string]int64
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{
		versions: make(map[string][]byte),
		latest:   make(map[string]int64),
	}
}

func (store *fakeSecretStore) Reference(
	tenantID string,
	connectionID string,
) string {
	return "vault://model-byok/tenants/" + tenantID + "/connections/" + connectionID
}

func (store *fakeSecretStore) WriteCAS(
	_ context.Context,
	secretRef string,
	credential []byte,
	expectedVersion int64,
) (int64, error) {
	current := store.latest[secretRef]

	if expectedVersion != current {
		return 0, domain.ErrSecretStoreConflict
	}

	version := current + 1

	store.versions[secretRef] = append([]byte(nil), credential...)
	store.latest[secretRef] = version

	return version, nil
}

func (store *fakeSecretStore) ReadVersion(
	_ context.Context,
	secretRef string,
	version int64,
) ([]byte, error) {
	if version != store.latest[secretRef] {
		return nil, domain.ErrSecretNotFound
	}

	value, ok := store.versions[secretRef]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}

	return append([]byte(nil), value...), nil
}

func (store *fakeSecretStore) CurrentVersion(
	_ context.Context,
	secretRef string,
) (int64, error) {
	version, ok := store.latest[secretRef]
	if !ok {
		return 0, domain.ErrSecretNotFound
	}

	return version, nil
}

func (store *fakeSecretStore) DestroyVersion(
	_ context.Context,
	_ string,
	_ int64,
) error {
	return nil
}

func (store *fakeSecretStore) Purge(
	_ context.Context,
	secretRef string,
) error {
	delete(store.versions, secretRef)
	delete(store.latest, secretRef)

	return nil
}

// stubVerifier is a configurable CredentialVerifier for application tests.
type stubVerifier struct {
	err    error
	status int
}

func (verifier stubVerifier) Verify(
	_ context.Context,
	_ string,
	_ []byte,
) (ports.ProviderVerification, error) {
	return ports.ProviderVerification{
		HTTPStatus:           verifier.status,
		DiscoveredModelCount: 1,
	}, verifier.err
}
