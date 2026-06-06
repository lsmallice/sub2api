package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	LeaderboardWindowDaily   = "daily"
	LeaderboardWindowWeekly  = "weekly"
	LeaderboardWindowMonthly = "monthly"
	LeaderboardWindowAllTime = "all_time"

	leaderboardTopLimit            = 10
	leaderboardDisplayNameMaxRunes = 32
)

var (
	ErrLeaderboardInvalidDisplayName    = infraerrors.BadRequest("LEADERBOARD_INVALID_DISPLAY_NAME", "leaderboard display name is invalid")
	ErrLeaderboardBanned                = infraerrors.Forbidden("LEADERBOARD_BANNED", "leaderboard participation is disabled by admin")
	ErrLeaderboardUnavailable           = infraerrors.ServiceUnavailable("LEADERBOARD_UNAVAILABLE", "leaderboard service is unavailable")
	ErrLeaderboardBackfillInvalidRange  = infraerrors.BadRequest("LEADERBOARD_BACKFILL_INVALID_RANGE", "leaderboard backfill range is invalid")
	ErrLeaderboardBackfillRangeTooLarge = infraerrors.BadRequest("LEADERBOARD_BACKFILL_RANGE_TOO_LARGE", "leaderboard backfill range is too large")
)

type LeaderboardRepository interface {
	GetParticipant(ctx context.Context, userID int64) (*LeaderboardParticipant, error)
	UpsertParticipant(ctx context.Context, input LeaderboardParticipantUpsert) (*LeaderboardParticipant, error)
	RemoveParticipant(ctx context.Context, input LeaderboardParticipantRemove) (*LeaderboardParticipant, error)
	SetParticipantBanStatus(ctx context.Context, input LeaderboardParticipantBanUpdate) (*LeaderboardParticipant, error)
	GetRanking(ctx context.Context, window string, startTime, endTime time.Time, limit int, currentUserID int64) ([]LeaderboardRankRow, *LeaderboardRankRow, error)
	GetHonorStats(ctx context.Context, userIDs []int64) (map[int64]map[string]LeaderboardHonorStats, error)
	RebuildUsageDaily(ctx context.Context, startDate, endDate time.Time) (int64, error)
	RebuildHonorStats(ctx context.Context, cutoff time.Time, latestCompleted map[string]time.Time) (int64, error)
	SnapshotPeriod(ctx context.Context, window string, startTime, endTime time.Time, limit int) (int64, error)
}

type LeaderboardCache interface {
	GetOverview(ctx context.Context, key string) (string, error)
	SetOverview(ctx context.Context, key string, payload string, ttl time.Duration) error
	DeleteOverview(ctx context.Context, key string) error
	DeleteAll(ctx context.Context) error
}

type LeaderboardParticipant struct {
	UserID      int64      `json:"-"`
	IsOptedIn   bool       `json:"is_opted_in"`
	IsBanned    bool       `json:"is_banned"`
	DisplayName string     `json:"display_name,omitempty"`
	DisplayCode string     `json:"display_code,omitempty"`
	OptedInAt   *time.Time `json:"opted_in_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
}

type LeaderboardParticipantUpsert struct {
	UserID      int64
	IsOptedIn   bool
	DisplayName string
	DisplayCode string
	Now         time.Time
}

type LeaderboardParticipantBanUpdate struct {
	UserID      int64
	IsBanned    bool
	DisplayCode string
	Now         time.Time
}

type LeaderboardParticipantRemove struct {
	UserID      int64
	DisplayCode string
	Now         time.Time
}

type LeaderboardRankRow struct {
	UserID      int64
	Rank        int
	DisplayName string
	DisplayCode string
	AvatarURL   string
	Tokens      int64
	Requests    int64
}

type LeaderboardHonorStats struct {
	TopAppearances int
	ChampionCount  int
	BestRank       int
	CurrentStreak  int
	ChampionStarts map[string][]time.Time
}

type LeaderboardOverview struct {
	Participant LeaderboardParticipantStatus `json:"participant"`
	Daily       LeaderboardWindowOverview    `json:"daily"`
	Weekly      LeaderboardWindowOverview    `json:"weekly"`
	Monthly     LeaderboardWindowOverview    `json:"monthly"`
	AllTime     LeaderboardWindowOverview    `json:"all_time"`
}

type LeaderboardBackfillResult struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	PeriodCount  int       `json:"period_count"`
	InsertedRows int64     `json:"inserted_rows"`
}

type LeaderboardParticipantStatus struct {
	IsOptedIn   bool       `json:"is_opted_in"`
	IsBanned    bool       `json:"is_banned"`
	DisplayName string     `json:"display_name,omitempty"`
	DisplayCode string     `json:"display_code,omitempty"`
	PublicName  string     `json:"public_name,omitempty"`
	OptedInAt   *time.Time `json:"opted_in_at,omitempty"`
}

type LeaderboardWindowOverview struct {
	Window   string                   `json:"window"`
	StartsAt *time.Time               `json:"starts_at,omitempty"`
	EndsAt   time.Time                `json:"ends_at"`
	Top10    []LeaderboardPublicEntry `json:"top10"`
	Me       *LeaderboardPublicEntry  `json:"me,omitempty"`
}

type LeaderboardPublicEntry struct {
	Rank           int    `json:"rank,omitempty"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	Tokens         int64  `json:"tokens"`
	Requests       int64  `json:"requests"`
	CurrentStreak  int    `json:"current_streak,omitempty"`
	ChampionCount  int    `json:"champion_count,omitempty"`
	TopAppearances int    `json:"top_appearances,omitempty"`
	BestRank       int    `json:"best_rank,omitempty"`
	IsMe           bool   `json:"is_me,omitempty"`
}

