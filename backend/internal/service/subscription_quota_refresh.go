package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SubscriptionQuotaRefreshWindow string

const (
	SubscriptionQuotaRefreshWindowDaily   SubscriptionQuotaRefreshWindow = "daily"
	SubscriptionQuotaRefreshWindowWeekly  SubscriptionQuotaRefreshWindow = "weekly"
	SubscriptionQuotaRefreshWindowMonthly SubscriptionQuotaRefreshWindow = "monthly"
)

const (
	SubscriptionQuotaRefreshActorUser  = "user"
	SubscriptionQuotaRefreshActorAdmin = "admin"
)

const (
	QuotaRefreshReasonDisabled                = "disabled"
	QuotaRefreshReasonInvalidWindow           = "invalid_window"
	QuotaRefreshReasonLimitNotConfigured      = "limit_not_configured"
	QuotaRefreshReasonWindowNotActive         = "window_not_active"
	QuotaRefreshReasonSubscriptionNotActive   = "subscription_not_active"
	QuotaRefreshReasonNaturalResetAvailable   = "natural_reset_available"
	QuotaRefreshReasonNotExhausted            = "not_exhausted"
	QuotaRefreshReasonInsufficientValidity    = "insufficient_validity"
	QuotaRefreshReasonSubscriptionUnavailable = "subscription_unavailable"
)

const quotaRefreshUsageEpsilon = 1e-9

var (
	ErrQuotaRefreshDisabled = infraerrors.Forbidden(
		"QUOTA_REFRESH_DISABLED",
		"subscription quota refresh is disabled",
	)
	ErrQuotaRefreshInvalidWindow = infraerrors.BadRequest(
		"QUOTA_REFRESH_INVALID_WINDOW",
		"quota refresh window must be one of daily, weekly, or monthly",
	)
	ErrQuotaRefreshLimitNotConfigured = infraerrors.BadRequest(
		"QUOTA_REFRESH_LIMIT_NOT_CONFIGURED",
		"quota refresh requires a configured limit for the selected window",
	)
	ErrQuotaRefreshWindowNotActive = infraerrors.BadRequest(
		"QUOTA_REFRESH_WINDOW_NOT_ACTIVE",
		"quota refresh requires an active usage window",
	)
	ErrQuotaRefreshNotExhausted = infraerrors.BadRequest(
		"QUOTA_REFRESH_NOT_EXHAUSTED",
		"quota refresh is only available after the selected quota is exhausted",
	)
	ErrQuotaRefreshInsufficientValidity = infraerrors.BadRequest(
		"QUOTA_REFRESH_INSUFFICIENT_VALIDITY",
		"subscription validity is insufficient for this quota refresh",
	)
	ErrQuotaRefreshNaturalResetAvailable = infraerrors.Conflict(
		"QUOTA_REFRESH_NATURAL_RESET_AVAILABLE",
		"the selected quota window can reset naturally without deducting validity",
	)
	ErrQuotaRefreshSubscriptionNotActive = infraerrors.Forbidden(
		"SUBSCRIPTION_NOT_ACTIVE",
		"subscription is not active",
	)
)

type SubscriptionQuotaRefreshWindowInfo struct {
	Window             SubscriptionQuotaRefreshWindow `json:"window"`
	Eligible           bool                           `json:"eligible"`
	Reason             string                         `json:"reason,omitempty"`
	DeductedSeconds    int64                          `json:"deducted_seconds"`
	CurrentUsageUSD    float64                        `json:"current_usage_usd"`
	LimitUSD           *float64                       `json:"limit_usd,omitempty"`
	CurrentWindowStart *time.Time                     `json:"current_window_start,omitempty"`
	NextResetAt        *time.Time                     `json:"next_reset_at,omitempty"`
	ProjectedExpiresAt *time.Time                     `json:"projected_expires_at,omitempty"`
}

