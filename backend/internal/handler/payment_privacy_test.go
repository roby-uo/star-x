//go:build unit

package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type paymentPrivacySettings struct {
	service.SettingRepository
}

func (*paymentPrivacySettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{service.SettingBalanceRechargeMult: ".2", service.SettingSubscriptionUSDToCNYRate: "7"}, nil
}

func TestUserPaymentPlansKeepPurchasePriceAndOmitRatePolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", "file:payment_privacy?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	group, err := client.Group.Create().SetName("VIP").SetPlatform(service.PlatformOpenAI).
		SetRateMultiplier(1.4).SetPeakRateEnabled(true).SetPeakStart("09:00").SetPeakEnd("18:00").SetPeakRateMultiplier(2).Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().SetGroupID(group.ID).SetName("Monthly VIP").SetPrice(49).
		SetCurrency("CNY").SetValidityDays(30).SetValidityUnit("day").SetForSale(true).Save(ctx)
	require.NoError(t, err)
	h := NewPaymentHandler(nil, service.NewPaymentConfigService(client, &paymentPrivacySettings{}, nil))
	for _, endpoint := range []struct {
		path   string
		handle gin.HandlerFunc
	}{
		{"/payment/plans", h.GetPlans},
		{"/payment/checkout-info", h.GetCheckoutInfo},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, endpoint.path, nil)
			endpoint.handle(c)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"price":49`)
			require.Contains(t, rec.Body.String(), `"currency":"CNY"`)
			for _, field := range []string{"rate_multiplier", "peak_rate_multiplier", "peak_rate_enabled", "peak_start", "peak_end"} {
				require.NotContains(t, rec.Body.String(), `"`+field+`"`)
			}
			if endpoint.path == "/payment/checkout-info" {
				require.Contains(t, rec.Body.String(), `"balance_recharge_multiplier":0.2`)
				require.Contains(t, rec.Body.String(), `"subscription_usd_to_cny_rate":7`)
			}
		})
	}
	// Administrator/provider cost policies must remain in persistent configuration.
	stored, err := client.Group.Get(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, 1.4, stored.RateMultiplier)
	require.Equal(t, 2.0, stored.PeakRateMultiplier)
}
