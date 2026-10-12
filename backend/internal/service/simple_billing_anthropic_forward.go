package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Anthropic's effective fast tier is carried by the final upstream beta header
// (or Bedrock body beta tokens), not by the client's original service_tier.
func (s *GatewayService) prepareSimpleBillingAnthropicForward(ctx context.Context, c *gin.Context, account *Account, model string, body []byte, headers http.Header) (*SimpleBillingSnapshot, error) {
	if actualModel := strings.TrimSpace(gjson.GetBytes(body, "model").String()); actualModel != "" {
		model = actualModel
	}
	tier := ""
	if containsBetaToken(headers.Get("anthropic-beta"), claude.BetaFastMode) {
		tier = "priority"
	}
	gjson.GetBytes(body, "anthropic_beta").ForEach(func(_, token gjson.Result) bool {
		if strings.TrimSpace(token.String()) == claude.BetaFastMode {
			tier = "priority"
		}
		return true
	})
	in := simpleBillingForwardInput(c, account, model, "chat", "", "", tier)
	in.HasImageInput = simpleBillingHasImageInput(body)
	return s.PrepareSimpleBilling(ctx, in)
}