type UpdateLeaderboardParticipantRequest struct {
	IsOptedIn   bool
	DisplayName *string
}

type LeaderboardService struct {
	repo  LeaderboardRepository
	cache LeaderboardCache
}

func NewLeaderboardService(repo LeaderboardRepository, caches ...LeaderboardCache) *LeaderboardService {
	var cache LeaderboardCache
	if len(caches) > 0 {
		cache = caches[0]
	}
	return &LeaderboardService{repo: repo, cache: cache}
}

func (s *LeaderboardService) GetOverview(ctx context.Context, userID int64, userTZ string) (*LeaderboardOverview, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}

	participant, err := s.repo.GetParticipant(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard participant: %w", err)
	}
	if participant == nil {
		participant = &LeaderboardParticipant{UserID: userID}
	}

	loc := resolveLeaderboardLocation(userTZ)
	now := time.Now().In(loc)
	endExclusive := startOfLeaderboardDay(now, loc).AddDate(0, 0, 1)
	cacheKey := leaderboardOverviewCacheKey(userID, now, loc)
	if s.cache != nil {
		if cached, err := s.cache.GetOverview(ctx, cacheKey); err == nil && strings.TrimSpace(cached) != "" {
			var overview LeaderboardOverview
			if err := json.Unmarshal([]byte(cached), &overview); err == nil {
				return &overview, nil
			}
		}
	}
	windows := []struct {
		name  string
		start *time.Time
	}{
		{name: LeaderboardWindowDaily, start: ptrLeaderboardTime(startOfLeaderboardDay(now, loc))},
		{name: LeaderboardWindowWeekly, start: ptrLeaderboardTime(startOfLeaderboardWeek(now, loc))},
		{name: LeaderboardWindowMonthly, start: ptrLeaderboardTime(startOfLeaderboardMonth(now, loc))},
		{name: LeaderboardWindowAllTime, start: nil},
	}

	rowsByWindow := make(map[string][]LeaderboardRankRow, len(windows))
	meByWindow := make(map[string]*LeaderboardRankRow, len(windows))
	userIDs := map[int64]struct{}{}
	for _, w := range windows {
		start := time.Time{}
		if w.start != nil {
			start = *w.start
		}
		topRows, meRow, err := s.repo.GetRanking(ctx, w.name, start, endExclusive, leaderboardTopLimit, userID)
		if err != nil {
			return nil, fmt.Errorf("get %s leaderboard ranking: %w", w.name, err)
		}
		rowsByWindow[w.name] = topRows
		meByWindow[w.name] = meRow
		for _, row := range topRows {
			userIDs[row.UserID] = struct{}{}
		}
		if meRow != nil {
			userIDs[meRow.UserID] = struct{}{}
		}
	}

	honorsByUser, err := s.repo.GetHonorStats(ctx, sortedInt64Keys(userIDs))
	if err != nil {
		return nil, fmt.Errorf("get leaderboard honor stats: %w", err)
	}
	build := func(window string, start *time.Time) LeaderboardWindowOverview {
		top10 := make([]LeaderboardPublicEntry, 0, len(rowsByWindow[window]))
		for _, row := range rowsByWindow[window] {
			top10 = append(top10, s.publicEntry(row, userID, honorStatsForWindow(honorsByUser, row.UserID, window)))
		}
		var me *LeaderboardPublicEntry
		if participant.IsOptedIn && meByWindow[window] != nil {
			entry := s.publicEntry(*meByWindow[window], userID, honorStatsForWindow(honorsByUser, meByWindow[window].UserID, window))
			me = &entry
		}
		return LeaderboardWindowOverview{
			Window:   window,
			StartsAt: start,
			EndsAt:   now,
			Top10:    top10,
			Me:       me,
		}
	}

	overview := &LeaderboardOverview{
		Participant: s.participantStatus(participant),
		Daily:       build(LeaderboardWindowDaily, windows[0].start),
		Weekly:      build(LeaderboardWindowWeekly, windows[1].start),
		Monthly:     build(LeaderboardWindowMonthly, windows[2].start),
		AllTime:     build(LeaderboardWindowAllTime, nil),
	}
	if s.cache != nil {
		if payload, err := json.Marshal(overview); err == nil {
			_ = s.cache.SetOverview(ctx, cacheKey, string(payload), 90*time.Second)
		}
	}
	return overview, nil
}

