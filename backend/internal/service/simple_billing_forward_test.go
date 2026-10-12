package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSimpleBillingPublicForwardsBlockUnconfirmedProcurementBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		forward func(*OpenAIGatewayService, *gin.Context, *Account) error
	}{
		{"responses", func(s *OpenAIGatewayService, c *gin.Context, a *Account) error {
			_, err := s.Forward(context.Background(), c, a, []byte(`{"model":"custom-model","input":"Hello"}`))
			return err
		}},
		{"chat", func(s *OpenAIGatewayService, c *gin.Context, a *Account) error {
			_, err := s.ForwardAsChatCompletions(context.Background(), c, a, []byte(`{"model":"custom-model","messages":[]}`), "", "")
			return err
		}},
		{"messages", func(s *OpenAIGatewayService, c *gin.Context, a *Account) error {
			_, err := s.ForwardAsAnthropic(context.Background(), c, a, []byte(`{"model":"custom-model","messages":[]}`), "", "")
			return err
		}},
		{"embeddings", func(s *OpenAIGatewayService, c *gin.Context, a *Account) error {
			_, err := s.ForwardEmbeddings(context.Background(), c, a, []byte(`{"model":"custom-model","input":"Hello"}`), "")
			return err
		}},
		{"images", func(s *OpenAIGatewayService, c *gin.Context, a *Account) error {
			_, err := s.ForwardImages(context.Background(), c, a, []byte(`{"model":"gpt-image-2","prompt":"Draw an icon","size":"2K"}`), &OpenAIImagesRequest{Model: "gpt-image-2", Size: "2K", SizeTier: "2K", ContentType: "application/json", Endpoint: openAIImagesGenerationsEndpoint}, "")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &memoryCostBook{raw: `{"simple_billing_enabled":true,"api_sale_multiplier":1.4,"cost_profiles":[]}`}
			billing := ProvideBillingService(&config.Config{}, nil, repo)
			// No transport is supplied: reaching a paid upstream call would panic
			// rather than returning the expected local pricing rejection.
			s := &OpenAIGatewayService{cfg: &config.Config{}, billingService: billing}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+test.name, strings.NewReader("{}"))
			c.Set("api_key", &APIKey{Group: &Group{ID: 7, RateMultiplier: 5}})
			account := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://unconfirmed.example/v1"}}
			err := test.forward(s, c, account)
			require.Error(t, err)
			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			require.Contains(t, recorder.Body.String(), "billing configuration")
			require.NotContains(t, recorder.Body.String(), "unconfirmed.example")
		})
	}
}

func TestLegacyStoredDollarLabelsRenderWalletAmountsAsYuan(t *testing.T) {
	result, err := renderNotificationEmail(NotificationEmailEventBalanceRechargeSuccess, "Recharge ${{recharge_amount}}", `<p>${{recharge_amount}} / ${{current_balance}}</p>`, map[string]string{"recharge_amount": "100.00", "current_balance": "620.00"}, nil)
	require.NoError(t, err)
	require.Equal(t, "Recharge ¥100.00", result.Subject)
	require.Contains(t, result.HTML, "¥100.00 / ¥620.00")
	require.NotContains(t, result.HTML, "$620")
}
