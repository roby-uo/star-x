//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBatchAPIKeyUsageStatsUsesRealTokensAndIndependentDateRanges(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureEntQueryMatcher{actual: &query}))
	require.NoError(t, err)
	defer db.Close()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	mock.ExpectQuery("SELECT").WithArgs(sqlmock.AnyArg(), start, end, sqlmock.AnyArg()).WillReturnRows(
		sqlmock.NewRows([]string{"api_key_id", "total_cost", "today_cost", "total_tokens", "today_tokens", "total_requests", "today_requests"}).
			AddRow(int64(7), 99.5, 10.25, int64(1200), int64(42), int64(12), int64(2)),
	)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	stats, err := repo.GetBatchAPIKeyUsageStats(context.Background(), []int64{7, 8, 7, -1}, start, end)
	require.NoError(t, err)
	require.Len(t, stats, 2)
	require.Equal(t, int64(1200), stats[7].TotalTokens)
	require.Equal(t, int64(42), stats[7].TodayTokens)
	require.Equal(t, int64(12), stats[7].TotalRequests)
	require.Equal(t, int64(2), stats[7].TodayRequests)
	require.Equal(t, int64(0), stats[8].TotalTokens)
	require.Equal(t, 99.5, stats[7].TotalActualCost, "admin financial values stay independent from measured tokens")
	require.Contains(t, query, "input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens")
	require.Contains(t, strings.Join(strings.Fields(query), " "), "WHERE created_at >= $2 AND created_at < $3")
	require.Contains(t, strings.Join(strings.Fields(query), " "), "WHERE created_at >= $4 AND created_at < $3")
	require.NoError(t, mock.ExpectationsWereMet())
}
