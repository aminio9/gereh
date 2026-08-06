package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AddArtifact commits artifact metadata and its outbox event atomically.
func (repository *Repository) AddArtifact(
	ctx context.Context,
	params ports.AddArtifactParams,
) (domain.Artifact, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Artifact.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Artifact{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		params.Artifact.TenantID,
		params.Artifact.TaskID,
	); err != nil {
		return domain.Artifact{}, err
	}

	metadata, err := json.Marshal(params.Artifact.Metadata)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf(
			"marshal artifact metadata: %w",
			err,
		)
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO work_artifacts (
				tenant_id,
				company_id,
				task_id,
				artifact_id,
				object_key,
				file_name,
				content_type,
				size_bytes,
				sha256,
				metadata,
				created_by_user_id,
				created_at,
				deleted_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4::uuid,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11::uuid,
				$12,
				$13
			)
		`,
		params.Artifact.TenantID,
		params.Artifact.CompanyID,
		params.Artifact.TaskID,
		params.Artifact.ID,
		params.Artifact.ObjectKey,
		params.Artifact.FileName,
		params.Artifact.ContentType,
		params.Artifact.SizeBytes,
		params.Artifact.SHA256,
		metadata,
		params.Artifact.CreatedByUserID,
		params.Artifact.CreatedAt,
		params.Artifact.DeletedAt,
	)
	if err != nil {
		return domain.Artifact{}, mapDatabaseError(err)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Artifact.TenantID,
		params.Event,
	); err != nil {
		return domain.Artifact{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Artifact{}, err
	}

	return params.Artifact, nil
}

// GetArtifact returns one artifact by identity.
func (repository *Repository) GetArtifact(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	artifactID string,
) (domain.Artifact, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.Artifact{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	artifact, err := scanArtifact(transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				company_id::text,
				task_id::text,
				artifact_id::text,
				object_key,
				file_name,
				content_type,
				size_bytes,
				sha256,
				metadata,
				created_by_user_id::text,
				created_at,
				deleted_at
			FROM work_artifacts
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND artifact_id = $3::uuid
		`,
		tenantID,
		taskID,
		artifactID,
	))
	if err != nil {
		return domain.Artifact{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Artifact{}, err
	}

	return artifact, nil
}

// DeleteArtifact soft-deletes artifact metadata.
func (repository *Repository) DeleteArtifact(
	ctx context.Context,
	params ports.DeleteArtifactParams,
) (domain.Artifact, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Artifact.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.Artifact{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_artifacts
			SET
				deleted_at = $4
			WHERE tenant_id = $1::uuid
			  AND artifact_id = $2::uuid
			  AND task_id = $3::uuid
			  AND deleted_at IS NULL
		`,
		params.Artifact.TenantID,
		params.Artifact.ID,
		params.Artifact.TaskID,
		params.Artifact.DeletedAt,
	)
	if err != nil {
		return domain.Artifact{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.Artifact{}, domain.ErrNotFound
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Artifact.TenantID,
		params.Event,
	); err != nil {
		return domain.Artifact{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.Artifact{}, err
	}

	return params.Artifact, nil
}

// ListArtifacts lists a task's non-deleted artifacts.
func (repository *Repository) ListArtifacts(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.Artifact, error) {
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
				company_id::text,
				task_id::text,
				artifact_id::text,
				object_key,
				file_name,
				content_type,
				size_bytes,
				sha256,
				metadata,
				created_by_user_id::text,
				created_at,
				deleted_at
			FROM work_artifacts
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND deleted_at IS NULL
			ORDER BY created_at, artifact_id
		`,
		tenantID,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list work artifacts: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var artifacts []domain.Artifact

	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}

		artifacts = append(artifacts, artifact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work artifacts: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return artifacts, nil
}

func scanArtifact(
	scanner rowScanner,
) (domain.Artifact, error) {
	var artifact domain.Artifact
	var metadata []byte

	err := scanner.Scan(
		&artifact.TenantID,
		&artifact.CompanyID,
		&artifact.TaskID,
		&artifact.ID,
		&artifact.ObjectKey,
		&artifact.FileName,
		&artifact.ContentType,
		&artifact.SizeBytes,
		&artifact.SHA256,
		&metadata,
		&artifact.CreatedByUserID,
		&artifact.CreatedAt,
		&artifact.DeletedAt,
	)
	if err != nil {
		return domain.Artifact{}, mapDatabaseError(err)
	}

	if len(metadata) > 0 {
		if err := json.Unmarshal(
			metadata,
			&artifact.Metadata,
		); err != nil {
			return domain.Artifact{}, fmt.Errorf(
				"decode artifact metadata: %w",
				err,
			)
		}
	}

	return artifact, nil
}
