-- currency was historically only a label. A new explicit cash price avoids
-- reinterpreting existing labels or increasing the checkout of any old plan.
-- Do not backfill: NULL means retain that plan's legacy checkout policy.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS cash_price_cny DECIMAL(20,2) NULL,
    ADD COLUMN IF NOT EXISTS cash_original_price_cny DECIMAL(20,2) NULL;
