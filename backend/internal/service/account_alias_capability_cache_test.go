//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

func aliasCapabilityAccount(id int64, mappings map[string]any) Account {
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": mappings}}
}

func TestAccountAliasCapabilitiesReuseModelDiscoveryAndDoNotReadAccountsOnLookup(t *testing.T) {
	groupID := int64(10)
	channels := &ChannelService{}
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {
		aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2", "my-video": "MiniMax-H3", "my-reasoner": "qwen3-vl"}),
	}}}
	gateway := &GatewayService{accountRepo: repo, channelService: channels, modelsListCache: gocache.New(time.Minute, time.Minute), modelsListCacheTTL: time.Minute}
	models := gateway.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI)
	require.ElementsMatch(t, []string{"my-image", "my-video", "my-reasoner"}, models)
	for alias, kind := range map[string]string{"my-image": "image", "my-video": "video", "my-reasoner": "unknown"} {
		for range 20 {
			capability, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, alias)
			require.True(t, found)
			require.Equal(t, kind, capability.ModelType)
			if kind != "unknown" {
				require.NotEmpty(t, capability.SupportedEndpoints)
			}
		}
	}
	require.Equal(t, models, gateway.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
	require.Equal(t, int64(1), repo.listByGroupCalls.Load())

	capability, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.True(t, found)
	capability.SupportedEndpoints[0] = "/modified"
	capability, found = channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.True(t, found)
	require.Equal(t, []string{"/v1/images/generations"}, capability.SupportedEndpoints)
}

func TestAccountAliasCapabilitiesModelDiscoveryFeedsBothGatewayResolvers(t *testing.T) {
	groupID := int64(10)
	channels := &ChannelService{}
	channels.cache.Store(populateChannelCache(nil, map[int64]string{groupID: PlatformOpenAI}))
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {
		aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2", "my-video": "MiniMax-H3"}),
	}}}
	gateway := &GatewayService{accountRepo: repo, channelService: channels, modelsListCache: gocache.New(time.Minute, time.Minute), modelsListCacheTTL: time.Minute}
	openAI := &OpenAIGatewayService{accountRepo: repo, channelService: channels}
	resolvers := map[string]func(context.Context, *int64, string, string) (ModelEndpointCapability, error){
		"gateway": gateway.ResolveModelEndpointCapability,
		"openai":  openAI.ResolveModelEndpointCapability,
	}
	expected := map[string]ModelEndpointCapability{
		"my-image": {ModelType: "image", SupportedEndpoints: []string{"/v1/images/generations"}},
		"my-video": {ModelType: "video", SupportedEndpoints: []string{"/v2/video_generation"}},
	}
	require.ElementsMatch(t, []string{"my-image", "my-video"}, gateway.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
	for name, resolve := range resolvers {
		t.Run(name, func(t *testing.T) {
			for alias, want := range expected {
				capability, err := resolve(context.Background(), &groupID, PlatformOpenAI, alias)
				require.NoError(t, err)
				require.Equal(t, want, capability)
			}
		})
	}
	require.Equal(t, int64(1), repo.listByGroupCalls.Load())
	require.Zero(t, repo.listAllCalls.Load())

	gateway.InvalidateAvailableModelsCache(&groupID, PlatformOpenAI)
	for _, resolve := range resolvers {
		for alias := range expected {
			capability, err := resolve(context.Background(), &groupID, PlatformOpenAI, alias)
			require.NoError(t, err)
			require.Equal(t, "unknown", capability.ModelType)
			require.False(t, capability.IsGenerationModel())
		}
	}
	require.Equal(t, int64(1), repo.listByGroupCalls.Load())
	require.Zero(t, repo.listAllCalls.Load())
}

func TestAccountAliasCapabilitiesKeepMixedRoutesCallable(t *testing.T) {
	groupID := int64(10)
	for name, second := range map[string]Account{
		"explicit-language-route": aliasCapabilityAccount(2, map[string]any{"my-image": "gpt-5.6"}),
		"wildcard-language-route": aliasCapabilityAccount(2, map[string]any{"my-*": "gpt-5.6"}),
		"unrestricted-route":      aliasCapabilityAccount(2, nil),
	} {
		t.Run(name, func(t *testing.T) {
			channels := &ChannelService{}
			channels.cacheAccountAliasCapabilities(&groupID, PlatformOpenAI, []Account{
				aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2"}), second,
			}, time.Minute)
			capability, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
			require.True(t, found)
			require.Equal(t, "unknown", capability.ModelType)
			require.False(t, capability.IsGenerationModel())
			require.Empty(t, capability.SupportedEndpoints)
		})
	}
}

