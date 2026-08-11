package postgres

// connectionColumns is the shared column projection consumed by
// scanConnection. Any SELECT/RETURNING that feeds scanConnection must use
// exactly this order.
const connectionColumns = `
	tenant_id::text,
	connection_id::text,
	provider_key,
	provider_pool_key,
	connection_type,
	display_name,
	status,
	credential_fingerprint,
	version,
	created_by_user_id::text,
	created_at,
	updated_at,
	archived_at
`
