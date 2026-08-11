BEGIN;

UPDATE model_access_providers
SET
    enabled = TRUE,
    version = version + 1,
    updated_at = clock_timestamp()
WHERE provider_key = 'nous';

DROP TABLE IF EXISTS
    model_access_secret_cleanup;

DROP TABLE IF EXISTS
    model_access_credential_operations;

DROP TABLE IF EXISTS
    model_access_connection_verification_events;

DROP TABLE IF EXISTS
    model_access_connection_credentials;

ALTER TABLE model_access_connection_revisions
    DROP COLUMN IF EXISTS credential_fingerprint;

ALTER TABLE model_access_connections
    DROP CONSTRAINT IF EXISTS
        model_access_connections_credential_fingerprint_check;

ALTER TABLE model_access_connections
    DROP COLUMN IF EXISTS credential_fingerprint;

COMMIT;
