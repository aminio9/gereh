package ports

import (
	"context"
	"time"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
)

// IdentityProvider performs protocol-specific OpenID Connect operations.
type IdentityProvider interface {
	AuthorizationURL(
		state string,
		nonce string,
		pkceVerifier string,
	) string

	Exchange(
		ctx context.Context,
		code string,
		pkceVerifier string,
		expectedNonce string,
	) (domain.ExternalIdentity, error)
}

// TransactionStore persists short-lived single-use login transactions.
type TransactionStore interface {
	PutTransaction(
		ctx context.Context,
		transaction domain.LoginTransaction,
		ttl time.Duration,
	) error

	TakeTransaction(
		ctx context.Context,
		state string,
	) (domain.LoginTransaction, error)
}

// SessionStore persists opaque browser sessions.
type SessionStore interface {
	PutSession(
		ctx context.Context,
		session domain.Session,
		ttl time.Duration,
	) error

	GetSession(
		ctx context.Context,
		sessionID string,
	) (domain.Session, error)

	DeleteSession(
		ctx context.Context,
		sessionID string,
	) error
}

// UserRepository maps verified external identities to internal users.
type UserRepository interface {
	ResolveUser(
		ctx context.Context,
		identity domain.ExternalIdentity,
	) (domain.User, error)
}
