package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func (s *OpenAIGatewayService) prepareSimpleBillingTextForward(ctx context.Context, c *gin.Context, account *Account, model string, body []byte) (*SimpleBillingSnapshot, error) {
	in := simpleBillingForwardInput(c, account, model, "chat", "", "", strings.TrimSpace(gjson.GetBytes(body, "service_tier").String()))
	in.HasImageInput = simpleBillingHasImageInput(body)
	snapshot, err := s.PrepareSimpleBilling(ctx, in)
	if err != nil || snapshot == nil {
		return snapshot, err
	}
	count := 0
	quality := ""
	gjson.GetBytes(body, "tools").ForEach(func(_, tool gjson.Result) bool {
		if tool.Get("type").String() == "image_generation" {
			count++
			quality = tool.Get("quality").String()
		}
		return true
	})
	if count == 0 {
		return snapshot, nil
	}
	if count > 1 {
		return nil, simplePriceError("多份图片生成工具的组合计费尚未确认")
	}
	image, err := resolveOpenAIResponsesImageBillingConfigDetailedFromBody(body, model)
	if err != nil {
		return nil, err
	}
	resolution := image.InputSize
	if resolution == "" {
		resolution = image.SizeTier
	}
	// tool.model is sent verbatim by these adapters. Applying an account alias
	// only to the price lookup would charge for a different actual model.
	media, err := s.PrepareSimpleBilling(ctx, simpleBillingForwardInput(c, account, image.Model, "image", resolution, quality, ""))
	if err != nil {
		return nil, err
	}
	snapshot.ImageBilling = media
	snapshot.RefreshHash()
	return snapshot, nil
}

func simpleBillingHasImageInput(body []byte) bool {
	var visit func(gjson.Result) bool
	visit = func(value gjson.Result) bool {
		if value.IsObject() {
			switch value.Get("type").String() {
			case "image", "image_url", "input_image":
				return true
			}
			for _, key := range []string{"inlineData", "inline_data", "fileData", "file_data"} {
				part := value.Get(key)
				if strings.HasPrefix(part.Get("mimeType").String(), "image/") || strings.HasPrefix(part.Get("mime_type").String(), "image/") {
					return true
				}
			}
			for _, key := range []string{"content", "parts"} {
				if visit(value.Get(key)) {
					return true
				}
			}
		} else if value.IsArray() {
			found := false
			value.ForEach(func(_, item gjson.Result) bool { found = visit(item); return !found })
			return found
		}
		return false
	}
	for _, key := range []string{"messages", "input", "contents"} {
		if visit(gjson.GetBytes(body, key)) {
			return true
		}
	}
	return false
}

// Seedream's ordinary image price can include additional reference images.
// Layered outputs and automatic multi-image batches require a different table.
func simpleBillingImageScene(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	if gjson.GetBytes(body, "layered").Bool() || gjson.GetBytes(body, "layers").Exists() || gjson.GetBytes(body, "sequential_image_generation").String() == "auto" {
		return ""
	}
	for _, field := range []string{"generation_mode", "output_mode", "generationMode", "scene"} {
		mode := strings.TrimSpace(gjson.GetBytes(body, field).String())
		if mode != "" && mode != "single_image" && mode != "single" {
			return ""
		}
	}
	return "single_image"
}

// The model is already mapped to the selected upstream. Capturing the selected
// group here keeps procurement pricing independent of user override multipliers.
func simpleBillingForwardInput(c *gin.Context, account *Account, model, modality, resolution, quality, tier string) SimpleBillingInput {
	in := SimpleBillingInput{Account: account, Model: model, Modality: modality, Resolution: resolution, Quality: quality, ServiceTier: tier}
	if c != nil {
		if key := getAPIKeyFromContext(c); key != nil {
			in.Group = key.Group
		}
	}
	return in
}

// Pricing failures are emitted before model generation. Do not expose supplier
// contracts or procurement details through a user-facing gateway response.
func writeSimpleBillingPreflightError(c *gin.Context) {
	if c == nil || c.Writer.Written() {
		return
	}
	message := "Model billing configuration is not confirmed. Please contact the site administrator."
	if c.Request != nil && strings.Contains(c.Request.URL.Path, "/v1beta/") {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": http.StatusServiceUnavailable, "status": "UNAVAILABLE", "message": message}})
		return
	}
	if c.Request != nil && strings.HasSuffix(c.Request.URL.Path, "/messages") {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": message}})
		return
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "service_unavailable", "code": "pricing_unconfirmed", "message": message}})
}