type SubscriptionQuotaRefreshSummary struct {
	Daily   SubscriptionQuotaRefreshWindowInfo `json:"daily"`
	Weekly  SubscriptionQuotaRefreshWindowInfo `json:"weekly"`
	Monthly SubscriptionQuotaRefreshWindowInfo `json:"monthly"`
}

type RefreshSubscriptionQuotaInput struct {
	SubscriptionID     int64
	Window             SubscriptionQuotaRefreshWindow
	RequireUserID      *int64
	ActorType          string
	ActorID            *int64
	IdempotencyKeyHash string
}

type SubscriptionQuotaRefreshEvent struct {
	SubscriptionID     int64
	UserID             int64
	GroupID            int64
	ActorType          string
	ActorID            *int64
	Window             SubscriptionQuotaRefreshWindow
	DeductedSeconds    int64
	OldExpiresAt       time.Time
	NewExpiresAt       time.Time
	OldWindowStart     *time.Time
	NewWindowStart     time.Time
	OldUsageUSD        float64
	LimitUSD           float64
	IdempotencyKeyHash string
	CreatedAt          time.Time
}

type SubscriptionQuotaRefreshPersistInput struct {
	SubscriptionID int64
	Window         SubscriptionQuotaRefreshWindow
	NewWindowStart time.Time
	NewExpiresAt   time.Time
	Event          *SubscriptionQuotaRefreshEvent
}

type SubscriptionQuotaRefreshResult struct {
	Window          SubscriptionQuotaRefreshWindow `json:"window"`
	DeductedSeconds int64                          `json:"deducted_seconds"`
	OldExpiresAt    time.Time                      `json:"old_expires_at"`
	NewExpiresAt    time.Time                      `json:"new_expires_at"`
	OldWindowStart  *time.Time                     `json:"old_window_start,omitempty"`
	NewWindowStart  time.Time                      `json:"new_window_start"`
	OldUsageUSD     float64                        `json:"old_usage_usd"`
	LimitUSD        float64                        `json:"limit_usd"`
	Subscription    *UserSubscription              `json:"subscription"`
}

func (w SubscriptionQuotaRefreshWindow) Valid() bool {
	switch w {
	case SubscriptionQuotaRefreshWindowDaily, SubscriptionQuotaRefreshWindowWeekly, SubscriptionQuotaRefreshWindowMonthly:
		return true
	default:
		return false
	}
}

func (s *SubscriptionService) BuildSubscriptionQuotaRefreshSummary(sub *UserSubscription) *SubscriptionQuotaRefreshSummary {
	if sub == nil || sub.Group == nil {
		return nil
	}
	now := time.Now()
	return &SubscriptionQuotaRefreshSummary{
		Daily:   buildSubscriptionQuotaRefreshInfo(sub, SubscriptionQuotaRefreshWindowDaily, now, s.quotaRefreshEnabled),
		Weekly:  buildSubscriptionQuotaRefreshInfo(sub, SubscriptionQuotaRefreshWindowWeekly, now, s.quotaRefreshEnabled),
		Monthly: buildSubscriptionQuotaRefreshInfo(sub, SubscriptionQuotaRefreshWindowMonthly, now, s.quotaRefreshEnabled),
	}
}

func (s *SubscriptionService) attachQuotaRefreshSummary(sub *UserSubscription) {
	if s == nil || sub == nil {
		return
	}
	sub.QuotaRefresh = s.BuildSubscriptionQuotaRefreshSummary(sub)
}

func (s *SubscriptionService) attachQuotaRefreshSummaries(subs []UserSubscription) {
	for i := range subs {
		s.attachQuotaRefreshSummary(&subs[i])
	}
}

