package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	accountAutoRotationSettingKey = "account_auto_rotation_state"
	accountAutoRotationInterval   = 5 * time.Hour
	accountAutoRotationTick       = 30 * time.Second
	accountAutoRotationMin        = 30 * time.Minute
	accountAutoRotationMax        = 4 * time.Hour
	accountAutoRotationStep       = 30 * time.Minute
)

// AccountAutoRotationStatus is the administrator-facing state of the timed
// OpenAI OAuth account rotation.
type AccountAutoRotationStatus struct {
	Enabled              bool       `json:"enabled"`
	IntervalSeconds      int64      `json:"interval_seconds"`
	ActiveAccountID      *int64     `json:"active_account_id,omitempty"`
	ActiveAccountName    string     `json:"active_account_name,omitempty"`
	CycleStartedAt       *time.Time `json:"cycle_started_at,omitempty"`
	NextRotationAt       *time.Time `json:"next_rotation_at,omitempty"`
	EligibleAccountIDs   []int64    `json:"eligible_account_ids"`
	EligibleAccountCount int        `json:"eligible_account_count"`
}

type accountAutoRotationState struct {
	Enabled             bool            `json:"enabled"`
	IntervalSeconds     int64           `json:"interval_seconds,omitempty"`
	ActiveAccountID     *int64          `json:"active_account_id,omitempty"`
	LastActiveAccountID *int64          `json:"last_active_account_id,omitempty"`
	CycleStartedAt      *time.Time      `json:"cycle_started_at,omitempty"`
	NextRotationAt      *time.Time      `json:"next_rotation_at,omitempty"`
	OriginalSchedulable map[string]bool `json:"original_schedulable,omitempty"`
}

