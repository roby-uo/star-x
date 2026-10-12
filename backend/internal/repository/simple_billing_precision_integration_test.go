//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// The integration harness creates a disposable PostgreSQL 18 database and
// applies the actual migration chain, including 186-188. These tests execute
// production repositories rather than approximating SQL with mocks.
func TestSimpleBillingPrecisionIntegration_TinyChargeSnapshotAndDedup(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Balance: 1})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-integration-" + uuid.NewString(), Name: "tiny-billing", Quota: 1})
	account := mustCreateAccount(t, client, &service.Account{Name: "tiny-billing-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 1.0}})
	const costUSD = 0.000000009
	inputPrice, freePrice := costUSD/1.4*1000000, 0.0
	q := &service.SimpleBillingSnapshot{Version: 11, AccountID: account.ID, Model: "integration-tiny-model", Modality: "chat", Route: "api", Source: "manual", Multiplier: 1.4, Currency: "USD", DiscountMultiplier: 1, BillingUnit: "per_million_tokens",
		Rows: []service.CostPriceRow{{ID: "tiny", Models: []string{"integration-tiny-model"}, Modality: "chat", BillingUnit: "per_million_tokens", InputPrice: &inputPrice, OutputPrice: &freePrice, CacheReadPrice: &freePrice, CacheWritePrice: &freePrice}}}
	q.RefreshHash()
	quoted, err := service.NewBillingService(&config.Config{}, nil).CalculateSimpleBilling(q, service.BillingPreviewInput{Model: q.Model, InputTokens: 1, Requests: 1})
	require.NoError(t, err)
	require.InDelta(t, costUSD, quoted.ActualCost, 1e-20)
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, AccountType: service.AccountTypeAPIKey, Model: q.Model, InputTokens: 1, PriceSnapshot: q,
		BalanceCost: quoted.ActualCost, APIKeyQuotaCost: quoted.ActualCost, APIKeyRateLimitCost: quoted.ActualCost, AccountQuotaCost: quoted.ActualCost}
	repo := NewUsageBillingRepository(client, integrationDB)
	first, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, second.Applied)

	assertStoredDecimal := func(query, expected string, args ...any) {
		t.Helper()
		var actual string
		require.NoError(t, integrationDB.QueryRowContext(ctx, query, args...).Scan(&actual))
		value, err := decimal.NewFromString(actual)
		require.NoError(t, err)
		want, err := decimal.NewFromString(expected)
		require.NoError(t, err)
		require.True(t, value.Equal(want), "%s: expected %s, got %s", query, expected, actual)
	}
	assertStoredDecimal("SELECT balance::text FROM users WHERE id=$1", "0.999999991", user.ID)
	assertStoredDecimal("SELECT quota_used::text FROM api_keys WHERE id=$1", "0.000000009", key.ID)
	for _, column := range []string{"usage_5h", "usage_1d", "usage_7d"} {
		assertStoredDecimal("SELECT "+column+"::text FROM api_keys WHERE id=$1", "0.000000009", key.ID)
	}
	assertStoredDecimal("SELECT (extra->>'quota_used')::numeric::text FROM accounts WHERE id=$1", "0.000000009", account.ID)

	var snapshotRaw []byte
	var version int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT config_version,snapshot FROM usage_billing_price_snapshots WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, key.ID).Scan(&version, &snapshotRaw))
	var stored service.SimpleBillingSnapshot
	require.NoError(t, json.Unmarshal(snapshotRaw, &stored))
	require.Equal(t, int64(11), version)
	require.Equal(t, q.Hash, stored.Hash)
	require.Equal(t, 1.4, stored.Multiplier)
	var snapshotCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_billing_price_snapshots WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, key.ID).Scan(&snapshotCount))
	require.Equal(t, 1, snapshotCount)

	// Usage logs are recorded separately after Apply by the gateway. Exercise
	// the actual insert repository too, so the tiny charge cannot survive in
	// the wallet while silently rounding to zero in usage/account statistics.
	accountStatsCost := costUSD
	created, err := NewUsageLogRepository(client, integrationDB).Create(ctx, &service.UsageLog{RequestID: cmd.RequestID, UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, Model: q.Model, InputTokens: 1, TotalCost: costUSD, ActualCost: costUSD, AccountStatsCost: &accountStatsCost, RateMultiplier: 1.4, CreatedAt: time.Now()})
	require.NoError(t, err)
	require.True(t, created)
	assertStoredDecimal("SELECT actual_cost::text FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", "0.000000009", cmd.RequestID, key.ID)
	assertStoredDecimal("SELECT account_stats_cost::text FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", "0.000000009", cmd.RequestID, key.ID)
}

