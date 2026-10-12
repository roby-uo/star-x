package openai_ws_v2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPassthroughUsageCapturesTrustedImageInputForEachTerminal(t *testing.T) {
	for _, details := range []string{"input_tokens_details", "prompt_tokens_details"} {
		t.Run(details, func(t *testing.T) {
			state := &relayState{}
			body := []byte(`{"response":{"usage":{"input_tokens":1000,"output_tokens":10,"` + details + `":{"image_tokens":350}}}}`)
			first := parseUsageAndAccumulate(state, body, "response.completed", nil)
			require.Equal(t, 350, first.ImageInputTokens)
			second := parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":20,"output_tokens":5}}}`), "response.completed", nil)
			require.Zero(t, second.ImageInputTokens, "a text-only turn must not inherit image usage")
			require.Equal(t, 350, state.usage.ImageInputTokens)
			require.Equal(t, 1020, state.usage.InputTokens)
		})
	}
	state := &relayState{}
	clientUsage := []byte(`{"response":{"usage":{"input_tokens":1000,"output_tokens":10,"input_tokens_details":{"image_tokens":350}}}}`)
	parsed := parseUsageAndAccumulate(state, clientUsage, "response.create", nil)
	require.Zero(t, parsed.ImageInputTokens, "client response.create cannot supply chargeable usage")
	parsed = parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":10,"output_tokens":1,"input_tokens_details":{"image_tokens":-1}}}}`), "response.completed", nil)
	require.Zero(t, parsed.ImageInputTokens, "negative supplier usage must not reduce a charge")
	parsed = parseUsageAndAccumulate(state, []byte(`{"response":{"usage":{"input_tokens":1000,"output_tokens":500},"tool_usage":{"image_gen":{"input_tokens_details":{"image_tokens":250},"output_tokens_details":{"image_tokens":300}}}}}`), "response.completed", nil)
	require.Equal(t, 250, parsed.ImageInputTokens)
	require.Equal(t, 300, parsed.ImageOutputTokens)
}