// AccountAutoRotationService owns the schedulable flag for regular OpenAI
// OAuth accounts while automatic rotation is enabled. State is stored in the
// settings table, so countdown and cursor survive process restarts.
type AccountAutoRotationService struct {
	accountRepo AccountRepository
	settingRepo SettingRepository
	settingSvc  *SettingService
	interval    time.Duration
	tick        time.Duration
	now         func() time.Time

	mu       sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func (s *AccountAutoRotationService) SetSettingService(settingSvc *SettingService) {
	if s != nil {
		s.settingSvc = settingSvc
	}
}

func NewAccountAutoRotationService(accountRepo AccountRepository, settingRepo SettingRepository) *AccountAutoRotationService {
	return &AccountAutoRotationService{
		accountRepo: accountRepo,
		settingRepo: settingRepo,
		interval:    accountAutoRotationInterval,
		tick:        accountAutoRotationTick,
		now:         time.Now,
		stopCh:      make(chan struct{}),
	}
}

func (s *AccountAutoRotationService) Start() {
	if s == nil || s.accountRepo == nil || s.settingRepo == nil || s.tick <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		s.reconcileInBackground()
		for {
			select {
			case <-ticker.C:
				s.reconcileInBackground()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AccountAutoRotationService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *AccountAutoRotationService) reconcileInBackground() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if _, err := s.Status(ctx); err != nil {
		slog.Warn("account_auto_rotation_reconcile_failed", "error", err)
	}
}

func (s *AccountAutoRotationService) Status(ctx context.Context) (*AccountAutoRotationStatus, error) {
	if s == nil || s.accountRepo == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("account auto rotation service unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reconcileLocked(ctx)
}

func (s *AccountAutoRotationService) SetEnabled(ctx context.Context, enabled bool) (*AccountAutoRotationStatus, error) {
	return s.Update(ctx, &enabled, nil)
}

// Update changes the enabled state and/or interval in one persisted operation.
// Existing installations without an interval retain the legacy five-hour
// default until an administrator selects one of the configurable intervals.
func (s *AccountAutoRotationService) Update(ctx context.Context, enabled *bool, intervalSeconds *int64) (*AccountAutoRotationStatus, error) {
	if s == nil || s.accountRepo == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("account auto rotation service unavailable")
	}
	if enabled == nil && intervalSeconds == nil {
		return nil, fmt.Errorf("automatic rotation update is empty")
	}
	if intervalSeconds != nil && !validAccountAutoRotationInterval(*intervalSeconds) {
		return nil, fmt.Errorf("rotation interval must be between 1800 and 14400 seconds in 1800-second steps")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, fmt.Errorf("list OpenAI accounts: %w", err)
	}
	candidates := regularOpenAIOAuthAccounts(accounts)
	wasEnabled := state.Enabled
	if intervalSeconds != nil {
		state.IntervalSeconds = *intervalSeconds
		if state.Enabled && state.CycleStartedAt != nil {
			nextAt := state.CycleStartedAt.Add(time.Duration(*intervalSeconds) * time.Second)
			state.NextRotationAt = &nextAt
		}
	}
	if enabled != nil {
		state.Enabled = *enabled
	}

	if !state.Enabled {
		if wasEnabled {
			if err := s.restoreOriginalSchedulable(ctx, candidates, state.OriginalSchedulable); err != nil {
				return nil, err
			}
		}
		state = accountAutoRotationState{IntervalSeconds: state.IntervalSeconds}
		if err := s.saveState(ctx, &state); err != nil {
			return nil, err
		}
		return buildAccountAutoRotationStatus(&state, nil, nil, s.intervalForState(&state)), nil
	}

	if !wasEnabled {
		state.OriginalSchedulable = make(map[string]bool, len(candidates))
		for i := range candidates {
			state.OriginalSchedulable[strconv.FormatInt(candidates[i].ID, 10)] = candidates[i].Schedulable
		}
	}
	if err := s.saveState(ctx, &state); err != nil {
		return nil, err
	}
	return s.reconcileStateLocked(ctx, &state, candidates)
}

func (s *AccountAutoRotationService) IsEnabled(ctx context.Context) (bool, error) {
	if s == nil || s.settingRepo == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadState(ctx)
	if err != nil {
		return false, err
	}
	return state.Enabled, nil
}

func (s *AccountAutoRotationService) reconcileLocked(ctx context.Context) (*AccountAutoRotationStatus, error) {
	state, err := s.loadState(ctx)
	if err != nil {
		return nil, err
	}
	if !state.Enabled {
		return buildAccountAutoRotationStatus(&state, nil, nil, s.intervalForState(&state)), nil
	}
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, fmt.Errorf("list OpenAI accounts: %w", err)
	}
	return s.reconcileStateLocked(ctx, &state, regularOpenAIOAuthAccounts(accounts))
}

func (s *AccountAutoRotationService) reconcileStateLocked(ctx context.Context, state *accountAutoRotationState, candidates []Account) (*AccountAutoRotationStatus, error) {
	stateBefore, _ := json.Marshal(state)
	now := s.now().UTC()
	interval := s.intervalForState(state)
	if state.OriginalSchedulable == nil {
		state.OriginalSchedulable = make(map[string]bool)
	}
	for i := range candidates {
		key := strconv.FormatInt(candidates[i].ID, 10)
		if _, exists := state.OriginalSchedulable[key]; !exists {
			state.OriginalSchedulable[key] = candidates[i].Schedulable
		}
	}

	eligible := make([]Account, 0, len(candidates))
	quotaCtx := ctx
	if s.settingSvc != nil {
		quotaCtx = withOpenAIQuotaAutoPauseSettings(ctx, s.settingSvc.GetOpenAIQuotaAutoPauseSettings(ctx))
	}
	for i := range candidates {
		quotaPaused, _ := shouldAutoPauseOpenAIAccountByQuota(quotaCtx, &candidates[i])
		if accountEligibleForAutoRotation(&candidates[i], now) && !quotaPaused {
			eligible = append(eligible, candidates[i])
		}
	}

	activeEligible := findAccountByID(eligible, state.ActiveAccountID) != nil
	rotationDue := state.NextRotationAt == nil || !now.Before(*state.NextRotationAt)
	if !activeEligible || rotationDue {
		cursor := state.ActiveAccountID
		if cursor == nil {
			cursor = state.LastActiveAccountID
		}
		next := nextRotationAccount(eligible, cursor)
		if next == nil {
			state.ActiveAccountID = nil
			state.CycleStartedAt = nil
			state.NextRotationAt = nil
		} else {
			id := next.ID
			started := now
			nextAt := now.Add(interval)
			state.ActiveAccountID = &id
			state.LastActiveAccountID = &id
			state.CycleStartedAt = &started
			state.NextRotationAt = &nextAt
		}
	}

	if err := s.applyExclusiveSchedulable(ctx, candidates, state.ActiveAccountID); err != nil {
		return nil, err
	}
	stateAfter, _ := json.Marshal(state)
	if !bytes.Equal(stateBefore, stateAfter) {
		if err := s.saveState(ctx, state); err != nil {
			return nil, err
		}
	}
	return buildAccountAutoRotationStatus(state, candidates, eligible, interval), nil
}

func (s *AccountAutoRotationService) intervalForState(state *accountAutoRotationState) time.Duration {
	if state != nil && state.IntervalSeconds > 0 {
		return time.Duration(state.IntervalSeconds) * time.Second
	}
	return s.interval
}

func validAccountAutoRotationInterval(seconds int64) bool {
	minSeconds := int64(accountAutoRotationMin / time.Second)
	maxSeconds := int64(accountAutoRotationMax / time.Second)
	stepSeconds := int64(accountAutoRotationStep / time.Second)
	return seconds >= minSeconds && seconds <= maxSeconds && seconds%stepSeconds == 0
}

func (s *AccountAutoRotationService) loadState(ctx context.Context) (accountAutoRotationState, error) {
	state := accountAutoRotationState{}
	raw, err := s.settingRepo.GetValue(ctx, accountAutoRotationSettingKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return state, nil
		}
		return state, fmt.Errorf("load account auto rotation state: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return state, fmt.Errorf("decode account auto rotation state: %w", err)
	}
	return state, nil
}

func (s *AccountAutoRotationService) saveState(ctx context.Context, state *accountAutoRotationState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode account auto rotation state: %w", err)
	}
	if err := s.settingRepo.Set(ctx, accountAutoRotationSettingKey, string(raw)); err != nil {
		return fmt.Errorf("save account auto rotation state: %w", err)
	}
	return nil
}

