package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type quotaRefreshUserSubRepoStub struct {
	userSubRepoNoop

	sub          *UserSubscription
	refreshCalls int
}

func (r *quotaRefreshUserSubRepoStub) GetByIDForUpdate(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	if r.sub.Group != nil {
		gp := *r.sub.Group
		cp.Group = &gp
	}
	return &cp, nil
}

func (r *quotaRefreshUserSubRepoStub) RefreshQuotaWindow(_ context.Context, input *SubscriptionQuotaRefreshPersistInput) (*UserSubscription, error) {
	r.refreshCalls++
	require := func(ok bool) {
		if !ok {
			panic("invalid refresh input")
		}
	}
	require(input != nil)
	require(r.sub != nil && r.sub.ID == input.SubscriptionID)
	r.sub.ExpiresAt = input.NewExpiresAt
	switch input.Window {
	case SubscriptionQuotaRefreshWindowDaily:
		r.sub.DailyUsageUSD = 0
		r.sub.DailyWindowStart = &input.NewWindowStart
	case SubscriptionQuotaRefreshWindowWeekly:
		r.sub.WeeklyUsageUSD = 0
		r.sub.WeeklyWindowStart = &input.NewWindowStart
	case SubscriptionQuotaRefreshWindowMonthly:
		r.sub.MonthlyUsageUSD = 0
		r.sub.MonthlyWindowStart = &input.NewWindowStart
	default:
		return nil, ErrQuotaRefreshInvalidWindow
	}
	cp := *r.sub
	if r.sub.Group != nil {
		gp := *r.sub.Group
		cp.Group = &gp
	}
	return &cp, nil
}

func newQuotaRefreshSvc(stub *quotaRefreshUserSubRepoStub) *SubscriptionService {
	return NewSubscriptionService(groupRepoNoop{}, stub, nil, nil, nil)
}

func quotaRefreshTestSub(window SubscriptionQuotaRefreshWindow, usage, limit float64, start *time.Time, expiresAt time.Time) *UserSubscription {
	group := &Group{ID: 20}
	sub := &UserSubscription{
		ID:        1,
		UserID:    10,
		GroupID:   20,
		Status:    SubscriptionStatusActive,
		StartsAt:  time.Now().Add(-48 * time.Hour),
		ExpiresAt: expiresAt,
		Group:     group,
	}
	switch window {
	case SubscriptionQuotaRefreshWindowDaily:
		group.DailyLimitUSD = &limit
		sub.DailyUsageUSD = usage
		sub.DailyWindowStart = start
	case SubscriptionQuotaRefreshWindowWeekly:
		group.WeeklyLimitUSD = &limit
		sub.WeeklyUsageUSD = usage
		sub.WeeklyWindowStart = start
	case SubscriptionQuotaRefreshWindowMonthly:
		group.MonthlyLimitUSD = &limit
		sub.MonthlyUsageUSD = usage
		sub.MonthlyWindowStart = start
	}
	return sub
}

func TestRefreshSubscriptionQuota_SuccessWindows(t *testing.T) {
	tests := []struct {
		name             string
		window           SubscriptionQuotaRefreshWindow
		windowElapsed    time.Duration
		expectedDeducted time.Duration
	}{
		{name: "daily", window: SubscriptionQuotaRefreshWindowDaily, windowElapsed: 20 * time.Hour, expectedDeducted: 4 * time.Hour},
		{name: "weekly", window: SubscriptionQuotaRefreshWindowWeekly, windowElapsed: 6 * 24 * time.Hour, expectedDeducted: 24 * time.Hour},
		{name: "monthly", window: SubscriptionQuotaRefreshWindowMonthly, windowElapsed: 29 * 24 * time.Hour, expectedDeducted: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			windowStart := before.Add(-tt.windowElapsed)
			expiresAt := before.Add(tt.expectedDeducted + 48*time.Hour)
			stub := &quotaRefreshUserSubRepoStub{
				sub: quotaRefreshTestSub(tt.window, 10, 10, &windowStart, expiresAt),
			}
			svc := newQuotaRefreshSvc(stub)

			result, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
				SubscriptionID: 1,
				Window:         tt.window,
				ActorType:      SubscriptionQuotaRefreshActorUser,
			})

			require.NoError(t, err)
			require.Equal(t, tt.window, result.Window)
			require.InDelta(t, tt.expectedDeducted.Seconds(), result.DeductedSeconds, 1)
			require.Equal(t, float64(10), result.OldUsageUSD)
			require.Equal(t, float64(10), result.LimitUSD)
			require.WithinDuration(t, expiresAt.Add(-tt.expectedDeducted), result.NewExpiresAt, time.Second)
			require.NotNil(t, result.OldWindowStart)
			require.Equal(t, windowStart, *result.OldWindowStart)
			require.True(t, !result.NewWindowStart.Before(before))
			require.Equal(t, 1, stub.refreshCalls)
			require.NotNil(t, result.Subscription.QuotaRefresh)
		})
	}
}

