// Package postgres persists users and external identities in PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aminio9/gereh/services/identity-access/internal/domain"
)

// Repository persists internal and external identities.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates an identity repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// ResolveUser maps an issuer and subject to exactly one internal user.
func (repository *Repository) ResolveUser(
	ctx context.Context,
	identity domain.ExternalIdentity,
) (domain.User, error) {
	transaction, err := repository.pool.BeginTx(
		ctx,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.User{}, fmt.Errorf(
			"begin identity transaction: %w",
			err,
		)
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	lockValue := identity.Issuer + "\x00" + identity.Subject

	if _, err := transaction.Exec(
		ctx,
		`
			SELECT pg_advisory_xact_lock(
				hashtextextended($1, 0)
			)
		`,
		lockValue,
	); err != nil {
		return domain.User{}, fmt.Errorf(
			"lock external identity: %w",
			err,
		)
	}

	user, err := findUser(
		ctx,
		transaction,
		identity.Issuer,
		identity.Subject,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, err
	}

	if errors.Is(err, pgx.ErrNoRows) {
		user, err = createUser(
			ctx,
			transaction,
			identity,
		)
		if err != nil {
			return domain.User{}, err
		}
	} else {
		user, err = updateUser(
			ctx,
			transaction,
			user.ID,
			identity,
		)
		if err != nil {
			return domain.User{}, err
		}
	}

	if err := upsertExternalIdentity(
		ctx,
		transaction,
		user.ID,
		identity,
	); err != nil {
		return domain.User{}, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf(
			"commit identity transaction: %w",
			err,
		)
	}

	user.Issuer = identity.Issuer
	user.Subject = identity.Subject
	user.EmailVerified = identity.EmailVerified

	return user, nil
}

func findUser(
	ctx context.Context,
	transaction pgx.Tx,
	issuer string,
	subject string,
) (domain.User, error) {
	var user domain.User

	err := transaction.QueryRow(
		ctx,
		`
			SELECT
				u.user_id::text,
				COALESCE(u.primary_email::text, ''),
				u.display_name,
				u.picture_url
			FROM iam_external_identities AS i
			JOIN iam_users AS u
				ON u.user_id = i.user_id
			WHERE i.issuer = $1
			  AND i.subject = $2
			  AND u.status = 'active'
			FOR UPDATE OF u, i
		`,
		issuer,
		subject,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.PictureURL,
	)
	if err != nil {
		return domain.User{}, err
	}

	return user, nil
}

func createUser(
	ctx context.Context,
	transaction pgx.Tx,
	identity domain.ExternalIdentity,
) (domain.User, error) {
	var user domain.User

	err := transaction.QueryRow(
		ctx,
		`
			INSERT INTO iam_users (
				primary_email,
				display_name,
				picture_url
			)
			VALUES (
				NULLIF($1, '')::citext,
				$2,
				$3
			)
			RETURNING
				user_id::text,
				COALESCE(primary_email::text, ''),
				display_name,
				picture_url
		`,
		identity.Email,
		identity.DisplayName,
		identity.PictureURL,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.PictureURL,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf(
			"create internal user: %w",
			err,
		)
	}

	return user, nil
}

func updateUser(
	ctx context.Context,
	transaction pgx.Tx,
	userID string,
	identity domain.ExternalIdentity,
) (domain.User, error) {
	var user domain.User

	err := transaction.QueryRow(
		ctx,
		`
			UPDATE iam_users
			SET
				primary_email = CASE
					WHEN $2 <> '' AND $3
					THEN $2::citext
					ELSE primary_email
				END,
				display_name = CASE
					WHEN $4 <> ''
					THEN $4
					ELSE display_name
				END,
				picture_url = CASE
					WHEN $5 <> ''
					THEN $5
					ELSE picture_url
				END,
				updated_at = clock_timestamp()
			WHERE user_id = $1::uuid
			  AND status = 'active'
			RETURNING
				user_id::text,
				COALESCE(primary_email::text, ''),
				display_name,
				picture_url
		`,
		userID,
		identity.Email,
		identity.EmailVerified,
		identity.DisplayName,
		identity.PictureURL,
	).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.PictureURL,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf(
			"update internal user: %w",
			err,
		)
	}

	return user, nil
}

func upsertExternalIdentity(
	ctx context.Context,
	transaction pgx.Tx,
	userID string,
	identity domain.ExternalIdentity,
) error {
	rawClaims := identity.RawClaims

	if len(rawClaims) == 0 {
		rawClaims = json.RawMessage(`{}`)
	}

	if _, err := transaction.Exec(
		ctx,
		`
			INSERT INTO iam_external_identities (
				issuer,
				subject,
				user_id,
				email,
				email_verified,
				raw_claims
			)
			VALUES (
				$1,
				$2,
				$3::uuid,
				NULLIF($4, '')::citext,
				$5,
				$6::jsonb
			)
			ON CONFLICT (issuer, subject)
			DO UPDATE SET
				email = EXCLUDED.email,
				email_verified = EXCLUDED.email_verified,
				raw_claims = EXCLUDED.raw_claims,
				last_seen_at = clock_timestamp()
		`,
		identity.Issuer,
		identity.Subject,
		userID,
		identity.Email,
		identity.EmailVerified,
		string(rawClaims),
	); err != nil {
		return fmt.Errorf(
			"upsert external identity: %w",
			err,
		)
	}

	return nil
}
