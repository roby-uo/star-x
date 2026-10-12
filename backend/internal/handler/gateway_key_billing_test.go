package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newKeyBillingContext(apiKey *service.APIKey) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/sub2api/billing", nil)
	if apiKey != nil {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	}
	return c, w
}

func TestGatewayHandlerKeyBillingInfoDoesNotDiscloseRates(t *testing.T) {
	groupID := int64(7)
	apiKey := &service.APIKey{UserID: 11, GroupID: &groupID, Key: "sk-sensitive-value", Group: &service.Group{
		ID: groupID, Name: "private-group-name", RateMultiplier: .75,
		PeakRateEnabled: true, PeakStart: "09:00", PeakEnd: "18:00", PeakRateMultiplier: 1.5,
		ImageRateMultiplier: 2, VideoRateMultiplier: 3,
	}}
	c, w := newKeyBillingContext(apiKey)
	// No rate repository is needed because this user endpoint never exposes rates.
	(&GatewayHandler{}).KeyBillingInfo(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var got keyBillingInfoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "sub2api.key_billing", got.Object)
	require.Equal(t, 2, got.SchemaVersion)
	require.Equal(t, "admin_only", got.PricingVisibility)
	require.False(t, got.ObservedAt.IsZero())
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fields))
	for _, field := range []string{"group_rate_multiplier", "user_rate_multiplier", "resolved_rate_multiplier", "effective_rate_multiplier", "peak_rate_enabled", "peak_start", "peak_end", "peak_rate_multiplier", "applied_peak_multiplier"} {
		require.NotContains(t, fields, field)
	}
	require.NotContains(t, w.Body.String(), apiKey.Key)
	require.NotContains(t, w.Body.String(), apiKey.Group.Name)
}

func TestGatewayHandlerKeyBillingInfoErrorsAreSafe(t *testing.T) {
	groupID := int64(7)
	for _, tc := range []struct {
		name   string
		key    *service.APIKey
		status int
	}{
		{"missing API key", nil, http.StatusUnauthorized},
		{"ungrouped API key", &service.APIKey{}, http.StatusForbidden},
		{"missing group", &service.APIKey{GroupID: &groupID}, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newKeyBillingContext(tc.key)
			(&GatewayHandler{}).KeyBillingInfo(c)
			require.Equal(t, tc.status, w.Code)
			require.NotContains(t, w.Body.String(), "multiplier")
		})
	}
}

func TestGatewayHandlerKeyBillingInfoSimpleModeRemainsUnsupported(t *testing.T) {
	groupID := int64(7)
	c, w := newKeyBillingContext(&service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID}})
	(&GatewayHandler{cfg: &config.Config{RunMode: config.RunModeSimple}}).KeyBillingInfo(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.NotContains(t, w.Body.String(), "multiplier")
}
