package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

type simpleBillingProtocolContextKey struct{}
type simpleBillingProtocolPreparer func(context.Context, SimpleBillingInput) (*SimpleBillingSnapshot, error)

// WithSimpleBillingContext lets protocol adapters use the existing gateway's
// billing service after they have resolved the exact upstream model. Each call
// captures its own immutable price snapshot; no additional service wiring is
// required for Gemini and Antigravity adapters.
func (s *GatewayService) WithSimpleBillingContext(ctx context.Context, group *Group) context.Context {
	if s == nil {
		return ctx
	}
	prepare := simpleBillingProtocolPreparer(func(callCtx context.Context, in SimpleBillingInput) (*SimpleBillingSnapshot, error) {
		in.Group = group
		return s.PrepareSimpleBilling(callCtx, in)
	})
	return context.WithValue(ctx, simpleBillingProtocolContextKey{}, prepare)
}

func prepareProtocolSimpleBilling(ctx context.Context, c *gin.Context, account *Account, model string, body []byte, imageSize string) (*SimpleBillingSnapshot, error) {
	prepare, _ := ctx.Value(simpleBillingProtocolContextKey{}).(simpleBillingProtocolPreparer)
	if prepare == nil {
		return nil, nil
	}
	modality := "chat"
	if isImageGenerationModel(model) {
		modality = "image"
	}
	in := simpleBillingForwardInput(c, account, model, modality, imageSize, "", "")
	in.HasImageInput = simpleBillingHasImageInput(body)
	return prepare(ctx, in)
}