func TestSimpleBillingPrecisionIntegration_MetadataFailureRollsBackClaimAndCharge(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Balance: 1})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-integration-" + uuid.NewString(), Name: "failed-metadata", Quota: 1})
	repo := NewUsageBillingRepository(client, integrationDB)
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: key.ID, BalanceCost: .5, APIKeyQuotaCost: .5,
		PriceSnapshot: &service.SimpleBillingSnapshot{Version: 12, Model: "integration-model", Multiplier: math.NaN()}}
	_, err := repo.Apply(ctx, cmd)
	require.Error(t, err, "a snapshot that cannot be serialized must fail the billing transaction")
	var balance, quota string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance::text FROM users WHERE id=$1", user.ID).Scan(&balance))
	require.Equal(t, "1.000000000000", balance)
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used::text FROM api_keys WHERE id=$1", key.ID).Scan(&quota))
	require.Equal(t, "0.000000000000", quota)
	var claimCount, snapshotCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2", cmd.RequestID, key.ID).Scan(&claimCount))
	require.Zero(t, claimCount, "failed metadata must not consume the idempotency key")
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_price_snapshots WHERE request_id=$1 AND api_key_id=$2", cmd.RequestID, key.ID).Scan(&snapshotCount))
	require.Zero(t, snapshotCount)

	// The same request may be retried after fixing its metadata. This succeeds
	// once because the failed transaction left both money and claim untouched.
	cmd.PriceSnapshot.Multiplier = 1.4
	cmd.PriceSnapshot.RefreshHash()
	cmd.RequestFingerprint = ""
	result, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func TestSimpleBillingPrecisionIntegration_LaterEffectFailureRollsBackSnapshot(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Balance: 1})
	// No API key has this negative id. The snapshot insert and wallet update
	// happen first; the later quota error must roll both changes back.
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: user.ID, APIKeyID: -1, BalanceCost: .25, APIKeyQuotaCost: .25,
		PriceSnapshot: &service.SimpleBillingSnapshot{Version: 13, Model: "integration-model", Multiplier: 1.4}}
	_, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, cmd)
	require.Error(t, err)
	var balance string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance::text FROM users WHERE id=$1", user.ID).Scan(&balance))
	require.Equal(t, "1.000000000000", balance)
	var snapshots, claims int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_price_snapshots WHERE request_id=$1", cmd.RequestID).Scan(&snapshots))
	require.Zero(t, snapshots)
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id=$1", cmd.RequestID).Scan(&claims))
	require.Zero(t, claims)
}

func TestSimpleBillingPrecisionIntegration_MigrationsKeepCashAndLegacyPlans(t *testing.T) {
	ctx := context.Background()
	for _, column := range []struct {
		table, name string
		scale       int
	}{{"users", "balance", 12}, {"api_keys", "quota_used", 12}, {"usage_logs", "actual_cost", 12}, {"payment_orders", "amount", 12}, {"payment_orders", "refund_amount", 12}, {"payment_orders", "pay_amount", 2}, {"subscription_plans", "cash_price_cny", 2}} {
		var scale int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT numeric_scale FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name=$2`, column.table, column.name).Scan(&scale))
		require.Equal(t, column.scale, scale, "%s.%s precision", column.table, column.name)
	}
	group := mustCreateGroup(t, testEntClient(t), &service.Group{Name: "legacy-cash-plan-" + uuid.NewString()})
	var planID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO subscription_plans(group_id,name,price,currency) VALUES($1,'legacy price label',20,'CNY') RETURNING id`, group.ID).Scan(&planID))
	require.NoError(t, ApplyMigrations(ctx, integrationDB))
	var price string
	var cashIsNull, originalCashIsNull bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT price::text,cash_price_cny IS NULL,cash_original_price_cny IS NULL FROM subscription_plans WHERE id=$1`, planID).Scan(&price, &cashIsNull, &originalCashIsNull))
	require.Equal(t, "20.00", price)
	require.True(t, cashIsNull)
	require.True(t, originalCashIsNull, "a display-only old CNY label must not create a new cash price")
}