func (s *AccountAutoRotationService) applyExclusiveSchedulable(ctx context.Context, candidates []Account, activeID *int64) error {
	for i := range candidates {
		want := activeID != nil && candidates[i].ID == *activeID
		if candidates[i].Schedulable == want {
			continue
		}
		if err := s.accountRepo.SetSchedulable(ctx, candidates[i].ID, want); err != nil {
			return fmt.Errorf("set account %d schedulable=%t: %w", candidates[i].ID, want, err)
		}
	}
	return nil
}

func (s *AccountAutoRotationService) restoreOriginalSchedulable(ctx context.Context, candidates []Account, original map[string]bool) error {
	for i := range candidates {
		want, recorded := original[strconv.FormatInt(candidates[i].ID, 10)]
		if !recorded || want == candidates[i].Schedulable {
			continue
		}
		if err := s.accountRepo.SetSchedulable(ctx, candidates[i].ID, want); err != nil {
			return fmt.Errorf("restore account %d schedulable=%t: %w", candidates[i].ID, want, err)
		}
	}
	return nil
}

func regularOpenAIOAuthAccounts(accounts []Account) []Account {
	result := make([]Account, 0, len(accounts))
	for i := range accounts {
		if accounts[i].Platform == PlatformOpenAI && accounts[i].Type == AccountTypeOAuth && !accounts[i].IsShadow() {
			result = append(result, accounts[i])
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func accountEligibleForAutoRotation(account *Account, now time.Time) bool {
	if account == nil || account.Status != StatusActive || account.IsShadow() {
		return false
	}
	if account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return false
	}
	if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
		return false
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return false
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return false
	}
	return openAIQuotaWindowHasCapacity(account.Extra, "5h", "secondary", now) &&
		openAIQuotaWindowHasCapacity(account.Extra, "7d", "primary", now)
}

func openAIQuotaWindowHasCapacity(extra map[string]any, canonicalWindow, legacyWindow string, now time.Time) bool {
	window := canonicalWindow
	used, ok := resolveAccountExtraNumber(extra, "codex_"+canonicalWindow+"_used_percent")
	if !ok {
		window = legacyWindow
		used, ok = resolveAccountExtraNumber(extra, "codex_"+legacyWindow+"_used_percent")
	}
	if !ok {
		// No snapshot yet is treated as provisionally available. The first
		// upstream response or quota probe will provide the authoritative value.
		return true
	}
	if openAIQuotaWindowReset(extra, window, now) {
		return true
	}
	return used < 100
}

func nextRotationAccount(eligible []Account, cursor *int64) *Account {
	if len(eligible) == 0 {
		return nil
	}
	if cursor == nil {
		return &eligible[0]
	}
	for i := range eligible {
		if eligible[i].ID > *cursor {
			return &eligible[i]
		}
	}
	return &eligible[0]
}

func findAccountByID(accounts []Account, id *int64) *Account {
	if id == nil {
		return nil
	}
	for i := range accounts {
		if accounts[i].ID == *id {
			return &accounts[i]
		}
	}
	return nil
}

func buildAccountAutoRotationStatus(state *accountAutoRotationState, candidates, eligible []Account, interval time.Duration) *AccountAutoRotationStatus {
	status := &AccountAutoRotationStatus{
		Enabled:              state != nil && state.Enabled,
		IntervalSeconds:      int64(interval / time.Second),
		EligibleAccountIDs:   make([]int64, 0, len(eligible)),
		EligibleAccountCount: len(eligible),
	}
	if state == nil {
		return status
	}
	status.ActiveAccountID = state.ActiveAccountID
	status.CycleStartedAt = state.CycleStartedAt
	status.NextRotationAt = state.NextRotationAt
	for i := range eligible {
		status.EligibleAccountIDs = append(status.EligibleAccountIDs, eligible[i].ID)
	}
	if active := findAccountByID(candidates, state.ActiveAccountID); active != nil {
		status.ActiveAccountName = active.Name
	}
	return status
}
