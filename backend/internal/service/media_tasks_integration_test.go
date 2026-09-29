//go:build integration

package service

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Uses an explicitly supplied disposable PostgreSQL database. Never connects to
// production or the application's configured database.
func TestMediaTaskReservationIntegration(t *testing.T) {
	dsn := os.Getenv("MEDIA_TEST_DSN")
	if dsn == "" {
		t.Skip("MEDIA_TEST_DSN not provided")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "media_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	db, err := sql.Open("postgres", parsed.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(ctx, `CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,10),frozen_balance NUMERIC(20,10) DEFAULT 0,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);CREATE TABLE api_keys(id BIGINT PRIMARY KEY,quota NUMERIC(20,10),quota_used NUMERIC(20,10),deleted_at TIMESTAMPTZ);INSERT INTO users(id,balance) VALUES(1,1);INSERT INTO api_keys VALUES(1,10,0,NULL);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/185_media_tasks.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	svc := &MediaTaskService{db: db, gateway: &OpenAIGatewayService{}}
	gid := int64(1)
	key := &APIKey{ID: 1, UserID: 1, GroupID: &gid, Group: &Group{ID: 1}}
	input := minimax.CreateVideoRequest{Model: "MiniMax-H3", Resolution: "768P", Duration: 5, Ratio: "16:9", Content: []minimax.VideoContent{{Type: "text", Text: "sunrise"}}}
	quote := &VideoQuote{UnitPrice: .08, Multiplier: 1.4, Total: .56, Currency: "USD"}
	var wg sync.WaitGroup
	tasks := make(chan *MediaTask, 2)
	freshCount := make(chan bool, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, fresh, e := svc.Begin(ctx, key, 10, input, quote, "same-request", nil)
			tasks <- task
			freshCount <- fresh
			failures <- e
		}()
	}
	wg.Wait()
	close(tasks)
	close(freshCount)
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	freshTotal := 0
	for fresh := range freshCount {
		if fresh {
			freshTotal++
		}
	}
	require.Equal(t, 1, freshTotal)
	var id string
	for task := range tasks {
		if id != "" {
			require.Equal(t, id, task.ID)
		}
		id = task.ID
	}
	var balance, frozen float64
	require.NoError(t, db.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=1`).Scan(&balance, &frozen))
	require.InDelta(t, .44, balance, 1e-9)
	require.InDelta(t, .56, frozen, 1e-9)
	_, _, err = svc.Begin(ctx, key, 10, input, quote, "second-request", nil)
	require.Error(t, err, "must reject aggregate spend beyond available balance")
	changed := input
	changed.Duration = 6
	_, _, err = svc.Begin(ctx, key, 10, changed, quote, "same-request", nil)
	require.Error(t, err, "must reject a reused idempotency key with changed parameters")
	_, err = svc.Get(ctx, id, 2)
	require.ErrorIs(t, err, sql.ErrNoRows, "another user must not see task")
	require.NoError(t, svc.Uncertain(ctx, id))
	require.NoError(t, svc.Reject(ctx, id))
	require.NoError(t, svc.Reject(ctx, id))
	require.NoError(t, db.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=1`).Scan(&balance, &frozen))
	require.InDelta(t, 1, balance, 1e-9)
	require.Zero(t, frozen)
}
