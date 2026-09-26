package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAccountSupportsAdminEndpoint(t *testing.T) {
	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "gpt-image-2": "gpt-image-2"},
		},
	}
	if !accountSupportsAdminEndpoint(account, "chat") || !accountSupportsAdminEndpoint(account, "images") {
		t.Fatal("OpenAI account with text and image models should match both endpoints")
	}

	textOnly := &service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"deepseek-chat": "deepseek-chat"},
		},
	}
	if accountSupportsAdminEndpoint(textOnly, "images") {
		t.Fatal("OpenAI-compatible text account must not be listed as an image account")
	}
	if isAdminImageModel("grok-imagine-video-1.5") {
		t.Fatal("Grok video model must not be listed as an image model")
	}
	anthropic := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey}
	if !accountSupportsAdminEndpoint(anthropic, "responses") {
		t.Fatal("Anthropic accounts support the gateway Responses adapter")
	}
}