func TestRefreshSubscriptionQuota_InsufficientValidity(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-20 * time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(3*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshInsufficientValidity)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_ExactDurationValidityIsInsufficient(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-20 * time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(4*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshInsufficientValidity)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestBuildSubscriptionQuotaRefreshInfo_DeductsRemainingWindowTime(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name             string
		window           SubscriptionQuotaRefreshWindow
		windowElapsed    time.Duration
		expectedDeducted time.Duration
	}{
		{name: "daily", window: SubscriptionQuotaRefreshWindowDaily, windowElapsed: 20 * time.Hour, expectedDeducted: 4 * time.Hour},
		{name: "weekly", window: SubscriptionQuotaRefreshWindowWeekly, windowElapsed: 6 * 24 * time.Hour, expectedDeducted: 24 * time.Hour},
		{name: "monthly", window: SubscriptionQuotaRefreshWindowMonthly, windowElapsed: 29 * 24 * time.Hour, expectedDeducted: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windowStart := now.Add(-tt.windowElapsed)
			expiresAt := now.Add(10 * 24 * time.Hour)
			sub := quotaRefreshTestSub(tt.window, 10, 10, &windowStart, expiresAt)
			sub.StartsAt = now.Add(-60 * 24 * time.Hour)

			info := buildSubscriptionQuotaRefreshInfo(sub, tt.window, now, true)

			require.True(t, info.Eligible)
			require.Equal(t, int64(tt.expectedDeducted/time.Second), info.DeductedSeconds)
			require.NotNil(t, info.ProjectedExpiresAt)
			require.Equal(t, expiresAt.Add(-tt.expectedDeducted), *info.ProjectedExpiresAt)
		})
	}
}

func TestRefreshSubscriptionQuota_NotExhausted(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 9.99, 10, &windowStart, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshNotExhausted)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_LimitNotConfigured(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	sub := quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour))
	sub.Group.DailyLimitUSD = nil
	stub := &quotaRefreshUserSubRepoStub{sub: sub}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshLimitNotConfigured)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_WindowNotActive(t *testing.T) {
	now := time.Now()
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, nil, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshWindowNotActive)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_NaturalResetAvailable(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-25 * time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshNaturalResetAvailable)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_InvalidWindow(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindow("yearly"),
	})

	require.ErrorIs(t, err, ErrQuotaRefreshInvalidWindow)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_SubscriptionNotActive(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	sub := quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour))
	sub.Status = SubscriptionStatusSuspended
	stub := &quotaRefreshUserSubRepoStub{sub: sub}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshSubscriptionNotActive)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_OwnershipRequired(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	userID := int64(999)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
		RequireUserID:  &userID,
	})

	require.ErrorIs(t, err, ErrSubscriptionNotFound)
	require.Equal(t, 0, stub.refreshCalls)
}

func TestRefreshSubscriptionQuota_Disabled(t *testing.T) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	stub := &quotaRefreshUserSubRepoStub{
		sub: quotaRefreshTestSub(SubscriptionQuotaRefreshWindowDaily, 10, 10, &windowStart, now.Add(72*time.Hour)),
	}
	svc := newQuotaRefreshSvc(stub)
	svc.quotaRefreshEnabled = false

	_, err := svc.RefreshSubscriptionQuota(context.Background(), RefreshSubscriptionQuotaInput{
		SubscriptionID: 1,
		Window:         SubscriptionQuotaRefreshWindowDaily,
	})

	require.ErrorIs(t, err, ErrQuotaRefreshDisabled)
	require.Equal(t, 0, stub.refreshCalls)
}