func (s *LeaderboardService) SnapshotHistoricalPeriodsSince(ctx context.Context, since, now time.Time) (*LeaderboardBackfillResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}
	loc := timezone.Location()
	actualNow := timezone.Now()
	if !now.IsZero() {
		now = now.In(loc)
	} else {
		now = actualNow
	}
	if now.After(actualNow) {
		now = actualNow
	}
	if !since.IsZero() {
		since = since.In(loc)
	}
	if since.IsZero() || !since.Before(now) {
		return nil, ErrLeaderboardBackfillInvalidRange
	}
	if now.Sub(since) > 370*24*time.Hour {
		return nil, ErrLeaderboardBackfillRangeTooLarge
	}

	periods := completedLeaderboardPeriodsSince(since, now, loc)
	inserted, err := s.rebuildDailyAggregatesAndSnapshotPeriods(ctx, since, now, periods)
	if err != nil {
		return nil, err
	}
	return &LeaderboardBackfillResult{
		StartTime:    since,
		EndTime:      now,
		PeriodCount:  len(periods),
		InsertedRows: inserted,
	}, nil
}

func (s *LeaderboardService) GetParticipant(ctx context.Context, userID int64) (*LeaderboardParticipantStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}
	participant, err := s.repo.GetParticipant(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard participant: %w", err)
	}
	if participant == nil {
		participant = &LeaderboardParticipant{UserID: userID}
	}
	status := s.participantStatus(participant)
	return &status, nil
}

func (s *LeaderboardService) UpdateParticipant(ctx context.Context, userID int64, req UpdateLeaderboardParticipantRequest) (*LeaderboardParticipantStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}

	participant, err := s.repo.GetParticipant(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard participant: %w", err)
	}
	if participant != nil && participant.IsBanned {
		return nil, ErrLeaderboardBanned
	}

	displayName, err := normalizeLeaderboardDisplayName(req.DisplayName)
	if err != nil {
		return nil, err
	}
	code, err := generateLeaderboardDisplayCode()
	if err != nil {
		return nil, fmt.Errorf("generate leaderboard display code: %w", err)
	}

	participant, err = s.repo.UpsertParticipant(ctx, LeaderboardParticipantUpsert{
		UserID:      userID,
		IsOptedIn:   req.IsOptedIn,
		DisplayName: displayName,
		DisplayCode: code,
		Now:         time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("update leaderboard participant: %w", err)
	}
	_ = s.rebuildHonors(ctx, time.Time{})
	_ = s.InvalidateCache(ctx)
	status := s.participantStatus(participant)
	return &status, nil
}

