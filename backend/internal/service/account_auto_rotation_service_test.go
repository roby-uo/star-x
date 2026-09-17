package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountAutoRotationRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r *accountAutoRotationRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	result := make([]Account, len(r.accounts))
	copy(result, r.accounts)
	return result, nil
}

func (r *accountAutoRotationRepoStub) SetSchedulable(_ context.Context, id int64, schedulable bool) error {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			r.accounts[i].Schedulable = schedulable
		}
	}
	return nil
}

type accountAutoRotationSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (r *accountAutoRotationSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *accountAutoRotationSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func TestAccountAutoRotation_ExhaustedAccountSwitchesImmediatelyAndRejoinsNextCycle(t *testing.T) {
	now := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	repo := &accountAutoRotationRepoStub{accounts: []Account{
		{ID: 10, Name: "A", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: quotaSnapshot(20, 30, now)},
		{ID: 20, Name: "B", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: quotaSnapshot(10, 40, now)},
	}}
	settings := &accountAutoRotationSettingRepoStub{values: map[string]string{}}
	svc := NewAccountAutoRotationService(repo, settings)
	svc.now = func() time.Time { return now }

	status, err := svc.SetEnabled(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, int64(10), *status.ActiveAccountID)
	require.True(t, repo.accounts[0].Schedulable)
	require.False(t, repo.accounts[1].Schedulable)
	require.Equal(t, now.Add(5*time.Hour), *status.NextRotationAt)

	// A uses up its short window before the five-hour timer expires. The next
	// reconciliation must fail over to B immediately and start a fresh cycle.
	now = now.Add(time.Hour)
	repo.accounts[0].Extra = quotaSnapshot(100, 30, now)
	status, err = svc.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(20), *status.ActiveAccountID)
	require.False(t, repo.accounts[0].Schedulable)
	require.True(t, repo.accounts[1].Schedulable)
	require.Equal(t, now.Add(5*time.Hour), *status.NextRotationAt)

	// A has quota again by B's normal boundary, so it naturally rejoins the
	// ordered pool and becomes the next serving account.
	now = now.Add(5 * time.Hour)
	repo.accounts[0].Extra = quotaSnapshot(5, 30, now)
	status, err = svc.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(10), *status.ActiveAccountID)
	require.Equal(t, []int64{10, 20}, status.EligibleAccountIDs)
}

func TestAccountAutoRotation_DisableRestoresOriginalSchedulableState(t *testing.T) {
	now := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	repo := &accountAutoRotationRepoStub{accounts: []Account{
		{ID: 1, Name: "manual-off", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: false},
		{ID: 2, Name: "manual-on", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
	}}
	settings := &accountAutoRotationSettingRepoStub{values: map[string]string{}}
	svc := NewAccountAutoRotationService(repo, settings)
	svc.now = func() time.Time { return now }

	_, err := svc.SetEnabled(context.Background(), true)
	require.NoError(t, err)
	require.True(t, repo.accounts[0].Schedulable)
	require.False(t, repo.accounts[1].Schedulable)

	status, err := svc.SetEnabled(context.Background(), false)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.False(t, repo.accounts[0].Schedulable)
	require.True(t, repo.accounts[1].Schedulable)
}

func TestAccountAutoRotation_UpdateIntervalPersistsAndRecalculatesCurrentCycle(t *testing.T) {
	now := time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	repo := &accountAutoRotationRepoStub{accounts: []Account{
		{ID: 10, Name: "A", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
		{ID: 20, Name: "B", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
	}}
	settings := &accountAutoRotationSettingRepoStub{values: map[string]string{}}
	svc := NewAccountAutoRotationService(repo, settings)
	svc.now = func() time.Time { return now }

	status, err := svc.SetEnabled(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, int64((5 * time.Hour).Seconds()), status.IntervalSeconds)
	require.Equal(t, now.Add(5*time.Hour), *status.NextRotationAt)

	now = now.Add(15 * time.Minute)
	intervalSeconds := int64((90 * time.Minute).Seconds())
	status, err = svc.Update(context.Background(), nil, &intervalSeconds)
	require.NoError(t, err)
	require.Equal(t, intervalSeconds, status.IntervalSeconds)
	require.Equal(t, time.Date(2026, 9, 17, 9, 30, 0, 0, time.UTC), *status.NextRotationAt)
	require.Contains(t, settings.values[accountAutoRotationSettingKey], `"interval_seconds":5400`)

	status, err = svc.SetEnabled(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, intervalSeconds, status.IntervalSeconds)

	status, err = svc.SetEnabled(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, intervalSeconds, status.IntervalSeconds)
	require.Equal(t, now.Add(90*time.Minute), *status.NextRotationAt)
}

func TestAccountAutoRotation_UpdateIntervalRotatesImmediatelyWhenNewDeadlinePassed(t *testing.T) {
	now := time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	repo := &accountAutoRotationRepoStub{accounts: []Account{
		{ID: 10, Name: "A", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
		{ID: 20, Name: "B", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true},
	}}
	settings := &accountAutoRotationSettingRepoStub{values: map[string]string{}}
	svc := NewAccountAutoRotationService(repo, settings)
	svc.now = func() time.Time { return now }

	status, err := svc.SetEnabled(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, int64(10), *status.ActiveAccountID)

	now = now.Add(time.Hour)
	intervalSeconds := int64((30 * time.Minute).Seconds())
	status, err = svc.Update(context.Background(), nil, &intervalSeconds)
	require.NoError(t, err)
	require.Equal(t, int64(20), *status.ActiveAccountID)
	require.Equal(t, now.Add(30*time.Minute), *status.NextRotationAt)
}

func TestAccountAutoRotation_UpdateIntervalRejectsUnsupportedValue(t *testing.T) {
	svc := NewAccountAutoRotationService(&accountAutoRotationRepoStub{}, &accountAutoRotationSettingRepoStub{values: map[string]string{}})
	intervalSeconds := int64(2700)
	_, err := svc.Update(context.Background(), nil, &intervalSeconds)
	require.ErrorContains(t, err, "1800-second steps")
}

func TestAccountEligibleForAutoRotation_RequiresBothQuotaWindows(t *testing.T) {
	now := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive}

	account.Extra = quotaSnapshot(100, 20, now)
	require.False(t, accountEligibleForAutoRotation(account, now))
	account.Extra = quotaSnapshot(20, 100, now)
	require.False(t, accountEligibleForAutoRotation(account, now))
	account.Extra = quotaSnapshot(99.9, 99.9, now)
	require.True(t, accountEligibleForAutoRotation(account, now))

	account.Extra = quotaSnapshot(100, 100, now.Add(-8*24*time.Hour))
	require.True(t, accountEligibleForAutoRotation(account, now), "expired quota windows are available again")
}

func quotaSnapshot(fiveHourUsed, sevenDayUsed float64, sampledAt time.Time) map[string]any {
	return map[string]any{
		"codex_5h_used_percent":  fiveHourUsed,
		"codex_7d_used_percent":  sevenDayUsed,
		"codex_5h_reset_at":      sampledAt.Add(5 * time.Hour).Format(time.RFC3339),
		"codex_7d_reset_at":      sampledAt.Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"codex_usage_updated_at": sampledAt.Format(time.RFC3339),
	}
}
