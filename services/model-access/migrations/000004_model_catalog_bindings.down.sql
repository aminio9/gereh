BEGIN;

DROP TABLE IF EXISTS
    model_access_binding_idempotency;

DROP TABLE IF EXISTS
    model_access_agent_binding_revisions;

DROP TABLE IF EXISTS
    model_access_agent_binding_fallbacks;

DROP TABLE IF EXISTS
    model_access_agent_bindings;

DROP TABLE IF EXISTS
    model_access_catalog_refresh_queue;

DROP TABLE IF EXISTS
    model_access_catalog_refreshes;

DROP TABLE IF EXISTS
    model_access_model_offerings;

DROP TABLE IF EXISTS
    model_access_catalog_states;

COMMIT;