func (s *LeaderboardService) BanParticipant(ctx context.Context, userID int64) (*LeaderboardParticipantStatus, error) {
	return s.SetParticipantBanStatus(ctx, userID, true)
}

func (s *LeaderboardService) UnbanParticipant(ctx context.Context, userID int64) (*LeaderboardParticipantStatus, error) {
	return s.SetParticipantBanStatus(ctx, userID, false)
}

func (s *LeaderboardService) RemoveParticipant(ctx context.Context, userID int64) (*LeaderboardParticipantStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}

	participant, err := s.repo.GetParticipant(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard participant: %w", err)
	}

	code := ""
	if participant != nil {
		code = participant.DisplayCode
	}
	if strings.TrimSpace(code) == "" {
		code, err = generateLeaderboardDisplayCode()
		if err != nil {
			return nil, fmt.Errorf("generate leaderboard display code: %w", err)
		}
	}

	participant, err = s.repo.RemoveParticipant(ctx, LeaderboardParticipantRemove{
		UserID:      userID,
		DisplayCode: code,
		Now:         time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("remove leaderboard participant: %w", err)
	}
	_ = s.rebuildHonors(ctx, time.Time{})
	_ = s.InvalidateCache(ctx)
	status := s.participantStatus(participant)
	return &status, nil
}

func (s *LeaderboardService) SetParticipantBanStatus(ctx context.Context, userID int64, isBanned bool) (*LeaderboardParticipantStatus, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLeaderboardUnavailable
	}

	participant, err := s.repo.GetParticipant(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard participant: %w", err)
	}

	code := ""
	if participant != nil {
		code = participant.DisplayCode
	}
	if strings.TrimSpace(code) == "" {
		code, err = generateLeaderboardDisplayCode()
		if err != nil {
			return nil, fmt.Errorf("generate leaderboard display code: %w", err)
		}
	}

	participant, err = s.repo.SetParticipantBanStatus(ctx, LeaderboardParticipantBanUpdate{
		UserID:      userID,
		IsBanned:    isBanned,
		DisplayCode: code,
		Now:         time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("set leaderboard participant ban status: %w", err)
	}
	_ = s.rebuildHonors(ctx, time.Time{})
	_ = s.InvalidateCache(ctx)
	status := s.participantStatus(participant)
	return &status, nil
}

func (s *LeaderboardService) SnapshotRecentlyCompletedPeriods(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return ErrLeaderboardUnavailable
	}
	loc := timezone.Location()
	if !now.IsZero() {
		now = now.In(loc)
	} else {
		now = timezone.Now()
	}
	periods := recentlyCompletedLeaderboardPeriods(now, loc)
	_, err := s.rebuildDailyAggregatesAndSnapshotPeriods(ctx, now.AddDate(0, 0, -2), now, periods)
	return err
}

func (s *LeaderboardService) InvalidateCache(ctx context.Context) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.DeleteAll(ctx)
}

func (s *LeaderboardService) rebuildHonors(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return ErrLeaderboardUnavailable
	}
	if now.IsZero() {
		now = timezone.Now()
	}
	if _, err := s.repo.RebuildHonorStats(ctx, now, latestCompletedLeaderboardStarts(now, timezone.Location())); err != nil {
		return fmt.Errorf("rebuild leaderboard honor stats: %w", err)
	}
	return nil
}

func (s *LeaderboardService) rebuildDailyAggregatesAndSnapshotPeriods(ctx context.Context, since, now time.Time, periods []leaderboardPeriod) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrLeaderboardUnavailable
	}
	loc := timezone.Location()
	if since.IsZero() || now.IsZero() || !since.Before(now) {
		return s.snapshotPeriods(ctx, periods)
	}
	startDate := startOfLeaderboardDay(since, loc)
	endDate := startOfLeaderboardDay(now, loc).AddDate(0, 0, 1)
	if _, err := s.repo.RebuildUsageDaily(ctx, startDate, endDate); err != nil {
		return 0, fmt.Errorf("rebuild leaderboard daily usage %s..%s: %w", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), err)
	}
	inserted, err := s.snapshotPeriods(ctx, periods)
	if err != nil {
		return inserted, err
	}
	if err := s.rebuildHonors(ctx, now); err != nil {
		return inserted, err
	}
	_ = s.InvalidateCache(ctx)
	return inserted, nil
}

