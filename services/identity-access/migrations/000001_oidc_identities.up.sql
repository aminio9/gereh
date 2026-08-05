BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE iam_users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    primary_email CITEXT,
    display_name TEXT NOT NULL DEFAULT '',
    picture_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT iam_users_status_check
        CHECK (status IN ('active', 'disabled'))
);

CREATE TABLE iam_external_identities (
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id UUID NOT NULL
        REFERENCES iam_users(user_id)
        ON DELETE CASCADE,
    email CITEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    raw_claims JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    PRIMARY KEY (issuer, subject)
);

CREATE INDEX iam_external_identities_user_id_idx
    ON iam_external_identities(user_id);

CREATE INDEX iam_users_primary_email_idx
    ON iam_users(primary_email)
    WHERE primary_email IS NOT NULL;

COMMIT;
