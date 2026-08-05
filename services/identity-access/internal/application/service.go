package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
	"github.com/aminio9/gereh/services/identity-access/internal/ports"
)

// Config defines authentication transaction and session policy.
type Config struct {
	TransactionTTL time.Duration
	SessionTTL     time.Duration
}

// BeginLoginResult contains the public redirect and browser binding.
type BeginLoginResult struct {
	AuthorizationURL string
	BrowserBinding   string
	ExpiresAt        time.Time
}

// CompleteLoginResult contains a newly-created session.
type CompleteLoginResult struct {
	SessionID string
	CSRFToken string
	Session   domain.Session
	ReturnTo  string
}

// Service implements OIDC login and session lifecycle.
type Service struct {
	config       Config
	provider     ports.IdentityProvider
	transactions ports.TransactionStore
	sessions     ports.SessionStore
	users        ports.UserRepository
	now          func() time.Time
}

// New creates an authentication service.
func New(
	config Config,
	provider ports.IdentityProvider,
	transactions ports.TransactionStore,
	sessions ports.SessionStore,
	users ports.UserRepository,
) (*Service, error) {
	if config.TransactionTTL <= 0 {
		return nil, fmt.Errorf(
			"transaction TTL must be greater than zero",
		)
	}

	if config.SessionTTL <= 0 {
		return nil, fmt.Errorf(
			"session TTL must be greater than zero",
		)
	}

	return &Service{
		config:       config,
		provider:     provider,
		transactions: transactions,
		sessions:     sessions,
		users:        users,
		now:          time.Now,
	}, nil
}

// BeginLogin starts a browser-bound OIDC authorization-code flow.
func (service *Service) BeginLogin(
	ctx context.Context,
	returnTo string,
) (BeginLoginResult, error) {
	returnTo, err := safeReturnPath(returnTo)
	if err != nil {
		return BeginLoginResult{}, err
	}

	state, err := randomToken(32)
	if err != nil {
		return BeginLoginResult{}, err
	}

	browserBinding, err := randomToken(32)
	if err != nil {
		return BeginLoginResult{}, err
	}

	nonce, err := randomToken(32)
	if err != nil {
		return BeginLoginResult{}, err
	}

	pkceVerifier := oauth2.GenerateVerifier()
	now := service.now().UTC()
	expiresAt := now.Add(service.config.TransactionTTL)

	transaction := domain.LoginTransaction{
		State:              state,
		BrowserBindingHash: hashSecret(browserBinding),
		Nonce:              nonce,
		PKCEVerifier:       pkceVerifier,
		ReturnTo:           returnTo,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
	}

	if err := service.transactions.PutTransaction(
		ctx,
		transaction,
		service.config.TransactionTTL,
	); err != nil {
		return BeginLoginResult{}, fmt.Errorf(
			"persist login transaction: %w",
			err,
		)
	}

	return BeginLoginResult{
		AuthorizationURL: service.provider.AuthorizationURL(
			state,
			nonce,
			pkceVerifier,
		),
		BrowserBinding: browserBinding,
		ExpiresAt:      expiresAt,
	}, nil
}

// CompleteLogin consumes a transaction and creates a browser session.
func (service *Service) CompleteLogin(
	ctx context.Context,
	state string,
	code string,
	browserBinding string,
) (CompleteLoginResult, error) {
	if strings.TrimSpace(state) == "" ||
		strings.TrimSpace(code) == "" ||
		strings.TrimSpace(browserBinding) == "" {
		return CompleteLoginResult{},
			domain.ErrInvalidRequest
	}

	transaction, err := service.transactions.TakeTransaction(
		ctx,
		state,
	)
	if err != nil {
		return CompleteLoginResult{}, err
	}

	now := service.now().UTC()

	if !transaction.ExpiresAt.After(now) {
		return CompleteLoginResult{},
			domain.ErrAuthenticationFailed
	}

	if !constantTimeEqual(
		transaction.BrowserBindingHash,
		hashSecret(browserBinding),
	) {
		return CompleteLoginResult{},
			domain.ErrAuthenticationFailed
	}

	identity, err := service.provider.Exchange(
		ctx,
		code,
		transaction.PKCEVerifier,
		transaction.Nonce,
	)
	if err != nil {
		return CompleteLoginResult{}, err
	}

	user, err := service.users.ResolveUser(
		ctx,
		identity,
	)
	if err != nil {
		return CompleteLoginResult{}, fmt.Errorf(
			"resolve internal user: %w",
			err,
		)
	}

	sessionID, err := randomToken(32)
	if err != nil {
		return CompleteLoginResult{}, err
	}

	csrfToken, err := randomToken(32)
	if err != nil {
		return CompleteLoginResult{}, err
	}

	session := domain.Session{
		ID:        sessionID,
		CSRFHash:  hashSecret(csrfToken),
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(service.config.SessionTTL),
	}

	if err := service.sessions.PutSession(
		ctx,
		session,
		service.config.SessionTTL,
	); err != nil {
		return CompleteLoginResult{}, fmt.Errorf(
			"persist browser session: %w",
			err,
		)
	}

	return CompleteLoginResult{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		Session:   session,
		ReturnTo:  transaction.ReturnTo,
	}, nil
}

// GetSession validates and returns an active browser session.
func (service *Service) GetSession(
	ctx context.Context,
	sessionID string,
) (domain.Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	session, err := service.sessions.GetSession(
		ctx,
		sessionID,
	)
	if err != nil {
		return domain.Session{}, err
	}

	if !session.ExpiresAt.After(service.now().UTC()) {
		_ = service.sessions.DeleteSession(
			ctx,
			sessionID,
		)

		return domain.Session{}, domain.ErrSessionNotFound
	}

	return session, nil
}

// DeleteSession validates CSRF and revokes a browser session.
func (service *Service) DeleteSession(
	ctx context.Context,
	sessionID string,
	csrfToken string,
) error {
	session, err := service.GetSession(
		ctx,
		sessionID,
	)
	if err != nil {
		return err
	}

	if !constantTimeEqual(
		session.CSRFHash,
		hashSecret(csrfToken),
	) {
		return domain.ErrCSRFValidation
	}

	return service.sessions.DeleteSession(
		ctx,
		sessionID,
	)
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)

	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf(
			"generate secure random token: %w",
			err,
		)
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSecret(value string) string {
	digest := sha256.Sum256([]byte(value))

	return hex.EncodeToString(digest[:])
}

func constantTimeEqual(left string, right string) bool {
	if len(left) != len(right) {
		return false
	}

	return subtle.ConstantTimeCompare(
		[]byte(left),
		[]byte(right),
	) == 1
}

func safeReturnPath(value string) (string, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return "/", nil
	}

	if !strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "//") ||
		strings.Contains(value, "\\") ||
		strings.ContainsAny(value, "\r\n") {
		return "", domain.ErrInvalidRequest
	}

	return value, nil
}
