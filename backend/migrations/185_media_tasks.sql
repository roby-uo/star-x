CREATE TABLE IF NOT EXISTS media_tasks (
 id TEXT PRIMARY KEY,
 user_id BIGINT NOT NULL,
 api_key_id BIGINT NOT NULL,
 group_id BIGINT NOT NULL,
 account_id BIGINT NOT NULL,
 model TEXT NOT NULL,
 idempotency_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 upstream_task_id TEXT,
 state TEXT NOT NULL DEFAULT 'submitting',
 billing_state TEXT NOT NULL DEFAULT 'reserved',
 quote JSONB NOT NULL,
 specification JSONB NOT NULL,
 hold_amount NUMERIC(20,10) NOT NULL DEFAULT 0,
 subscription_id BIGINT,
 result JSONB,
 error TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(api_key_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS media_tasks_user_created ON media_tasks(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS media_tasks_pending ON media_tasks(updated_at) WHERE state IN ('submitting','queued','running');

CREATE TABLE IF NOT EXISTS api_key_model_access (
 api_key_id BIGINT PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
 rules JSONB NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