func buildSubscriptionQuotaRefreshInfo(
	sub *UserSubscription,
	window SubscriptionQuotaRefreshWindow,
	now time.Time,
	enabled bool,
) SubscriptionQuotaRefreshWindowInfo {
	info := SubscriptionQuotaRefreshWindowInfo{
		Window: window,
	}
	if sub == nil || sub.Group == nil {
		info.Reason = QuotaRefreshReasonSubscriptionUnavailable
		return info
	}
	usage, limit, windowStart := subscriptionQuotaRefreshWindowState(sub, window)
	info.CurrentUsageUSD = usage
	info.LimitUSD = limit
	info.CurrentWindowStart = windowStart
	info.NextResetAt = subscriptionQuotaRefreshNextResetAt(sub, window)

	if !enabled {
		info.Reason = QuotaRefreshReasonDisabled
		return info
	}
	if !window.Valid() {
		info.Reason = QuotaRefreshReasonInvalidWindow
		return info
	}
	if sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) {
		info.Reason = QuotaRefreshReasonSubscriptionNotActive
		return info
	}
	if limit == nil || *limit <= 0 {
		info.Reason = QuotaRefreshReasonLimitNotConfigured
		return info
	}
	if windowStart == nil {
		info.Reason = QuotaRefreshReasonWindowNotActive
		return info
	}
	deductDuration, ok := subscriptionQuotaRefreshDeductionDuration(info.NextResetAt, now)
	if !ok {
		info.Reason = QuotaRefreshReasonNaturalResetAvailable
		return info
	}
	info.DeductedSeconds = subscriptionQuotaRefreshDeductedSeconds(deductDuration)
	if usage+quotaRefreshUsageEpsilon < *limit {
		info.Reason = QuotaRefreshReasonNotExhausted
		return info
	}
	projected := sub.ExpiresAt.Add(-deductDuration)
	info.ProjectedExpiresAt = &projected
	if !projected.After(now) {
		info.Reason = QuotaRefreshReasonInsufficientValidity
		return info
	}
	info.Eligible = true
	return info
}

func subscriptionQuotaRefreshDeductionDuration(nextResetAt *time.Time, now time.Time) (time.Duration, bool) {
	if nextResetAt == nil || !nextResetAt.After(now) {
		return 0, false
	}
	return nextResetAt.Sub(now), true
}

func subscriptionQuotaRefreshDeductedSeconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	return int64((duration + time.Second - time.Nanosecond) / time.Second)
}