func (s *LeaderboardService) snapshotPeriods(ctx context.Context, periods []leaderboardPeriod) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrLeaderboardUnavailable
	}
	var inserted int64
	for _, p := range periods {
		affected, err := s.repo.SnapshotPeriod(ctx, p.window, p.start, p.end, leaderboardTopLimit)
		if err != nil {
			return inserted, fmt.Errorf("snapshot %s leaderboard period %s: %w", p.window, p.start.Format(time.RFC3339), err)
		}
		inserted += affected
	}
	return inserted, nil
}

func (s *LeaderboardService) participantStatus(p *LeaderboardParticipant) LeaderboardParticipantStatus {
	if p == nil {
		return LeaderboardParticipantStatus{}
	}
	return LeaderboardParticipantStatus{
		IsOptedIn:   p.IsOptedIn,
		IsBanned:    p.IsBanned,
		DisplayName: p.DisplayName,
		DisplayCode: p.DisplayCode,
		PublicName:  publicLeaderboardName(p.DisplayName, p.DisplayCode),
		OptedInAt:   p.OptedInAt,
	}
}

func (s *LeaderboardService) publicEntry(row LeaderboardRankRow, currentUserID int64, honors LeaderboardHonorStats) LeaderboardPublicEntry {
	return LeaderboardPublicEntry{
		Rank:           row.Rank,
		DisplayName:    publicLeaderboardName(row.DisplayName, row.DisplayCode),
		AvatarURL:      row.AvatarURL,
		Tokens:         row.Tokens,
		Requests:       row.Requests,
		CurrentStreak:  honors.CurrentStreak,
		ChampionCount:  honors.ChampionCount,
		TopAppearances: honors.TopAppearances,
		BestRank:       honors.BestRank,
		IsMe:           row.UserID == currentUserID,
	}
}

func normalizeLeaderboardDisplayName(raw *string) (string, error) {
	if raw == nil {
		return "", nil
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, *raw)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if cleaned == "" {
		return "", nil
	}
	if strings.ContainsAny(cleaned, "@<>") {
		return "", ErrLeaderboardInvalidDisplayName
	}
	if len([]rune(cleaned)) > leaderboardDisplayNameMaxRunes {
		return "", ErrLeaderboardInvalidDisplayName
	}
	return cleaned, nil
}

func generateLeaderboardDisplayCode() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b[:])), nil
}

func publicLeaderboardName(displayName, displayCode string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	code := strings.TrimSpace(displayCode)
	if code == "" {
		code = "ANON"
	}
	return "用户 #" + code
}

func honorStatsForWindow(honors map[int64]map[string]LeaderboardHonorStats, userID int64, window string) LeaderboardHonorStats {
	if honors == nil {
		return LeaderboardHonorStats{}
	}
	byWindow := honors[userID]
	if byWindow == nil {
		return LeaderboardHonorStats{}
	}
	return byWindow[window]
}

