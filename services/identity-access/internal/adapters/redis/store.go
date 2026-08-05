// Package redis stores OIDC transactions and browser sessions in Redis.
package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
)

// Store persists login transactions and browser sessions in Redis.
type Store struct {
	client *goredis.Client
	prefix string
}

// New creates a Redis authentication store.
func New(
	client *goredis.Client,
	prefix string,
) *Store {
	prefix = strings.TrimSuffix(
		strings.TrimSpace(prefix),
		":",
	)

	if prefix == "" {
		prefix = "gereh:iam"
	}

	return &Store{
		client: client,
		prefix: prefix,
	}
}

// PutTransaction stores a one-time login transaction.
func (store *Store) PutTransaction(
	ctx context.Context,
	transaction domain.LoginTransaction,
	ttl time.Duration,
) error {
	value, err := json.Marshal(transaction)
	if err != nil {
		return fmt.Errorf(
			"marshal login transaction: %w",
			err,
		)
	}

	if err := store.client.Set(
		ctx,
		store.transactionKey(transaction.State),
		value,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf(
			"store login transaction: %w",
			err,
		)
	}

	return nil
}

// TakeTransaction atomically consumes a login transaction.
func (store *Store) TakeTransaction(
	ctx context.Context,
	state string,
) (domain.LoginTransaction, error) {
	value, err := store.client.GetDel(
		ctx,
		store.transactionKey(state),
	).Bytes()
	if errors.Is(err, goredis.Nil) {
		return domain.LoginTransaction{},
			domain.ErrAuthenticationFailed
	}

	if err != nil {
		return domain.LoginTransaction{}, fmt.Errorf(
			"consume login transaction: %w",
			err,
		)
	}

	var transaction domain.LoginTransaction

	if err := json.Unmarshal(value, &transaction); err != nil {
		return domain.LoginTransaction{}, fmt.Errorf(
			"decode login transaction: %w",
			err,
		)
	}

	return transaction, nil
}

// PutSession stores an opaque browser session.
func (store *Store) PutSession(
	ctx context.Context,
	session domain.Session,
	ttl time.Duration,
) error {
	value, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf(
			"marshal browser session: %w",
			err,
		)
	}

	if err := store.client.Set(
		ctx,
		store.sessionKey(session.ID),
		value,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf(
			"store browser session: %w",
			err,
		)
	}

	return nil
}

// GetSession retrieves an opaque browser session.
func (store *Store) GetSession(
	ctx context.Context,
	sessionID string,
) (domain.Session, error) {
	value, err := store.client.Get(
		ctx,
		store.sessionKey(sessionID),
	).Bytes()
	if errors.Is(err, goredis.Nil) {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	if err != nil {
		return domain.Session{}, fmt.Errorf(
			"retrieve browser session: %w",
			err,
		)
	}

	var session domain.Session

	if err := json.Unmarshal(value, &session); err != nil {
		return domain.Session{}, fmt.Errorf(
			"decode browser session: %w",
			err,
		)
	}

	return session, nil
}

// DeleteSession revokes a browser session.
func (store *Store) DeleteSession(
	ctx context.Context,
	sessionID string,
) error {
	if err := store.client.Del(
		ctx,
		store.sessionKey(sessionID),
	).Err(); err != nil {
		return fmt.Errorf(
			"delete browser session: %w",
			err,
		)
	}

	return nil
}

func (store *Store) transactionKey(state string) string {
	return store.prefix +
		":oidc:transaction:" +
		hashKey(state)
}

func (store *Store) sessionKey(sessionID string) string {
	return store.prefix +
		":session:" +
		hashKey(sessionID)
}

func hashKey(value string) string {
	digest := sha256.Sum256([]byte(value))

	return hex.EncodeToString(digest[:])
}
