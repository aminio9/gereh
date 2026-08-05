package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
	"github.com/aminio9/gereh/services/identity-access/internal/ports"
)

type fakeProvider struct {
	authorizationURL string
	exchangeIdentity domain.ExternalIdentity
	exchangeError    error
}

func (fake *fakeProvider) AuthorizationURL(
	_ string,
	_ string,
	_ string,
) string {
	return fake.authorizationURL
}

func (fake *fakeProvider) Exchange(
	_ context.Context,
	_ string,
	_ string,
	_ string,
) (domain.ExternalIdentity, error) {
	if fake.exchangeError != nil {
		return domain.ExternalIdentity{}, fake.exchangeError
	}

	return fake.exchangeIdentity, nil
}

type fakeTransactionStore struct {
	mu           sync.Mutex
	transactions map[string]domain.LoginTransaction
}

func newFakeTransactionStore() *fakeTransactionStore {
	return &fakeTransactionStore{
		transactions: make(map[string]domain.LoginTransaction),
	}
}

func (store *fakeTransactionStore) PutTransaction(
	_ context.Context,
	transaction domain.LoginTransaction,
	_ time.Duration,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.transactions[transaction.State] = transaction

	return nil
}

func (store *fakeTransactionStore) TakeTransaction(
	_ context.Context,
	state string,
) (domain.LoginTransaction, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	transaction, ok := store.transactions[state]
	if !ok {
		return domain.LoginTransaction{}, domain.ErrSessionNotFound
	}

	delete(store.transactions, state)

	return transaction, nil
}

type fakeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{
		sessions: make(map[string]domain.Session),
	}
}

func (store *fakeSessionStore) PutSession(
	_ context.Context,
	session domain.Session,
	_ time.Duration,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.sessions[session.ID] = session

	return nil
}

func (store *fakeSessionStore) GetSession(
	_ context.Context,
	sessionID string,
) (domain.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	session, ok := store.sessions[sessionID]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	return session, nil
}

func (store *fakeSessionStore) DeleteSession(
	_ context.Context,
	sessionID string,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	delete(store.sessions, sessionID)

	return nil
}

type fakeUserRepository struct {
	user domain.User
}

func (repo *fakeUserRepository) ResolveUser(
	_ context.Context,
	_ domain.ExternalIdentity,
) (domain.User, error) {
	return repo.user, nil
}

func newService(
	provider ports.IdentityProvider,
	transactions ports.TransactionStore,
	sessions ports.SessionStore,
	users ports.UserRepository,
	now func() time.Time,
) *Service {
	service, err := New(
		Config{
			TransactionTTL: 5 * time.Minute,
			SessionTTL:     24 * time.Hour,
		},
		provider,
		transactions,
		sessions,
		users,
	)
	if err != nil {
		panic(err)
	}

	if now != nil {
		service.now = now
	}

	return service
}

func TestLoginLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	provider := &fakeProvider{
		authorizationURL: "https://issuer.example.com/auth",
		exchangeIdentity: domain.ExternalIdentity{
			Issuer:        "https://issuer.example.com",
			Subject:       "sub-123",
			Email:         "alice@example.com",
			EmailVerified: true,
			DisplayName:   "Alice",
		},
	}
	transactions := newFakeTransactionStore()
	sessions := newFakeSessionStore()
	users := &fakeUserRepository{
		user: domain.User{
			ID:            "user-1",
			Issuer:        "https://issuer.example.com",
			Subject:       "sub-123",
			Email:         "alice@example.com",
			EmailVerified: true,
			DisplayName:   "Alice",
		},
	}

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	service := newService(provider, transactions, sessions, users, func() time.Time {
		return now
	})

	begin, err := service.BeginLogin(ctx, "/dashboard")
	if err != nil {
		t.Fatalf("BeginLogin returned error: %v", err)
	}

	if begin.AuthorizationURL != provider.authorizationURL {
		t.Errorf(
			"authorization URL = %q, want %q",
			begin.AuthorizationURL,
			provider.authorizationURL,
		)
	}

	if begin.BrowserBinding == "" {
		t.Error("BeginLogin returned empty browser binding")
	}

	if !begin.ExpiresAt.After(now) {
		t.Error("BeginLogin returned an already-expired transaction")
	}

	if len(transactions.transactions) != 1 {
		t.Fatalf(
			"expected 1 stored transaction, got %d",
			len(transactions.transactions),
		)
	}

	var state string
	for key, transaction := range transactions.transactions {
		state = key
		if transaction.ReturnTo != "/dashboard" {
			t.Errorf(
				"transaction return_to = %q, want %q",
				transaction.ReturnTo,
				"/dashboard",
			)
		}
	}

	complete, err := service.CompleteLogin(
		ctx,
		state,
		"auth-code-123",
		begin.BrowserBinding,
	)
	if err != nil {
		t.Fatalf("CompleteLogin returned error: %v", err)
	}

	if complete.SessionID == "" || complete.CSRFToken == "" {
		t.Error("CompleteLogin returned empty session ID or CSRF token")
	}

	if complete.ReturnTo != "/dashboard" {
		t.Errorf("ReturnTo = %q, want %q", complete.ReturnTo, "/dashboard")
	}

	if complete.Session.ExpiresAt.Before(now) {
		t.Error("created session is already expired")
	}

	if _, err := service.GetSession(ctx, complete.SessionID); err != nil {
		t.Errorf("GetSession returned error: %v", err)
	}

	if err := service.DeleteSession(
		ctx,
		complete.SessionID,
		complete.CSRFToken,
	); err != nil {
		t.Errorf("DeleteSession returned error: %v", err)
	}

	if _, err := service.GetSession(ctx, complete.SessionID); !errors.Is(
		err,
		domain.ErrSessionNotFound,
	) {
		t.Errorf("GetSession after DeleteSession = %v, want session not found", err)
	}
}

func TestBeginLoginRejectsOpenRedirect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	provider := &fakeProvider{authorizationURL: "https://issuer.example.com/auth"}
	transactions := newFakeTransactionStore()
	sessions := newFakeSessionStore()
	users := &fakeUserRepository{}

	service := newService(provider, transactions, sessions, users, nil)

	unsafe := []string{
		"https://evil.example.com",
		"//evil.example.com",
		"/path\\..\\..\\evil",
		"/evil\r\nSet-Cookie: x=y",
	}

	for _, target := range unsafe {
		if _, err := service.BeginLogin(ctx, target); !errors.Is(
			err,
			domain.ErrInvalidRequest,
		) {
			t.Errorf(
				"BeginLogin(%q) error = %v, want invalid request",
				target,
				err,
			)
		}
	}

	if len(transactions.transactions) != 0 {
		t.Error(
			"unsafe return paths still persisted a login transaction",
		)
	}
}
