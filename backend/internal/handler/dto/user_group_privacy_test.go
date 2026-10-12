package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserGroupRelationsDoNotExposePricing(t *testing.T) {
	price := .4
	g := &service.Group{ID: 7, Platform: "openai", Description: "采购折扣及内部费率说明", RateMultiplier: 1.4, ImageRateMultiplier: 2,
		VideoRateMultiplier: 3, PeakRateMultiplier: 2, ImagePrice1K: &price, VideoPrice480P: &price,
		WebSearchPricePerCall: &price, AllowImageGeneration: true,
	}
	for _, value := range []any{
		GroupFromService(g),
		APIKeyFromService(&service.APIKey{Group: g}),
		UsageLogFromService(&service.UsageLog{Group: g}),
		UserSubscriptionFromService(&service.UserSubscription{Group: g}),
	} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(raw), `"description"`)
		require.NotContains(t, string(raw), g.Description)
		for _, field := range []string{"rate_multiplier", "image_rate_multiplier", "video_rate_multiplier", "peak_rate_multiplier", "image_price_1k", "video_price_480p", "web_search_price_per_call"} {
			require.NotContains(t, string(raw), `"`+field+`"`)
		}
		require.Contains(t, string(raw), `"allow_image_generation":true`)
	}
	raw, err := json.Marshal(GroupFromServiceAdmin(g))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"rate_multiplier":1.4`)
	require.Contains(t, string(raw), `"description":"采购折扣及内部费率说明"`)
	require.Contains(t, string(raw), `"image_price_1k":0.4`)
	raw, err = json.Marshal(UsageLogFromServiceAdmin(&service.UsageLog{Group: g, TotalCost: .2}))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"rate_multiplier":1.4`)
	require.Contains(t, string(raw), `"total_cost":0.2`)
}

func TestAdminNestedGroupRelationsKeepPricing(t *testing.T) {
	group := &service.Group{RateMultiplier: 1.4, ImageRateMultiplier: 2, VideoRateMultiplier: 3}
	key := service.APIKey{Group: group}
	subscription := service.UserSubscription{Group: group}
	for _, value := range []any{
		APIKeyFromServiceAdmin(&key),
		UserFromServiceAdmin(&service.User{APIKeys: []service.APIKey{key}}),
		UserFromServiceAdmin(&service.User{Subscriptions: []service.UserSubscription{subscription}}),
		RedeemCodeFromServiceAdmin(&service.RedeemCode{Group: group}),
		UserSubscriptionFromServiceAdmin(&subscription),
		UsageLogFromServiceAdmin(&service.UsageLog{APIKey: &key}),
	} {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"rate_multiplier":1.4`)
		require.Contains(t, string(raw), `"image_rate_multiplier":2`)
		require.Contains(t, string(raw), `"video_rate_multiplier":3`)
	}
	user := UserFromService(&service.User{APIKeys: []service.APIKey{key}, Subscriptions: []service.UserSubscription{subscription}, GroupRates: map[int64]float64{7: .75}})
	raw, err := json.Marshal(user)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"rate_multiplier"`)
	require.NotContains(t, string(raw), `"group_rates"`)
}
