package service

import (
	"context"
	"encoding/json"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const simpleBillingWSUnavailableMessage = "Model billing configuration is not confirmed. Please contact the site administrator."

func (s *OpenAIGatewayService) prepareOpenAIWSSimpleBilling(ctx context.Context, c *gin.Context, account *Account, finalModel string, payload []byte) (*SimpleBillingSnapshot, error) {
	return s.prepareSimpleBillingTextForward(ctx, c, account, finalModel, payload)
}

func (s *OpenAIGatewayService) simpleBillingWSPreflightError(ctx context.Context, conn *coderws.Conn) error {
	if conn != nil {
		payload, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]string{"type": "service_unavailable", "code": "pricing_unconfirmed", "message": simpleBillingWSUnavailableMessage}})
		writeCtx, cancel := context.WithTimeout(ctx, s.openAIWSWriteTimeout())
		_ = conn.Write(writeCtx, coderws.MessageText, payload)
		cancel()
	}
	// Do not put supplier pricing details in the websocket close reason/cause.
	return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, simpleBillingWSUnavailableMessage, nil)
}
