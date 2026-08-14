package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportedAgentClients(t *testing.T) {
	t.Run("openai exposes chat completions clients and conditionally claude", func(t *testing.T) {
		withoutMessages := supportedAgentClients("openai", false)
		require.Equal(t, []string{
			"codex", "codex-ws", "opencode", "hermes", "workbuddy", "trae-ide",
		}, withoutMessages)

		withMessages := supportedAgentClients("openai", true)
		require.Equal(t, []string{
			"codex", "codex-ws", "claude", "opencode", "hermes", "workbuddy", "trae-ide",
		}, withMessages)
	})

	t.Run("other platforms retain their existing guides", func(t *testing.T) {
		require.Equal(t, []string{"gemini", "opencode"}, supportedAgentClients("gemini", false))
		require.Equal(t, []string{"claude", "gemini", "opencode"}, supportedAgentClients("antigravity", false))
		require.Equal(t, []string{"grok", "claude", "codex", "opencode"}, supportedAgentClients("grok", false))
		require.Equal(t, []string{"claude", "opencode"}, supportedAgentClients("anthropic", false))
	})
}