func TestAccountAliasCapabilitiesRespectAccountMappingCase(t *testing.T) {
	groupID := int64(10)
	channels := &ChannelService{}
	channels.cacheAccountAliasCapabilities(&groupID, PlatformOpenAI, []Account{
		aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2"}),
		aliasCapabilityAccount(2, map[string]any{"MY-IMAGE": "gpt-5.6"}),
	}, time.Minute)
	capability, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.True(t, found)
	require.Equal(t, "image", capability.ModelType)
	capability, found = channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "MY-IMAGE")
	require.True(t, found)
	require.Equal(t, "unknown", capability.ModelType)
}

func TestAccountAliasCapabilitiesExpireAndInvalidateByGroupAndPlatform(t *testing.T) {
	groupID, otherGroupID := int64(10), int64(20)
	channels := &ChannelService{}
	accounts := []Account{aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2"})}
	channels.cacheAccountAliasCapabilities(&groupID, PlatformOpenAI, accounts, time.Minute)
	channels.cacheAccountAliasCapabilities(&otherGroupID, PlatformOpenAI, accounts, time.Minute)
	channels.cacheAccountAliasCapabilities(&groupID, PlatformAnthropic, accounts, time.Minute)
	gateway := &GatewayService{channelService: channels}
	gateway.InvalidateAvailableModelsCache(&groupID, PlatformOpenAI)
	_, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.False(t, found)
	_, found = channels.cachedAccountAliasCapability(&otherGroupID, PlatformOpenAI, "my-image")
	require.True(t, found)
	_, found = channels.cachedAccountAliasCapability(&groupID, PlatformAnthropic, "my-image")
	require.True(t, found)

	key := channelGroupPlatformKey{groupID: otherGroupID, platform: PlatformOpenAI}
	entry := channels.accountAliasCapabilities.entries[key]
	entry.expiresAt = time.Now().Add(-time.Second)
	channels.accountAliasCapabilities.entries[key] = entry
	_, found = channels.cachedAccountAliasCapability(&otherGroupID, PlatformOpenAI, "my-image")
	require.False(t, found)
	gateway.InvalidateAvailableModelsCache(nil, "")
	_, found = channels.cachedAccountAliasCapability(&groupID, PlatformAnthropic, "my-image")
	require.False(t, found)
}

func TestAccountAliasCapabilitiesAreClearedAfterDiscoveryFailure(t *testing.T) {
	groupID := int64(10)
	channels := &ChannelService{}
	channels.cacheAccountAliasCapabilities(&groupID, PlatformOpenAI, []Account{
		aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2"}),
	}, time.Minute)
	gateway := &GatewayService{channelService: channels, accountRepo: &modelsListAccountRepoStub{err: errors.New("account query unavailable")}}
	require.Nil(t, gateway.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI))
	_, found := channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.False(t, found)
}

func TestGlobalDiscoveryFailureDoesNotRemoveGroupAliasCapabilities(t *testing.T) {
	groupID := int64(10)
	channels := &ChannelService{}
	accounts := []Account{aliasCapabilityAccount(1, map[string]any{"my-image": "gpt-image-2"})}
	channels.cacheAccountAliasCapabilities(&groupID, PlatformOpenAI, accounts, time.Minute)
	channels.cacheAccountAliasCapabilities(nil, PlatformOpenAI, accounts, time.Minute)
	gateway := &GatewayService{channelService: channels, accountRepo: &modelsListAccountRepoStub{err: errors.New("global account query unavailable")}}
	require.Nil(t, gateway.GetAvailableModels(context.Background(), nil, PlatformOpenAI))
	_, found := channels.cachedAccountAliasCapability(nil, PlatformOpenAI, "my-image")
	require.False(t, found)
	_, found = channels.cachedAccountAliasCapability(&groupID, PlatformOpenAI, "my-image")
	require.True(t, found)
}
