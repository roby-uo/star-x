-- Keep the existing USD ledger and historical values. New CNY payments are
-- credited as CNY / 6.2, which needs ledger precision rather than cash cents.
-- pay_amount remains the original provider amount and is deliberately unchanged.
ALTER TABLE payment_orders
    ALTER COLUMN amount TYPE DECIMAL(30,12),
    ALTER COLUMN refund_amount TYPE DECIMAL(30,12);

-- Twelve fractional digits keep a low-cost token charge from disappearing in
-- the balance while still being present in usage records. Only numeric ledger
-- and internal unit-price columns are widened. Cash payments, subscription
-- cash prices, discounts and rate multipliers retain their original types.
-- Preserve every column's previous integer capacity, including older variants.
DO $$
DECLARE ledger_column RECORD;
BEGIN
    FOR ledger_column IN
        SELECT c.table_schema, c.table_name, c.column_name,
               GREATEST(24, c.numeric_precision - c.numeric_scale + 12) AS target_precision
        FROM information_schema.columns c
        WHERE c.table_schema = current_schema()
          AND c.data_type = 'numeric' AND c.numeric_scale < 12
          AND (
            (c.table_name = 'users' AND c.column_name IN ('balance','frozen_balance','total_recharged','balance_notify_threshold'))
            OR (c.table_name = 'api_keys' AND c.column_name IN ('quota','quota_used','rate_limit_5h','rate_limit_1d','rate_limit_7d','usage_5h','usage_1d','usage_7d'))
            OR (c.table_name = 'groups' AND c.column_name IN ('daily_limit_usd','weekly_limit_usd','monthly_limit_usd','image_price_1k','image_price_2k','image_price_4k','video_price_480p','video_price_720p','video_price_1080p','web_search_price_per_call'))
            OR (c.table_name = 'user_subscriptions' AND c.column_name IN ('daily_usage_usd','weekly_usage_usd','monthly_usage_usd'))
            OR (c.table_name = 'user_platform_quotas' AND c.column_name IN ('daily_limit_usd','weekly_limit_usd','monthly_limit_usd','daily_usage_usd','weekly_usage_usd','monthly_usage_usd'))
            OR (c.table_name = 'redeem_codes' AND c.column_name = 'value')
            OR (c.table_name IN ('promo_codes','promo_code_usages') AND c.column_name = 'bonus_amount')
            OR (c.table_name = 'usage_logs' AND c.column_name IN ('input_cost','output_cost','cache_creation_cost','cache_read_cost','image_input_cost','image_output_cost','total_cost','actual_cost','account_stats_cost'))
            OR (c.table_name IN ('usage_dashboard_hourly','usage_dashboard_daily') AND c.column_name IN ('total_cost','actual_cost','account_cost'))
            OR (c.table_name = 'billing_usage_entries' AND c.column_name = 'delta_usd')
            OR (c.table_name = 'batch_image_jobs' AND c.column_name IN ('estimated_cost','hold_amount','actual_cost','base_unit_price','billable_unit_price','hold_unit_price'))
            OR (c.table_name = 'batch_image_items' AND c.column_name = 'billed_amount')
            OR (c.table_name = 'media_tasks' AND c.column_name = 'hold_amount')
            OR (c.table_name = 'user_affiliates' AND c.column_name IN ('aff_quota','aff_history_quota','aff_frozen_quota'))
            OR (c.table_name = 'user_affiliate_ledger' AND c.column_name IN ('amount','balance_after','aff_quota_after','aff_frozen_quota_after','aff_history_quota_after'))
            OR (c.table_name IN ('channel_model_pricing','channel_model_pricing_intervals','channel_account_stats_model_pricing','channel_account_stats_pricing_intervals') AND c.column_name IN ('input_price','output_price','cache_write_price','cache_read_price','image_input_price','image_output_price','per_request_price'))
          )
    LOOP
        EXECUTE format('ALTER TABLE %I.%I ALTER COLUMN %I TYPE NUMERIC(%s,12)',
            ledger_column.table_schema, ledger_column.table_name,
            ledger_column.column_name, ledger_column.target_precision);
    END LOOP;
END $$;
