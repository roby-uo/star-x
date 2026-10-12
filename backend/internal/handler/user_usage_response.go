package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// User-facing responses deliberately whitelist measured usage. Finance fields
// remain on service/repository models for administrator reporting and billing.
type userUsageStat struct {
	TotalRequests            int64              `json:"total_requests"`
	TotalInputTokens         int64              `json:"total_input_tokens"`
	TotalOutputTokens        int64              `json:"total_output_tokens"`
	TotalCacheTokens         int64              `json:"total_cache_tokens"`
	TotalCacheCreationTokens int64              `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64              `json:"total_cache_read_tokens"`
	TotalTokens              int64              `json:"total_tokens"`
	AverageDurationMs        float64            `json:"average_duration_ms"`
	Endpoints                []userEndpointStat `json:"endpoints,omitempty"`
}

type userEndpointStat struct {
	Endpoint    string `json:"endpoint"`
	Requests    int64  `json:"requests"`
	TotalTokens int64  `json:"total_tokens"`
}

func userUsageStatsFromUsageStats(stats *usagestats.UsageStats) *userUsageStat {
	if stats == nil {
		return nil
	}
	out := &userUsageStat{
		TotalRequests: stats.TotalRequests, TotalInputTokens: stats.TotalInputTokens,
		TotalOutputTokens: stats.TotalOutputTokens, TotalCacheTokens: stats.TotalCacheTokens,
		TotalCacheCreationTokens: stats.TotalCacheCreationTokens, TotalCacheReadTokens: stats.TotalCacheReadTokens,
		TotalTokens: stats.TotalTokens, AverageDurationMs: stats.AverageDurationMs,
	}
	for _, endpoint := range stats.Endpoints {
		out.Endpoints = append(out.Endpoints, userEndpointStat{endpoint.Endpoint, endpoint.Requests, endpoint.TotalTokens})
	}
	return out
}

type userTrendPoint struct {
	Date                string `json:"date"`
	Requests            int64  `json:"requests"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CacheReadTokens     int64  `json:"cache_read_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
}

func userTrendFromUsageTrend(trend []usagestats.TrendDataPoint) []userTrendPoint {
	out := make([]userTrendPoint, 0, len(trend))
	for _, row := range trend {
		out = append(out, userTrendPoint{row.Date, row.Requests, row.InputTokens, row.OutputTokens, row.CacheCreationTokens, row.CacheReadTokens, row.TotalTokens})
	}
	return out
}

type userDashboardStat struct {
	TotalAPIKeys             int64              `json:"total_api_keys"`
	ActiveAPIKeys            int64              `json:"active_api_keys"`
	TotalRequests            int64              `json:"total_requests"`
	TotalInputTokens         int64              `json:"total_input_tokens"`
	TotalOutputTokens        int64              `json:"total_output_tokens"`
	TotalCacheCreationTokens int64              `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64              `json:"total_cache_read_tokens"`
	TotalTokens              int64              `json:"total_tokens"`
	TodayRequests            int64              `json:"today_requests"`
	TodayInputTokens         int64              `json:"today_input_tokens"`
	TodayOutputTokens        int64              `json:"today_output_tokens"`
	TodayCacheCreationTokens int64              `json:"today_cache_creation_tokens"`
	TodayCacheReadTokens     int64              `json:"today_cache_read_tokens"`
	TodayTokens              int64              `json:"today_tokens"`
	AverageDurationMs        float64            `json:"average_duration_ms"`
	Rpm                      int64              `json:"rpm"`
	Tpm                      int64              `json:"tpm"`
	ByPlatform               []userPlatformStat `json:"by_platform,omitempty"`
}

type userPlatformStat struct {
	Platform      string `json:"platform"`
	TotalRequests int64  `json:"total_requests"`
	TotalTokens   int64  `json:"total_tokens"`
	TodayRequests int64  `json:"today_requests"`
	TodayTokens   int64  `json:"today_tokens"`
}

func userDashboardFromUsageDashboard(stats *usagestats.UserDashboardStats) *userDashboardStat {
	if stats == nil {
		return nil
	}
	out := &userDashboardStat{
		TotalAPIKeys: stats.TotalAPIKeys, ActiveAPIKeys: stats.ActiveAPIKeys,
		TotalRequests: stats.TotalRequests, TotalInputTokens: stats.TotalInputTokens,
		TotalOutputTokens: stats.TotalOutputTokens, TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens: stats.TotalCacheReadTokens, TotalTokens: stats.TotalTokens,
		TodayRequests: stats.TodayRequests, TodayInputTokens: stats.TodayInputTokens,
		TodayOutputTokens: stats.TodayOutputTokens, TodayCacheCreationTokens: stats.TodayCacheCreationTokens,
		TodayCacheReadTokens: stats.TodayCacheReadTokens, TodayTokens: stats.TodayTokens,
		AverageDurationMs: stats.AverageDurationMs, Rpm: stats.Rpm, Tpm: stats.Tpm,
	}
	for _, row := range stats.ByPlatform {
		out.ByPlatform = append(out.ByPlatform, userPlatformStat{row.Platform, row.TotalRequests, row.TotalTokens, row.TodayRequests, row.TodayTokens})
	}
	return out
}

type userAPIKeyUsageStat struct {
	APIKeyID      int64 `json:"api_key_id"`
	TodayTokens   int64 `json:"today_tokens"`
	TotalTokens   int64 `json:"total_tokens"`
	TodayRequests int64 `json:"today_requests"`
	TotalRequests int64 `json:"total_requests"`
}

func userAPIKeyStatsFromUsageStats(stats map[int64]*usagestats.BatchAPIKeyUsageStats) map[int64]userAPIKeyUsageStat {
	out := make(map[int64]userAPIKeyUsageStat, len(stats))
	for id, stat := range stats {
		if stat != nil {
			out[id] = userAPIKeyUsageStat{stat.APIKeyID, stat.TodayTokens, stat.TotalTokens, stat.TodayRequests, stat.TotalRequests}
		}
	}
	return out
}

type userAPIKeyDailyPoint struct {
	Date             string `json:"date"`
	Requests         int64  `json:"requests"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

func userDailyUsageFromUsage(points []usagestats.APIKeyDailyUsagePoint) []userAPIKeyDailyPoint {
	out := make([]userAPIKeyDailyPoint, 0, len(points))
	for _, row := range points {
		out = append(out, userAPIKeyDailyPoint{row.Date, row.Requests, row.InputTokens, row.OutputTokens, row.CacheReadTokens, row.CacheWriteTokens, row.TotalTokens})
	}
	return out
}
