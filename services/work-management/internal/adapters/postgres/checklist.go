package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"github.com/aminio9/gereh/services/work-management/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AddChecklistItem commits a checklist item and its outbox event atomically.
func (repository *Repository) AddChecklistItem(
	ctx context.Context,
	params ports.AddChecklistItemParams,
) (domain.ChecklistItem, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Item.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	if err := taskExists(
		ctx,
		transaction,
		params.Item.TenantID,
		params.Item.TaskID,
	); err != nil {
		return domain.ChecklistItem{}, err
	}

	_, err = transaction.Exec(
		ctx,
		`
			INSERT INTO work_checklist_items (
				tenant_id,
				task_id,
				item_id,
				title,
				completed,
				position,
				version,
				created_at,
				updated_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3::uuid,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9
			)
		`,
		params.Item.TenantID,
		params.Item.TaskID,
		params.Item.ID,
		params.Item.Title,
		params.Item.Completed,
		params.Item.Position,
		params.Item.Version,
		params.Item.CreatedAt,
		params.Item.UpdatedAt,
	)
	if err != nil {
		return domain.ChecklistItem{}, mapDatabaseError(err)
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Item.TenantID,
		params.Event,
	); err != nil {
		return domain.ChecklistItem{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.ChecklistItem{}, err
	}

	return params.Item, nil
}

// GetChecklistItem returns one checklist item by identity.
func (repository *Repository) GetChecklistItem(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
	itemID string,
) (domain.ChecklistItem, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		tenantID,
		actorUserID,
		pgx.TxOptions{
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	item, err := scanChecklistItem(transaction.QueryRow(
		ctx,
		`
			SELECT
				tenant_id::text,
				task_id::text,
				item_id::text,
				title,
				completed,
				position,
				version,
				created_at,
				updated_at
			FROM work_checklist_items
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			  AND item_id = $3::uuid
		`,
		tenantID,
		taskID,
		itemID,
	))
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.ChecklistItem{}, err
	}

	return item, nil
}

// UpdateChecklistItem applies an optimistic-concurrency checklist item update.
func (repository *Repository) UpdateChecklistItem(
	ctx context.Context,
	params ports.UpdateChecklistItemParams,
) (domain.ChecklistItem, error) {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Item.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return domain.ChecklistItem{}, err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			UPDATE work_checklist_items
			SET
				title = $4,
				completed = $5,
				version = version + 1,
				updated_at = $6
			WHERE tenant_id = $1::uuid
			  AND item_id = $2::uuid
			  AND task_id = $3::uuid
			  AND version = $7
		`,
		params.Item.TenantID,
		params.Item.ID,
		params.Item.TaskID,
		params.Item.Title,
		params.Item.Completed,
		params.Item.UpdatedAt,
		params.ExpectedVersion,
	)
	if err != nil {
		return domain.ChecklistItem{}, mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		if err := rejectChecklistVersionOrMissing(
			ctx,
			transaction,
			params.Item.TenantID,
			params.Item.TaskID,
			params.Item.ID,
			params.ExpectedVersion,
		); err != nil {
			return domain.ChecklistItem{}, err
		}
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Item.TenantID,
		params.Event,
	); err != nil {
		return domain.ChecklistItem{}, err
	}

	if err := commit(ctx, transaction); err != nil {
		return domain.ChecklistItem{}, err
	}

	return params.Item, nil
}

// DeleteChecklistItem removes a checklist item.
func (repository *Repository) DeleteChecklistItem(
	ctx context.Context,
	params ports.DeleteChecklistItemParams,
) error {
	transaction, err := repository.beginUserTenant(
		ctx,
		params.Item.TenantID,
		params.ActorUserID,
		pgx.TxOptions{},
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	result, err := transaction.Exec(
		ctx,
		`
			DELETE FROM work_checklist_items
			WHERE tenant_id = $1::uuid
			  AND item_id = $2::uuid
			  AND task_id = $3::uuid
		`,
		params.Item.TenantID,
		params.Item.ID,
		params.Item.TaskID,
	)
	if err != nil {
		return mapDatabaseError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if err := insertOutbox(
		ctx,
		transaction,
		params.Item.TenantID,
		params.Event,
	); err != nil {
		return err
	}

	return commit(ctx, transaction)
}

// ListChecklist lists a task's checklist items ordered by position.
func (repository *Repository) ListChecklist(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	taskID string,
) ([]domain.ChecklistItem, error) {
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
				item_id::text,
				title,
				completed,
				position,
				version,
				created_at,
				updated_at
			FROM work_checklist_items
			WHERE tenant_id = $1::uuid
			  AND task_id = $2::uuid
			ORDER BY position, item_id
		`,
		tenantID,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list work checklist: %w",
			mapDatabaseError(err),
		)
	}
	defer rows.Close()

	var items []domain.ChecklistItem

	for rows.Next() {
		item, err := scanChecklistItem(rows)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate work checklist: %w",
			err,
		)
	}

	if err := commit(ctx, transaction); err != nil {
		return nil, err
	}

	return items, nil
}

func rejectChecklistVersionOrMissing(
	ctx context.Context,
	transaction pgx.Tx,
	tenantID string,
	taskID string,
	itemID string,
	expectedVersion int64,
) error {
	var currentVersion int64

	err := transaction.QueryRow(
		ctx,
		`
			SELECT version
			FROM work_checklist_items
			WHERE tenant_id = $1::uuid
			  AND item_id = $2::uuid
			  AND task_id = $3::uuid
		`,
		tenantID,
		itemID,
		taskID,
	).Scan(&currentVersion)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf(
			"query checklist item version: %w",
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

func scanChecklistItem(
	scanner rowScanner,
) (domain.ChecklistItem, error) {
	var item domain.ChecklistItem

	err := scanner.Scan(
		&item.TenantID,
		&item.TaskID,
		&item.ID,
		&item.Title,
		&item.Completed,
		&item.Position,
		&item.Version,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return domain.ChecklistItem{}, mapDatabaseError(err)
	}

	return item, nil
}
