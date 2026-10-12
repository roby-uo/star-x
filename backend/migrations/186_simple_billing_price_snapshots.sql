-- Keep the price accepted before the upstream call in the same transaction as
-- balance deduction. Values are USD ledger amounts; CNY display uses fixed 6.2.
CREATE TABLE IF NOT EXISTS usage_billing_price_snapshots (
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    config_version BIGINT NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id)
);
CREATE INDEX IF NOT EXISTS idx_usage_billing_price_snapshots_user_created
    ON usage_billing_price_snapshots (user_id, created_at DESC);