func (s *SubscriptionService) RefreshSubscriptionQuota(ctx context.Context, input RefreshSubscriptionQuotaInput) (*SubscriptionQuotaRefreshResult, error) {
	if s == nil {
		return nil, ErrSubscriptionNotFound
	}
	if !s.quotaRefreshEnabled {
		return nil, ErrQuotaRefreshDisabled
	}
	if !input.Window.Valid() {
		return nil, ErrQuotaRefreshInvalidWindow
	}
	if input.ActorType == "" {
		input.ActorType = SubscriptionQuotaRefreshActorUser
	}

	var result *SubscriptionQuotaRefreshResult
	if err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		now := time.Now()
		sub, err := s.userSubRepo.GetByIDForUpdate(txCtx, input.SubscriptionID)
		if err != nil {
			return err
		}
		if input.RequireUserID != nil && sub.UserID != *input.RequireUserID {
			return ErrSubscriptionNotFound
		}

		info := buildSubscriptionQuotaRefreshInfo(sub, input.Window, now, s.quotaRefreshEnabled)
		if !info.Eligible {
			return quotaRefreshErrorForReason(info.Reason)
		}

		oldUsage, limit, oldWindowStart := subscriptionQuotaRefreshWindowState(sub, input.Window)
		if limit == nil {
			return ErrQuotaRefreshLimitNotConfigured
		}
		newWindowStart := now
		newExpiresAt := *info.ProjectedExpiresAt
		event := &SubscriptionQuotaRefreshEvent{
			SubscriptionID:     sub.ID,
			UserID:             sub.UserID,
			GroupID:            sub.GroupID,
			ActorType:          input.ActorType,
			ActorID:            input.ActorID,
			Window:             input.Window,
			DeductedSeconds:    info.DeductedSeconds,
			OldExpiresAt:       sub.ExpiresAt,
			NewExpiresAt:       newExpiresAt,
			OldWindowStart:     cloneTimePtr(oldWindowStart),
			NewWindowStart:     newWindowStart,
			OldUsageUSD:        oldUsage,
			LimitUSD:           *limit,
			IdempotencyKeyHash: input.IdempotencyKeyHash,
			CreatedAt:          now,
		}
		updated, err := s.userSubRepo.RefreshQuotaWindow(txCtx, &SubscriptionQuotaRefreshPersistInput{
			SubscriptionID: sub.ID,
			Window:         input.Window,
			NewWindowStart: newWindowStart,
			NewExpiresAt:   newExpiresAt,
			Event:          event,
		})
		if err != nil {
			return err
		}
		s.attachQuotaRefreshSummary(updated)
		result = &SubscriptionQuotaRefreshResult{
			Window:          input.Window,
			DeductedSeconds: info.DeductedSeconds,
			OldExpiresAt:    sub.ExpiresAt,
			NewExpiresAt:    newExpiresAt,
			OldWindowStart:  cloneTimePtr(oldWindowStart),
			NewWindowStart:  newWindowStart,
			OldUsageUSD:     oldUsage,
			LimitUSD:        *limit,
			Subscription:    updated,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if result != nil && result.Subscription != nil {
		s.InvalidateSubCache(result.Subscription.UserID, result.Subscription.GroupID)
		if s.subCacheL1 != nil {
			s.subCacheL1.Wait()
		}
		if s.billingCacheService != nil {
			_ = s.billingCacheService.InvalidateSubscription(ctx, result.Subscription.UserID, result.Subscription.GroupID)
		}
	}

	return result, nil
}

func quotaRefreshErrorForReason(reason string) error {
	switch reason {
	case QuotaRefreshReasonDisabled:
		return ErrQuotaRefreshDisabled
	case QuotaRefreshReasonInvalidWindow:
		return ErrQuotaRefreshInvalidWindow
	case QuotaRefreshReasonLimitNotConfigured:
		return ErrQuotaRefreshLimitNotConfigured
	case QuotaRefreshReasonWindowNotActive:
		return ErrQuotaRefreshWindowNotActive
	case QuotaRefreshReasonSubscriptionNotActive:
		return ErrQuotaRefreshSubscriptionNotActive
	case QuotaRefreshReasonNaturalResetAvailable:
		return ErrQuotaRefreshNaturalResetAvailable
	case QuotaRefreshReasonNotExhausted:
		return ErrQuotaRefreshNotExhausted
	case QuotaRefreshReasonInsufficientValidity:
		return ErrQuotaRefreshInsufficientValidity
	default:
		return ErrQuotaRefreshInvalidWindow
	}
}

func subscriptionQuotaRefreshWindowState(sub *UserSubscription, window SubscriptionQuotaRefreshWindow) (float64, *float64, *time.Time) {
	if sub == nil || sub.Group == nil {
		return 0, nil, nil
	}
	switch window {
	case SubscriptionQuotaRefreshWindowDaily:
		return sub.DailyUsageUSD, sub.Group.DailyLimitUSD, sub.DailyWindowStart
	case SubscriptionQuotaRefreshWindowWeekly:
		return sub.WeeklyUsageUSD, sub.Group.WeeklyLimitUSD, sub.WeeklyWindowStart
	case SubscriptionQuotaRefreshWindowMonthly:
		return sub.MonthlyUsageUSD, sub.Group.MonthlyLimitUSD, sub.MonthlyWindowStart
	default:
		return 0, nil, nil
	}
}

func subscriptionQuotaRefreshNextResetAt(sub *UserSubscription, window SubscriptionQuotaRefreshWindow) *time.Time {
	if sub == nil {
		return nil
	}
	switch window {
	case SubscriptionQuotaRefreshWindowDaily:
		return sub.DailyResetTime()
	case SubscriptionQuotaRefreshWindowWeekly:
		return sub.WeeklyResetTime()
	case SubscriptionQuotaRefreshWindowMonthly:
		return sub.MonthlyResetTime()
	default:
		return nil
	}
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}
