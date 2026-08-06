package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AddComment commits a task comment and its outbox event atomically.
func (repository *Repository) AddComment(
	ctx context.Context,
	params ports.AddCommentParams,
) (domain.Comment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Comment.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Comment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		params.Comment.TenantID,
		params.Comment.TaskID,
	); err != nil {
		return domain.Comment{}, err
	}

	if err := insertComment(
		ctx,
		transaction,
		params.Comment,
	); err != nil {
		return domain.Comment{}, err
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Comment.TenantID,
		params.Event,
	); err != nil {
		return domain.Comment{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Comment{}, err
	}

	return params.Comment, nil
}

// GetComment returns one comment by identity.
func (repository *Repository) GetComment(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	commentID string,
) (domain.Comment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Comment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	var comment domain.Comment
	var authorType string

	err = transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				task_id::text,
				comment_id::text,
				author_type,
				author_user_id::text,
				author_agent_id::text,
				body,
				version,
				created_at,
				updated_at,
				deleted_at
			FROM work_comments
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND comment_id = $3::uuid
		`,
		tenantID,
		taskID,
		commentID,
	).Scan(
		&comment.TenantID,
		&comment.TaskID,
		&comment.ID,
		&authorType,
		&comment.AuthorUserID,
		&comment.AuthorAgentID,
		&comment.Body,
		&comment.Version,
		&comment.CreatedAt,
		&comment.UpdatedAt,
		&comment.DeletedAt,
	)
	if err != nil {
		return domain.Comment{}, mapDatabaseError(err)
	}

	comment.AuthorType = domain.AssigneeType(authorType)

	if err := commit(ctx, transaction); err != nil {
		return domain.Comment{}, err
	}

	return comment, nil
}

// UpdateComment applies an optimistic-concurrency comment update. Only the
// original author may update; the application layer enforces ownership.
func (repository *Repository) UpdateComment(
	ctx context.Context,
	params ports.UpdateCommentParams,
) (domain.Comment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Comment.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Comment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_comments
			SET
				body = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND comment_id = $2::uuid
			  AND task_id = $3::uuid
			  AND version = $6
			  AND deleted_at IS NULL
		`,
		params.Comment.TenantID,
		params.Comment.ID,
		params.Comment.TaskID,
		params.Comment.Body,
		params.Comment.UpdatedAt,
		params.ExpectedVersion,
	)
	if err != nil {
		return domain.Comment{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectCommentVersionOrMissing(
			ctx,
			transaction,
			params.Comment.TenantID,
			params.Comment.TaskID,
			params.Comment.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Comment{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Comment.TenantID,
		params.Event,
	); err != nil {
		return domain.Comment{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Comment{}, err
	}

	return params.Comment, nil
}

// DeleteComment soft-deletes a comment.
func (repository *Repository) DeleteComment(
	ctx context.Context,
	params ports.DeleteCommentParams,
) (domain.Comment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Comment.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Comment{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_comments
			SET
				deleted_at = $4,
				version = version + 1,
				updated_at = $5
			WHERE tenant_id = $1::uuid
			  AND comment_id = $2::uuid
			  AND task_id = $3::uuid
			  AND version = $6
			  AND deleted_at IS NULL
		`,
		params.Comment.TenantID,
		params.Comment.ID,
		params.Comment.TaskID,
		params.Comment.DeletedAt,
		params.Comment.UpdatedAt,
		params.ExpectedVersion,
	)
	if err != nil {
		return domain.Comment{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectCommentVersionOrMissing(
			ctx,
			transaction,
			params.Comment.TenantID,
			params.Comment.TaskID,
			params.Comment.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.Comment{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Comment.TenantID,
		params.Event,
	); err != nil {
		return domain.Comment{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Comment{}, err
	}

	return params.Comment, nil
}

// ListComments lists a task's comments, newest first.
func (repository *Repository) ListComments(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Comment, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		tenantID,
		taskID,
	); err != nil {
		return nil, err
	}

	rows, err := transaction.Query(
		ctx,
		`
			SELECT
				tenant_id::text,
				task_id::text,
				comment_id::text,
				author_type,
				author_user_id::text,
				author_agent_id::text,
				body,
				version,
				created_at,
				updated_at,
				deleted_at
			FROM work_comments
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			ORDER BY created_at, comment_id
		`,
		tenantID,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list work comments: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var comments []domain.Comment

	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work comments: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return comments, nil
}

func rejectCommentVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	taskID string,
	commentID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM work_comments
			WHERE tenant_id = $1::uuid
			  AND comment_id = $2::uuid
			  AND task_id = $3::uuid
		`,
		tenantID,
		commentID,
		taskID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query comment version: %w",
			err,
		)
	}

	return fmt.Errorf(
		"%w: expected %d, found %d",
		domain.ErrVersionConflict,
		expectedVersion,
		currentVersion,
	)
}

func insertComment(
	ctx context.Context,
	transaction pgx.Tx,
	comment domain.Comment,
) error {
	_, err := transaction.Exec(
		ctx,
		`
			INSERT INTO work_comments (
				tenant_id,
				task_id,
				comment_id,
				author_type,
				author_user_id,
				author_agent_id,
				body,
				version,
				created_at,
				updated_at,
				deleted_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5::uuid,
				$6::uuid,
				$7,
				$8,
				$9,
				$10,
				$11
			)
		`,
		comment.TenantID,
		comment.TaskID,
		comment.ID,
		string(comment.AuthorType),
		comment.AuthorUserID,
		comment.AuthorAgentID,
		comment.Body,
		comment.Version,
		comment.CreatedAt,
		comment.UpdatedAt,
		comment.DeletedAt,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	return nil
}

func scanComment(
	scanner rowScanner,
) (domain.Comment, error) {
	var comment domain.Comment
	var authorType string

	err := scanner.Scan(
		&comment.TenantID,
		&comment.TaskID,
		&comment.ID,
		&authorType,
		&comment.AuthorUserID,
		&comment.AuthorAgentID,
		&comment.Body,
		&comment.Version,
		&comment.CreatedAt,
		&comment.UpdatedAt,
		&comment.DeletedAt,
	)
	if err != nil {
		return domain.Comment{}, mapDatabaseError(err)
	}

	comment.AuthorType = domain.AssigneeType(authorType)

	return comment, nil
}
