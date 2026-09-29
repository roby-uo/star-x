package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MediaTask is also the user-safe representation. Credentials, request bodies,
// upstream account identifiers and raw provider errors are never returned.
type MediaTask struct {
	ID             string                      `json:"id"`
	UserID         int64                       `json:"user_id"`
	APIKeyID       int64                       `json:"api_key_id"`
	GroupID        int64                       `json:"group_id"`
	AccountID      int64                       `json:"-"`
	Model          string                      `json:"model"`
	RequestHash    string                      `json:"-"`
	UpstreamTaskID string                      `json:"task_id,omitempty"`
	State          string                      `json:"state"`
	BillingState   string                      `json:"billing_state"`
	Quote          VideoQuote                  `json:"quote"`
	Specification  minimax.CreateVideoRequest  `json:"specification"`
	HoldAmount     float64                     `json:"-"`
	SubscriptionID *int64                      `json:"-"`
	Result         *minimax.QueryVideoResponse `json:"result,omitempty"`
	Error          string                      `json:"error,omitempty"`
	CreatedAt      time.Time                   `json:"created_at"`
}

// Refund credits the customer's balance after an administrator reviews a failed
// task. It does not claim that the provider refunded its cost, or reset usage limits.
func (s *MediaTaskService) Refund(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	t, err := scanMediaTask(tx.QueryRowContext(ctx, mediaTaskSelect+` WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if t.BillingState == "refunded" {
		return nil
	}
	if t.BillingState != "charged" || (t.State != "failed" && t.State != "cancelled") {
		return fmt.Errorf("仅可退还已结算的失败或取消任务")
	}
	if t.SubscriptionID != nil {
		return fmt.Errorf("订阅任务请通过订阅额度管理处理补偿")
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, t.Quote.Total, t.UserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return fmt.Errorf("用户不存在，无法退还余额")
	}
	_, err = tx.ExecContext(ctx, `UPDATE media_tasks SET billing_state='refunded',error='已由管理员退还余额；原调用用量仍保留',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if s.gateway.billingCacheService != nil {
		_ = s.gateway.billingCacheService.InvalidateUserBalance(ctx, t.UserID)
	}
	return nil
}

type MediaTaskService struct {
	db       *sql.DB
	accounts AccountRepository
	keys     *APIKeyService
	gateway  *OpenAIGatewayService
	stop     chan struct{}
}

func NewMediaTaskService(db *sql.DB, accounts AccountRepository, keys *APIKeyService, gateway *OpenAIGatewayService) *MediaTaskService {
	s := &MediaTaskService{db: db, accounts: accounts, keys: keys, gateway: gateway, stop: make(chan struct{})}
	go s.run()
	return s
}

func (s *MediaTaskService) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

const mediaTaskSelect = `SELECT id,user_id,api_key_id,group_id,account_id,model,request_hash,COALESCE(upstream_task_id,''),state,billing_state,quote,specification,hold_amount,subscription_id,result,error,created_at FROM media_tasks`

func scanMediaTask(row interface{ Scan(...any) error }) (*MediaTask, error) {
	var t MediaTask
	var quote, spec, result []byte
	err := row.Scan(&t.ID, &t.UserID, &t.APIKeyID, &t.GroupID, &t.AccountID, &t.Model, &t.RequestHash, &t.UpstreamTaskID, &t.State, &t.BillingState, &quote, &spec, &t.HoldAmount, &t.SubscriptionID, &result, &t.Error, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(quote, &t.Quote); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(spec, &t.Specification); err != nil {
		return nil, err
	}
	if len(result) > 0 {
		if err = json.Unmarshal(result, &t.Result); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func (s *MediaTaskService) Get(ctx context.Context, id string, userID int64) (*MediaTask, error) {
	return scanMediaTask(s.db.QueryRowContext(ctx, mediaTaskSelect+` WHERE id=$1 AND ($2::bigint=0 OR user_id=$2)`, id, userID))
}
func (s *MediaTaskService) ByUpstream(ctx context.Context, id string, userID, keyID int64) (*MediaTask, error) {
	return scanMediaTask(s.db.QueryRowContext(ctx, mediaTaskSelect+` WHERE upstream_task_id=$1 AND user_id=$2 AND api_key_id=$3`, id, userID, keyID))
}

func (s *MediaTaskService) ByIdempotency(ctx context.Context, keyID int64, id string) (*MediaTask, error) {
	return scanMediaTask(s.db.QueryRowContext(ctx, mediaTaskSelect+` WHERE api_key_id=$1 AND idempotency_key=$2`, keyID, id))
}
func (s *MediaTaskService) List(ctx context.Context, userID int64, offset int) ([]*MediaTask, error) {
	rows, err := s.db.QueryContext(ctx, mediaTaskSelect+` WHERE ($1::bigint=0 OR user_id=$1) ORDER BY created_at DESC LIMIT 50 OFFSET $2`, userID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*MediaTask, 0)
	for rows.Next() {
		t, e := scanMediaTask(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// Begin reserves balance in the same transaction that claims the idempotency
// key. Nothing is sent upstream unless this transaction commits.
func (s *MediaTaskService) Begin(ctx context.Context, key *APIKey, accountID int64, input minimax.CreateVideoRequest, quote *VideoQuote, idempotency string, subscription *UserSubscription) (*MediaTask, bool, error) {
	raw, _ := json.Marshal(input)
	hash := HashUsageRequestPayload(raw)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize reservations for this user and key, including requests with distinct keys.
	var balance float64
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, key.UserID).Scan(&balance); err != nil {
		return nil, false, err
	}
	existing, e := scanMediaTask(tx.QueryRowContext(ctx, mediaTaskSelect+` WHERE api_key_id=$1 AND idempotency_key=$2`, key.ID, idempotency))
	if e == nil {
		if existing.RequestHash != hash {
			return nil, false, fmt.Errorf("幂等键已用于不同请求")
		}
		return existing, false, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, false, e
	}
	var quota, used float64
	if err = tx.QueryRowContext(ctx, `SELECT quota,quota_used FROM api_keys WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, key.ID).Scan(&quota, &used); err != nil {
		return nil, false, err
	}
	var reserved float64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM((quote->>'total')::numeric),0) FROM media_tasks WHERE api_key_id=$1 AND billing_state='reserved'`, key.ID).Scan(&reserved); err != nil {
		return nil, false, err
	}
	if quota > 0 && used+reserved+quote.Total > quota {
		return nil, false, fmt.Errorf("API 密钥剩余额度不足")
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_tasks WHERE user_id=$1 AND state IN ('submitting','queued','running')`, key.UserID).Scan(&active); err != nil {
		return nil, false, err
	}
	if active >= 10 {
		return nil, false, fmt.Errorf("同时进行的视频任务最多 10 个，请等待已有任务完成")
	}
	hold := quote.Total
	var subID *int64
	if subscription != nil && key.Group != nil && key.Group.IsSubscriptionType() {
		hold = 0
		subID = &subscription.ID
		var daily, weekly, monthly, pending float64
		err = tx.QueryRowContext(ctx, `SELECT daily_usage_usd,weekly_usage_usd,monthly_usage_usd,(SELECT COALESCE(SUM((quote->>'total')::numeric),0) FROM media_tasks WHERE subscription_id=$1 AND billing_state='reserved') FROM user_subscriptions WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, subscription.ID, key.UserID).Scan(&daily, &weekly, &monthly, &pending)
		if err != nil {
			return nil, false, err
		}
		for _, limit := range []struct {
			used float64
			max  *float64
		}{{daily, key.Group.DailyLimitUSD}, {weekly, key.Group.WeeklyLimitUSD}, {monthly, key.Group.MonthlyLimitUSD}} {
			if limit.max != nil && *limit.max > 0 && limit.used+pending+quote.Total > *limit.max {
				return nil, false, fmt.Errorf("订阅剩余额度不足以支付本次视频生成")
			}
		}
	}
	if hold > balance {
		return nil, false, fmt.Errorf("余额不足以支付本次视频生成")
	}
	if hold > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2`, hold, key.UserID); err != nil {
			return nil, false, err
		}
	}
	// Retain only specifications, not potentially large or sensitive input images.
	spec := input
	spec.Content = nil
	specJSON, _ := json.Marshal(spec)
	quoteJSON, _ := json.Marshal(quote)
	id := "vid_" + uuid.NewString()
	_, err = tx.ExecContext(ctx, `INSERT INTO media_tasks(id,user_id,api_key_id,group_id,account_id,model,idempotency_key,request_hash,quote,specification,hold_amount,subscription_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, key.UserID, key.ID, *key.GroupID, accountID, input.Model, idempotency, hash, quoteJSON, specJSON, hold, subID)
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	if s.gateway.billingCacheService != nil {
		_ = s.gateway.billingCacheService.InvalidateUserBalance(ctx, key.UserID)
	}
	t, err := s.Get(ctx, id, key.UserID)
	return t, true, err
}

func (s *MediaTaskService) Accepted(ctx context.Context, id, upstreamID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE media_tasks SET upstream_task_id=$2,state='queued',error='',updated_at=NOW() WHERE id=$1 AND state IN ('submitting','uncertain') AND billing_state='reserved' AND upstream_task_id IS NULL`, id, upstreamID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return fmt.Errorf("任务受理状态保存失败，需要人工核对")
	}
	return nil
}

func (s *MediaTaskService) Uncertain(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE media_tasks SET state='uncertain',error='上游是否受理尚未确认，请联系管理员核对；不要重复提交',updated_at=NOW() WHERE id=$1 AND state='submitting'`, id)
	return err
}

func (s *MediaTaskService) Reject(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	t, err := scanMediaTask(tx.QueryRowContext(ctx, mediaTaskSelect+` WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if t.BillingState == "released" {
		return nil
	}
	if t.BillingState != "reserved" {
		return fmt.Errorf("已结算任务不能释放预占额度")
	}
	if t.UpstreamTaskID != "" || (t.State != "submitting" && t.State != "uncertain") {
		return fmt.Errorf("已受理的任务不能直接释放预占额度")
	}
	if t.HoldAmount > 0 {
		result, releaseErr := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=frozen_balance-$1,updated_at=NOW() WHERE id=$2 AND frozen_balance >= $1`, t.HoldAmount, t.UserID)
		if releaseErr != nil {
			return releaseErr
		}
		affected, e := result.RowsAffected()
		if e != nil || affected != 1 {
			return fmt.Errorf("预占余额不一致，需人工核对")
		}

	}
	_, err = tx.ExecContext(ctx, `UPDATE media_tasks SET state='failed',billing_state='released',error='上游未受理，预占额度已释放',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if s.gateway.billingCacheService != nil {
		_ = s.gateway.billingCacheService.InvalidateUserBalance(ctx, t.UserID)
	}
	return nil
}

func (s *MediaTaskService) Refresh(ctx context.Context, t *MediaTask) error {
	if t.UpstreamTaskID == "" {
		return nil
	}
	if t.BillingState == "reserved" {
		if err := s.Settle(ctx, t); err != nil {
			logger.L().Warn("media_task.settlement_pending", zap.String("id", t.ID), zap.Error(err))
		}
	}
	if t.State == "succeeded" || t.State == "failed" || t.State == "cancelled" {
		return nil
	}
	account, err := s.accounts.GetByID(ctx, t.AccountID)
	if err != nil {
		return err
	}
	base, ok := MiniMaxVideoBaseURL(account)
	if !ok {
		return fmt.Errorf("上游账号不可用")
	}
	client, err := minimax.NewVideoClient(base, &http.Client{Timeout: 25 * time.Second})
	if err != nil {
		return err
	}
	result, err := client.Query(ctx, account.GetCredential("api_key"), t.UpstreamTaskID)
	if err != nil {
		return err
	}
	if result.Task.ID != t.UpstreamTaskID {
		return fmt.Errorf("上游任务不匹配")
	}
	switch result.Task.Status {
	case "queued", "running", "succeeded", "failed", "cancelled":
	default:
		return fmt.Errorf("未知上游任务状态")
	}
	payload, _ := json.Marshal(result)
	message := ""
	if result.Task.Status == "failed" || result.Task.Status == "cancelled" {
		message = "任务未成功，费用需根据上游账单核对"
	}
	_, err = s.db.ExecContext(ctx, `UPDATE media_tasks SET state=$2,result=$3,error=$4,updated_at=NOW() WHERE id=$1 AND state NOT IN ('succeeded','failed','cancelled')`, t.ID, result.Task.Status, payload, message)
	return err
}

func (s *MediaTaskService) Settle(ctx context.Context, t *MediaTask) error {
	key, err := s.keys.GetByID(ctx, t.APIKeyID)
	if err != nil {
		return err
	}
	if key.GroupID == nil || *key.GroupID != t.GroupID {
		return fmt.Errorf("任务密钥分组已变化，需要人工核对账务")
	}
	account, err := s.accounts.GetByID(ctx, t.AccountID)
	if err != nil {
		return err
	}
	var subscription *UserSubscription
	if t.SubscriptionID != nil {
		subscription, err = s.gateway.userSubRepo.GetByID(ctx, *t.SubscriptionID)
		if err != nil {
			return err
		}
	}
	err = s.gateway.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: t.UpstreamTaskID, ResponseID: t.UpstreamTaskID, Model: t.Model, UpstreamModel: t.Model, VideoCount: 1, VideoResolution: t.Specification.Resolution, VideoDurationSeconds: t.Specification.Duration, VideoQuote: &t.Quote},
		APIKey: key, User: key.User, Account: account, Subscription: subscription, MediaTaskID: t.ID, APIKeyService: s.keys,
		InboundEndpoint: "/v2/video_generation", UpstreamEndpoint: "/v2/video_generation", RequestPayloadHash: t.RequestHash,
	})
	if err == nil && s.gateway.cfg != nil && s.gateway.cfg.RunMode == "simple" {
		_, err = s.db.ExecContext(ctx, `UPDATE media_tasks SET billing_state='charged',updated_at=NOW() WHERE id=$1 AND hold_amount=0`, t.ID)
	}
	return err
}

func (s *MediaTaskService) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			rows, err := s.db.QueryContext(ctx, mediaTaskSelect+` WHERE upstream_task_id IS NOT NULL AND (billing_state='reserved' OR state IN ('queued','running')) ORDER BY updated_at LIMIT 20`)
			if err == nil {
				tasks := []*MediaTask{}
				for rows.Next() {
					t, e := scanMediaTask(rows)
					if e == nil {
						tasks = append(tasks, t)
					}
				}
				rows.Close()
				for _, t := range tasks {
					select {
					case <-s.stop:
						cancel()
						return
					default:
					}
					taskCtx, taskCancel := context.WithTimeout(context.Background(), 30*time.Second)
					refreshErr := s.Refresh(taskCtx, t)
					taskCancel()
					if refreshErr != nil {
						logger.L().Warn("media_task.refresh_pending", zap.String("id", t.ID), zap.Error(refreshErr))
						retryCtx, retryCancel := context.WithTimeout(context.Background(), 3*time.Second)
						_, _ = s.db.ExecContext(retryCtx, `UPDATE media_tasks SET updated_at=NOW() WHERE id=$1`, t.ID)
						retryCancel()
					}
				}
			}
			cancel()
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			// A crash after sending a request is ambiguous; never submit it again.
			_, _ = s.db.ExecContext(ctx, `UPDATE media_tasks SET state='uncertain',error='提交结果待核对，请联系管理员',updated_at=NOW() WHERE state='submitting' AND created_at<NOW()-INTERVAL '2 minutes'`)
			cancel()
		}
	}
}
