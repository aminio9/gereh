package postgres

import (
	"context"
	"fmt"

	"github.com/aminio9/gereh/services/projection/internal/domain"
)

func (transaction *projectionTransaction) AppendActivity(
	ctx context.Context,
	value domain.Activity,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_task_activity (
				tenant_id,
				event_id,
				event_type,
				company_id,
				project_id,
				task_id,
				actor_type,
				actor_id,
				summary,
				occurred_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2::uuid,
				$3,
				$4::uuid,
				$5::uuid,
				$6::uuid,
				$7,
				$8::uuid,
				$9,
				$10,
				$11
			)
			ON CONFLICT (
				tenant_id,
				event_id
			)
			DO NOTHING
		`,
		value.TenantID,
		value.EventID,
		value.EventType,
		value.CompanyID,
		value.ProjectID,
		value.TaskID,
		value.ActorType,
		value.ActorID,
		value.Summary,
		value.OccurredAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"append task activity projection: %w",
			err,
		)
	}

	return nil
}

func (transaction *projectionTransaction) UpsertSearchDocument(
	ctx context.Context,
	value domain.SearchDocument,
) error {
	_, err := transaction.tx.Exec(
		ctx,
		`
			INSERT INTO projection_search_documents (
				tenant_id,
				document_type,
				document_id,
				company_id,
				title,
				subtitle,
				body,
				status,
				deleted,
				source_version,
				source_event_id,
				updated_at,
				projected_at
			)
			VALUES (
				$1::uuid,
				$2,
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
			ON CONFLICT (
				tenant_id,
				document_type,
				document_id
			)
			DO UPDATE SET
				company_id =
					EXCLUDED.company_id,
				title =
					EXCLUDED.title,
				subtitle =
					EXCLUDED.subtitle,
				body =
					EXCLUDED.body,
				status =
					EXCLUDED.status,
				deleted =
					EXCLUDED.deleted,
				source_version =
					EXCLUDED.source_version,
				source_event_id =
					EXCLUDED.source_event_id,
				updated_at =
					EXCLUDED.updated_at,
				projected_at =
					EXCLUDED.projected_at
			WHERE
				(
					projection_search_documents.deleted
					OR
					projection_search_documents.source_version
					<
					EXCLUDED.source_version
				)
		`,
		value.TenantID,
		value.Type,
		value.ID,
		value.CompanyID,
		value.Title,
		value.Subtitle,
		value.Body,
		value.Status,
		value.Deleted,
		value.SourceVersion,
		value.SourceEventID,
		value.UpdatedAt,
		value.ProjectedAt,
	)
	if err != nil {
		return fmt.Errorf(
			"upsert search document projection: %w",
			err,
		)
	}

	return nil
}
