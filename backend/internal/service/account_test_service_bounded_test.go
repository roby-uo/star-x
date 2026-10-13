//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newBoundedAccountTestService(account *Account, responses ...*http.Response) (*AccountTestService, *queuedHTTPUpstream) {
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{account.ID: account},
		},
	}
	upstream := &queuedHTTPUpstream{responses: responses}
	return &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}, upstream
}

func TestAccountTestService_BoundedResponsesUsesRequestedPromptAndLimit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		accountType string
		limit       int
	}{
		{name: "oauth shortest", accountType: AccountTypeOAuth, limit: 1},
		{name: "oauth authorized eight tokens", accountType: AccountTypeOAuth, limit: 8},
		{name: "apikey authorized eight tokens", accountType: AccountTypeAPIKey, limit: 8},
		{name: "apikey largest bounded test", accountType: AccountTypeAPIKey, limit: 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{
				ID:          10,
				Platform:    PlatformOpenAI,
				Type:        tc.accountType,
				Concurrency: 1,
				Credentials: map[string]any{"access_token": "test-token", "api_key": "sk-test", "base_url": "https://compat-upstream.example/v1"},
				Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: true},
			}
			response := newJSONResponse(http.StatusOK, "data: {\"type\":\"response.completed\"}\n\n")
			svc, upstream := newBoundedAccountTestService(account, response)
			ctx, recorder := newTestContext()
			err := svc.TestAccountConnectionWithOptions(ctx, account.ID, "gpt-6.1-sol", " Reply OK only. ", "", AccountConnectionTestOptions{MaxOutputTokens: tc.limit})
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			body, err := io.ReadAll(upstream.requests[0].Body)
			require.NoError(t, err)
			require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(body, "model").String())
			require.Equal(t, int64(tc.limit), gjson.GetBytes(body, "max_output_tokens").Int())
			require.Equal(t, "Reply OK only.", gjson.GetBytes(body, "input.0.content.0.text").String())
			require.Equal(t, "Reply briefly.", gjson.GetBytes(body, "instructions").String())
			require.True(t, gjson.GetBytes(body, "stream").Bool())
			if tc.accountType == AccountTypeOAuth {
				require.True(t, gjson.GetBytes(body, "store").Exists())
				require.False(t, gjson.GetBytes(body, "store").Bool())
			}
			require.Contains(t, recorder.Body.String(), `"success":true`)
		})
	}
}

func TestAccountTestService_BoundedResponsesEmptyPromptHasShortDefault(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	svc, upstream := newBoundedAccountTestService(account, newJSONResponse(http.StatusOK, "data: {\"type\":\"response.completed\"}\n\n"))
	ctx, _ := newTestContext()
	require.NoError(t, svc.TestAccountConnectionWithOptions(ctx, account.ID, "gpt-6.1-sol", "  ", "", AccountConnectionTestOptions{MaxOutputTokens: 8}))
	require.Len(t, upstream.requests, 1)
	body, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.Equal(t, "Reply OK only.", gjson.GetBytes(body, "input.0.content.0.text").String())
	require.Equal(t, int64(8), gjson.GetBytes(body, "max_output_tokens").Int())
}

func TestAccountTestService_BoundedRejectsInvalidLimitBeforeAccountLookup(t *testing.T) {
	for _, limit := range []int{-1, 65} {
		ctx, recorder := newTestContext()
		svc := &AccountTestService{}
		err := svc.TestAccountConnectionWithOptions(ctx, 10, "gpt-6.1-sol", "OK", "", AccountConnectionTestOptions{MaxOutputTokens: limit})
		require.Error(t, err)
		require.Contains(t, recorder.Body.String(), "between 1 and 64")
	}
}

func TestAccountTestService_BoundedRejectsUnsupportedRoutesWithoutRequests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		model    string
		mode     string
		apiKey   bool
	}{
		{name: "other platform", platform: PlatformGemini, model: "gemini-3.5-flash"},
		{name: "compact", platform: PlatformOpenAI, model: "gpt-6.1-sol", mode: AccountTestModeCompact},
		{name: "image", platform: PlatformOpenAI, model: "gpt-image-2"},
		{name: "video", platform: PlatformOpenAI, model: "MiniMax-H3"},
		{name: "chat completions", platform: PlatformOpenAI, model: "gpt-6.1-sol", apiKey: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{ID: 10, Platform: tc.platform, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
			if tc.apiKey {
				account.Type = AccountTypeAPIKey
				account.Credentials = map[string]any{"api_key": "sk-test", "base_url": "https://compat-upstream.example/v1"}
				account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: false}
			}
			svc, upstream := newBoundedAccountTestService(account)
			ctx, _ := newTestContext()
			err := svc.TestAccountConnectionWithOptions(ctx, account.ID, tc.model, "OK", tc.mode, AccountConnectionTestOptions{MaxOutputTokens: 8})
			require.Error(t, err)
			require.Contains(t, err.Error(), "Bounded account tests")
			require.Empty(t, upstream.requests)
		})
	}
}