func sortedInt64Keys(values map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func ptrLeaderboardTime(t time.Time) *time.Time {
	return &t
}

func leaderboardOverviewCacheKey(userID int64, now time.Time, loc *time.Location) string {
	day := startOfLeaderboardDay(now, loc)
	week := startOfLeaderboardWeek(now, loc)
	month := startOfLeaderboardMonth(now, loc)
	return fmt.Sprintf(
		"user:%d:day:%s:week:%s:month:%s",
		userID,
		day.Format("2006-01-02"),
		week.Format("2006-01-02"),
		month.Format("2006-01"),
	)
}

func resolveLeaderboardLocation(userTZ string) *time.Location {
	_ = userTZ
	return timezone.Location()
}

func startOfLeaderboardDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func startOfLeaderboardWeek(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return time.Date(t.Year(), t.Month(), t.Day()-weekday+1, 0, 0, 0, 0, loc)
}

func startOfLeaderboardMonth(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func latestCompletedLeaderboardStarts(now time.Time, loc *time.Location) map[string]time.Time {
	dayStart := startOfLeaderboardDay(now, loc)
	weekStart := startOfLeaderboardWeek(now, loc)
	monthStart := startOfLeaderboardMonth(now, loc)
	return map[string]time.Time{
		LeaderboardWindowDaily:   dayStart.AddDate(0, 0, -1),
		LeaderboardWindowWeekly:  weekStart.AddDate(0, 0, -7),
		LeaderboardWindowMonthly: monthStart.AddDate(0, -1, 0),
	}
}

type leaderboardPeriod struct {
	window string
	start  time.Time
	end    time.Time
}

func recentlyCompletedLeaderboardPeriods(now time.Time, loc *time.Location) []leaderboardPeriod {
	dayStart := startOfLeaderboardDay(now, loc)
	weekStart := startOfLeaderboardWeek(now, loc)
	monthStart := startOfLeaderboardMonth(now, loc)

	return []leaderboardPeriod{
		{window: LeaderboardWindowDaily, start: dayStart.AddDate(0, 0, -1), end: dayStart},
		{window: LeaderboardWindowDaily, start: dayStart.AddDate(0, 0, -2), end: dayStart.AddDate(0, 0, -1)},
		{window: LeaderboardWindowWeekly, start: weekStart.AddDate(0, 0, -7), end: weekStart},
		{window: LeaderboardWindowMonthly, start: monthStart.AddDate(0, -1, 0), end: monthStart},
	}
}

func completedLeaderboardPeriodsSince(since, now time.Time, loc *time.Location) []leaderboardPeriod {
	if since.IsZero() || now.IsZero() || !since.Before(now) {
		return nil
	}

	dayStart := startOfLeaderboardDay(since, loc)
	dayEnd := startOfLeaderboardDay(now, loc)
	weekStart := startOfLeaderboardWeek(since, loc)
	weekEnd := startOfLeaderboardWeek(now, loc)
	monthStart := startOfLeaderboardMonth(since, loc)
	monthEnd := startOfLeaderboardMonth(now, loc)

	periods := make([]leaderboardPeriod, 0, 32)
	for cur := dayStart; cur.Before(dayEnd); cur = cur.AddDate(0, 0, 1) {
		periods = append(periods, leaderboardPeriod{
			window: LeaderboardWindowDaily,
			start:  cur,
			end:    cur.AddDate(0, 0, 1),
		})
	}
	for cur := weekStart; cur.Before(weekEnd); cur = cur.AddDate(0, 0, 7) {
		periods = append(periods, leaderboardPeriod{
			window: LeaderboardWindowWeekly,
			start:  cur,
			end:    cur.AddDate(0, 0, 7),
		})
	}
	for cur := monthStart; cur.Before(monthEnd); cur = cur.AddDate(0, 1, 0) {
		periods = append(periods, leaderboardPeriod{
			window: LeaderboardWindowMonthly,
			start:  cur,
			end:    cur.AddDate(0, 1, 0),
		})
	}
	return periods
}

func currentLeaderboardStreak(starts []time.Time, latest time.Time, window string) int {
	if latest.IsZero() || len(starts) == 0 {
		return 0
	}
	seen := make(map[int64]struct{}, len(starts))
	for _, start := range starts {
		seen[start.UTC().Unix()] = struct{}{}
	}
	streak := 0
	cursor := latest
	for {
		if _, ok := seen[cursor.UTC().Unix()]; !ok {
			break
		}
		streak++
		switch window {
		case LeaderboardWindowDaily:
			cursor = cursor.AddDate(0, 0, -1)
		case LeaderboardWindowWeekly:
			cursor = cursor.AddDate(0, 0, -7)
		case LeaderboardWindowMonthly:
			cursor = cursor.AddDate(0, -1, 0)
		default:
			return 0
		}
	}
	return streak
}

type LeaderboardSnapshotService struct {
	leaderboard *LeaderboardService
	interval    time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

func NewLeaderboardSnapshotService(leaderboard *LeaderboardService, interval time.Duration) *LeaderboardSnapshotService {
	return &LeaderboardSnapshotService{
		leaderboard: leaderboard,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (s *LeaderboardSnapshotService) Start() {
	if s == nil || s.leaderboard == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *LeaderboardSnapshotService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *LeaderboardSnapshotService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.leaderboard.SnapshotRecentlyCompletedPeriods(ctx, timezone.Now()); err != nil {
		log.Printf("[LeaderboardSnapshot] snapshot failed: %v", err)
	}
}
