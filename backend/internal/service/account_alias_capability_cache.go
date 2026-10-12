package service

import (
	"strings"
	"sync"
	"time"
)

// Account aliases are learned from the existing model-discovery account query.
// Both gateway services share this cache through ChannelService; text requests
// never query accounts merely to classify an unknown custom model name.
type accountAliasCapabilityCache struct {
	mu      sync.RWMutex
	entries map[channelGroupPlatformKey]accountAliasCapabilitySnapshot
}

type accountAliasCapabilitySnapshot struct {
	expiresAt    time.Time
	capabilities map[string]ModelEndpointCapability
}

func (s *ChannelService) cacheAccountAliasCapabilities(groupID *int64, platform string, accounts []Account, ttl time.Duration) {
	if s == nil {
		return
	}
	if ttl <= 0 {
		ttl = defaultModelsListCacheTTL
	}
	aliases := make(map[string]string)
	for i := range accounts {
		for alias := range accounts[i].GetModelMapping() {
			// Wildcard-only discovery entries are not literal model IDs. Concrete
			// aliases below still consider matching wildcard routes on every account.
			if !strings.Contains(alias, "*") {
				aliases[strings.TrimSpace(alias)] = alias
			}
		}
	}
	capabilities := make(map[string]ModelEndpointCapability, len(aliases))
	for key, alias := range aliases {
		var capability ModelEndpointCapability
		seen := false
		for i := range accounts {
			account := &accounts[i]
			if !account.IsModelSupported(alias) {
				continue
			}
			next := inferModelEndpointCapability(account.GetMappedModel(alias), account.Platform)
			if !seen {
				capability, seen = next, true
				continue
			}
			if capability.ModelType != next.ModelType {
				capability = ModelEndpointCapability{ModelType: "unknown", SupportedEndpoints: []string{}}
				break
			}
			capability.SupportedEndpoints = commonModelEndpoints(capability.SupportedEndpoints, next.SupportedEndpoints)
		}
		if seen {
			capabilities[key] = capability
		}
	}
	now := time.Now()
	s.accountAliasCapabilities.mu.Lock()
	defer s.accountAliasCapabilities.mu.Unlock()
	if s.accountAliasCapabilities.entries == nil {
		s.accountAliasCapabilities.entries = make(map[channelGroupPlatformKey]accountAliasCapabilitySnapshot)
	}
	for key, entry := range s.accountAliasCapabilities.entries {
		if !now.Before(entry.expiresAt) {
			delete(s.accountAliasCapabilities.entries, key)
		}
	}
	key := channelGroupPlatformKey{groupID: derefGroupID(groupID), platform: strings.TrimSpace(platform)}
	s.accountAliasCapabilities.entries[key] = accountAliasCapabilitySnapshot{expiresAt: now.Add(ttl), capabilities: capabilities}
}

func commonModelEndpoints(left, right []string) []string {
	common := make([]string, 0, len(left))
	for _, endpoint := range left {
		for _, other := range right {
			if endpoint == other {
				common = append(common, endpoint)
				break
			}
		}
	}
	return common
}

func (s *ChannelService) cachedAccountAliasCapability(groupID *int64, platform, model string) (ModelEndpointCapability, bool) {
	if s == nil {
		return ModelEndpointCapability{}, false
	}
	key := channelGroupPlatformKey{groupID: derefGroupID(groupID), platform: strings.TrimSpace(platform)}
	s.accountAliasCapabilities.mu.RLock()
	defer s.accountAliasCapabilities.mu.RUnlock()
	entry, exists := s.accountAliasCapabilities.entries[key]
	if !exists || !time.Now().Before(entry.expiresAt) {
		return ModelEndpointCapability{}, false
	}
	capability, exists := entry.capabilities[strings.TrimSpace(model)]
	if !exists {
		return ModelEndpointCapability{}, false
	}
	capability.SupportedEndpoints = append([]string{}, capability.SupportedEndpoints...)
	return capability, true
}

func (s *ChannelService) invalidateAccountAliasCapabilities(groupID *int64, platform string) {
	if s == nil {
		return
	}
	platform = strings.TrimSpace(platform)
	s.accountAliasCapabilities.mu.Lock()
	defer s.accountAliasCapabilities.mu.Unlock()
	for key := range s.accountAliasCapabilities.entries {
		if groupID != nil && key.groupID != *groupID {
			continue
		}
		if platform != "" && key.platform != platform {
			continue
		}
		delete(s.accountAliasCapabilities.entries, key)
	}
}

func (s *ChannelService) clearAccountAliasCapabilityScope(groupID *int64, platform string) {
	if s == nil {
		return
	}
	s.accountAliasCapabilities.mu.Lock()
	defer s.accountAliasCapabilities.mu.Unlock()
	delete(s.accountAliasCapabilities.entries, channelGroupPlatformKey{groupID: derefGroupID(groupID), platform: strings.TrimSpace(platform)})
}
