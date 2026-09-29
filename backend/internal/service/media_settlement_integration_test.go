//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMediaTaskSettlementIntegration(t *testing.T) {
	dsn := os.Getenv("MEDIA_TEST_DSN")
	if dsn == "" {
		t.Skip("MEDIA_TEST_DSN not provided")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "media_settlement_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("postgres", parsed.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(ctx, `CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,10),frozen_balance NUMERIC(20,10) DEFAULT 0,deleted_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
 CREATE TABLE api_keys(id BIGINT PRIMARY KEY,quota NUMERIC(20,10),quota_used NUMERIC(20,10),deleted_at TIMESTAMPTZ);
 CREATE TABLE usage_billing_dedup(id BIGSERIAL PRIMARY KEY,request_id TEXT,api_key_id BIGINT,request_fingerprint TEXT,UNIQUE(request_id,api_key_id));
 CREATE TABLE usage_billing_dedup_archive(request_id TEXT,api_key_id BIGINT,request_fingerprint TEXT);
 INSERT INTO users(id,balance) VALUES(1,1);INSERT INTO api_keys VALUES(1,10,0,NULL);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/185_media_tasks.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	tasks := service.NewMediaTaskService(db, nil, nil, &service.OpenAIGatewayService{})
	defer tasks.Stop()
	gid := int64(1)
	key := &service.APIKey{ID: 1, UserID: 1, GroupID: &gid, Group: &service.Group{ID: 1}}
	input := minimax.CreateVideoRequest{Model: "MiniMax-H3", Resolution: "768P", Duration: 5}
	quote := &service.VideoQuote{UnitPrice: .08, Multiplier: 1.4, Total: .56, Currency: "USD"}
	task, fresh, err := tasks.Begin(ctx, key, 10, input, quote, "settlement", nil)
	require.NoError(t, err)
	require.True(t, fresh)
	require.NoError(t, tasks.Accepted(ctx, task.ID, "upstream-id"))
	billing := repository.NewUsageBillingRepository(nil, db)
	command := &service.UsageBillingCommand{RequestID: "upstream-id", APIKeyID: 1, UserID: 1, AccountID: 10, BalanceCost: .56, MediaTaskID: task.ID}
	result, err := billing.Apply(ctx, command)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, command)
	require.NoError(t, err)
	require.False(t, result.Applied)
	var balance, frozen float64
	require.NoError(t, db.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=1`).Scan(&balance, &frozen))
	require.InDelta(t, .44, balance, 1e-9)
	require.Zero(t, frozen)
	current, err := tasks.Get(ctx, task.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "charged", current.BillingState)
	require.Error(t, tasks.Reject(ctx, task.ID), "accepted tasks must not use release path")
	_, err = db.ExecContext(ctx, `UPDATE media_tasks SET state='failed' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	require.NoError(t, tasks.Refund(ctx, task.ID))
	require.NoError(t, tasks.Refund(ctx, task.ID))
	require.NoError(t, db.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=1`).Scan(&balance, &frozen))
	require.InDelta(t, 1, balance, 1e-9)
	require.Zero(t, frozen)
	result, err = billing.Apply(ctx, command)
	require.NoError(t, err)
	require.False(t, result.Applied, "late billing retry must not charge a refunded task")
}