func TestAccountTestService_BoundedUpstreamRejectionNeverRetriesWithoutLimit(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	svc, upstream := newBoundedAccountTestService(account, newJSONResponse(http.StatusBadRequest, `{"error":{"message":"Unsupported parameter: max_output_tokens"}}`))
	ctx, recorder := newTestContext()
	err := svc.TestAccountConnectionWithOptions(ctx, account.ID, "gpt-6.1-sol", "Reply OK only.", "", AccountConnectionTestOptions{MaxOutputTokens: 8})
	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	body, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.Equal(t, int64(8), gjson.GetBytes(body, "max_output_tokens").Int())
	require.Contains(t, recorder.Body.String(), "Unsupported parameter: max_output_tokens")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_DefaultConnectionKeepsExistingPayload(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	svc, upstream := newBoundedAccountTestService(account, newJSONResponse(http.StatusOK, "data: {\"type\":\"response.completed\"}\n\n"))
	ctx, _ := newTestContext()
	require.NoError(t, svc.TestAccountConnection(ctx, account.ID, "gpt-5.4", "unused default prompt", ""))
	require.Len(t, upstream.requests, 1)
	body, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(body, "max_output_tokens").Exists())
	require.Equal(t, "hi", gjson.GetBytes(body, "input.0.content.0.text").String())
	require.Greater(t, len(strings.TrimSpace(gjson.GetBytes(body, "instructions").String())), len("Reply briefly."))
}

func TestAccountTestService_BoundedReportsActualUpstreamModelAndUsage(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	stream := "data: {\"type\":\"response.created\",\"response\":{\"model\":\"actual-upstream-model\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"actual-upstream-model\",\"status\":\"completed\",\"usage\":{\"input_tokens\":4,\"output_tokens\":2,\"total_tokens\":6}}}\n\n"
	svc, upstream := newBoundedAccountTestService(account, newJSONResponse(http.StatusOK, stream))
	ctx, recorder := newTestContext()
	require.NoError(t, svc.TestAccountConnectionWithOptions(ctx, account.ID, "gpt-6.1-sol", "Reply OK only.", "", AccountConnectionTestOptions{MaxOutputTokens: 8}))
	require.Len(t, upstream.requests, 1)
	events := recorder.Body.String()
	require.Contains(t, events, `"type":"test_start","model":"gpt-6.1-sol"`)
	require.Contains(t, events, `"type":"test_complete","model":"actual-upstream-model","status":"completed"`)
	require.Contains(t, events, `"upstream_model":"actual-upstream-model"`)
	require.Contains(t, events, `"output_tokens":2`)
	require.Contains(t, events, `"input_tokens":4`)
	require.Contains(t, events, `"total_tokens":6`)
}

func TestAccountTestService_BoundedLimitReachedIsReportedWithoutRetry(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	stream := "data: {\"type\":\"response.incomplete\",\"response\":{\"model\":\"gpt-6.1-sol\",\"status\":\"incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"usage\":{\"input_tokens\":4,\"output_tokens\":8,\"total_tokens\":12}}}"
	svc, upstream := newBoundedAccountTestService(account, newJSONResponse(http.StatusOK, stream))
	ctx, recorder := newTestContext()
	err := svc.TestAccountConnectionWithOptions(ctx, account.ID, "gpt-6.1-sol", "Reply OK only.", "", AccountConnectionTestOptions{MaxOutputTokens: 8})
	require.ErrorContains(t, err, "reached max_output_tokens")
	require.Len(t, upstream.requests, 1)
	events := recorder.Body.String()
	require.Contains(t, events, `"model":"gpt-6.1-sol","status":"limit_reached"`)
	require.Contains(t, events, `"output_tokens":8`)
	require.Contains(t, events, `"incomplete_reason":"max_output_tokens"`)
	require.NotContains(t, events, `"success":true`)
	require.NotContains(t, events, "unsupported model")
}

func TestAccountTestService_BoundedRejectsReportedOutputOverLimit(t *testing.T) {
	svc := &AccountTestService{}
	ctx, recorder := newTestContext()
	stream := `data: {"type":"response.completed","response":{"model":"gpt-6.1-sol","usage":{"output_tokens":9}}}`
	err := svc.processBoundedOpenAIStream(ctx, strings.NewReader(stream), 8)
	require.ErrorContains(t, err, "more output tokens")
	require.Contains(t, recorder.Body.String(), `"status":"limit_exceeded"`)
	require.Contains(t, recorder.Body.String(), `"output_tokens":9`)
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_BoundedEOFDoesNotInventUpstreamModel(t *testing.T) {
	svc := &AccountTestService{}
	ctx, recorder := newTestContext()
	err := svc.processBoundedOpenAIStream(ctx, strings.NewReader(`data: {"type":"response.output_text.delta","delta":"OK"}`), 8)
	require.ErrorContains(t, err, "ended before")
	require.Contains(t, recorder.Body.String(), `"status":"stream_ended"`)
	require.NotContains(t, recorder.Body.String(), "upstream_model")
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestAccountTestService_BoundedDoneWithFailedStatusIsNotSuccess(t *testing.T) {
	svc := &AccountTestService{}
	ctx, recorder := newTestContext()
	stream := `data: {"type":"response.done","response":{"model":"gpt-6.1-sol","status":"failed","usage":{"output_tokens":0}}}`
	err := svc.processBoundedOpenAIStream(ctx, strings.NewReader(stream), 8)
	require.ErrorContains(t, err, "non-completed response: failed")
	require.Contains(t, recorder.Body.String(), `"status":"failed"`)
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}
