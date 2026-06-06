package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type leaderboardRepoStub struct {
	participant      *LeaderboardParticipant
	top              map[string][]LeaderboardRankRow
	me               map[string]*LeaderboardRankRow
	honors           map[int64]map[string]LeaderboardHonorStats
	upserted         LeaderboardParticipantUpsert
	removed          LeaderboardParticipantRemove
	banUpdated       LeaderboardParticipantBanUpdate
	snapshotAffected int64
	rebuildDailyRows int64
	rebuildHonorRows int64
	rebuildDailyFrom time.Time
	rebuildDailyTo   time.Time
	rebuildHonorAt   time.Time
	snapshotCalls    []leaderboardPeriod
}

func (r *leaderboardRepoStub) GetParticipant(context.Context, int64) (*LeaderboardParticipant, error) {
	return r.participant, nil
}

func (r *leaderboardRepoStub) UpsertParticipant(_ context.Context, input LeaderboardParticipantUpsert) (*LeaderboardParticipant, error) {
	r.upserted = input
	optedInAt := &input.Now
	if !input.IsOptedIn {
		optedInAt = nil
	}
	return &LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   input.IsOptedIn,
		DisplayName: input.DisplayName,
		DisplayCode: firstNonEmpty(input.DisplayCode, "ABCD"),
		OptedInAt:   optedInAt,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *leaderboardRepoStub) RemoveParticipant(_ context.Context, input LeaderboardParticipantRemove) (*LeaderboardParticipant, error) {
	r.removed = input
	return &LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   false,
		IsBanned:    false,
		DisplayCode: firstNonEmpty(input.DisplayCode, "ABCD"),
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *leaderboardRepoStub) SetParticipantBanStatus(_ context.Context, input LeaderboardParticipantBanUpdate) (*LeaderboardParticipant, error) {
	r.banUpdated = input
	return &LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   false,
		IsBanned:    input.IsBanned,
		DisplayCode: firstNonEmpty(input.DisplayCode, "ABCD"),
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *leaderboardRepoStub) GetRanking(_ context.Context, window string, _ time.Time, _ time.Time, _ int, _ int64) ([]LeaderboardRankRow, *LeaderboardRankRow, error) {
	return r.top[window], r.me[window], nil
}

func (r *leaderboardRepoStub) GetHonorStats(_ context.Context, _ []int64) (map[int64]map[string]LeaderboardHonorStats, error) {
	return r.honors, nil
}

func (r *leaderboardRepoStub) RebuildUsageDaily(_ context.Context, startDate, endDate time.Time) (int64, error) {
	r.rebuildDailyFrom = startDate
	r.rebuildDailyTo = endDate
	return r.rebuildDailyRows, nil
}

func (r *leaderboardRepoStub) RebuildHonorStats(_ context.Context, cutoff time.Time, _ map[string]time.Time) (int64, error) {
	r.rebuildHonorAt = cutoff
	return r.rebuildHonorRows, nil
}

func (r *leaderboardRepoStub) SnapshotPeriod(_ context.Context, window string, startTime, endTime time.Time, _ int) (int64, error) {
	r.snapshotCalls = append(r.snapshotCalls, leaderboardPeriod{
		window: window,
		start:  startTime,
		end:    endTime,
	})
	return r.snapshotAffected, nil
}

func TestLeaderboardOverviewRedactsUserIdentityAndHighlightsMe(t *testing.T) {
	optedInAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &leaderboardRepoStub{
		participant: &LeaderboardParticipant{
			UserID:      42,
			IsOptedIn:   true,
			DisplayName: "Alice",
			DisplayCode: "A83F",
			OptedInAt:   &optedInAt,
		},
		top: map[string][]LeaderboardRankRow{
			LeaderboardWindowDaily: {
				{UserID: 7, Rank: 1, DisplayCode: "BEEF", AvatarURL: "https://cdn.example.com/bob.png", Tokens: 200, Requests: 2},
				{UserID: 42, Rank: 2, DisplayName: "Alice", DisplayCode: "A83F", AvatarURL: "https://cdn.example.com/alice.png", Tokens: 100, Requests: 1},
			},
			LeaderboardWindowWeekly: {
				{UserID: 42, Rank: 1, DisplayName: "Alice", DisplayCode: "A83F", AvatarURL: "https://cdn.example.com/alice.png", Tokens: 700, Requests: 7},
			},
		},
		me: map[string]*LeaderboardRankRow{
			LeaderboardWindowDaily:  {UserID: 42, Rank: 2, DisplayName: "Alice", DisplayCode: "A83F", AvatarURL: "https://cdn.example.com/alice.png", Tokens: 100, Requests: 1},
			LeaderboardWindowWeekly: {UserID: 42, Rank: 1, DisplayName: "Alice", DisplayCode: "A83F", AvatarURL: "https://cdn.example.com/alice.png", Tokens: 700, Requests: 7},
		},
		honors: map[int64]map[string]LeaderboardHonorStats{
			42: {
				LeaderboardWindowDaily: {
					TopAppearances:        3,
					ChampionCount:         1,
					RunnerUpCount:         2,
					ThirdPlaceCount:       1,
					BestRank:              1,
					CurrentStreak:         1,
					CurrentRunnerUpStreak: 2,
					LongestRunnerUpStreak: 2,
					PerennialRunnerUp:     true,
					ChampionStarts: map[string][]time.Time{},
				},
				LeaderboardWindowWeekly: {
					TopAppearances: 7,
					ChampionCount:  5,
					BestRank:       1,
					ChampionStarts: map[string][]time.Time{},
				},
			},
		},
	}

	overview, err := NewLeaderboardService(repo).GetOverview(context.Background(), 42, "UTC")
	require.NoError(t, err)
	require.True(t, overview.Participant.IsOptedIn)
	require.Equal(t, "Alice", overview.Participant.PublicName)
	require.Len(t, overview.Daily.Top10, 2)
	require.Equal(t, "用户 #BEEF", overview.Daily.Top10[0].DisplayName)
	require.Equal(t, "https://cdn.example.com/bob.png", overview.Daily.Top10[0].AvatarURL)
	require.Equal(t, "Alice", overview.Daily.Top10[1].DisplayName)
	require.Equal(t, "https://cdn.example.com/alice.png", overview.Daily.Top10[1].AvatarURL)
	require.True(t, overview.Daily.Top10[1].IsMe)
	require.NotNil(t, overview.Daily.Me)
	require.Equal(t, int64(100), overview.Daily.Me.Tokens)
	require.Equal(t, "https://cdn.example.com/alice.png", overview.Daily.Me.AvatarURL)
	require.Equal(t, 1, overview.Daily.Me.ChampionCount)
	require.Equal(t, 2, overview.Daily.Me.RunnerUpCount)
	require.Equal(t, 1, overview.Daily.Me.ThirdPlaceCount)
	require.Equal(t, 3, overview.Daily.Me.TopAppearances)
	require.Equal(t, 0, overview.Daily.Me.CurrentStreak)
	require.Equal(t, 2, overview.Daily.Me.CurrentRunnerUpStreak)
	require.Equal(t, 2, overview.Daily.Me.LongestRunnerUpStreak)
	require.True(t, overview.Daily.Me.PerennialRunnerUp)
	require.NotNil(t, overview.Weekly.Me)
	require.Equal(t, 5, overview.Weekly.Me.ChampionCount)
	require.Equal(t, 7, overview.Weekly.Me.TopAppearances)
}

func TestLeaderboardOverviewHidesMeWhenOptedOut(t *testing.T) {
	repo := &leaderboardRepoStub{
		participant: &LeaderboardParticipant{
			UserID:      42,
			IsOptedIn:   false,
			DisplayCode: "A83F",
		},
		top: map[string][]LeaderboardRankRow{
			LeaderboardWindowDaily: {{UserID: 7, Rank: 1, DisplayCode: "BEEF", Tokens: 200, Requests: 2}},
		},
		me: map[string]*LeaderboardRankRow{
			LeaderboardWindowDaily: {UserID: 42, Rank: 2, DisplayCode: "A83F", Tokens: 100, Requests: 1},
		},
		honors: map[int64]map[string]LeaderboardHonorStats{},
	}

	overview, err := NewLeaderboardService(repo).GetOverview(context.Background(), 42, "UTC")
	require.NoError(t, err)
	require.False(t, overview.Participant.IsOptedIn)
	require.Nil(t, overview.Daily.Me)
}

func TestLeaderboardOverviewOnlyExposesStreakHonorsAboveOne(t *testing.T) {
	repo := &leaderboardRepoStub{
		participant: &LeaderboardParticipant{
			UserID:      42,
			IsOptedIn:   true,
			DisplayCode: "A83F",
		},
		top: map[string][]LeaderboardRankRow{
			LeaderboardWindowDaily: {{UserID: 42, Rank: 1, DisplayCode: "A83F", Tokens: 100, Requests: 1}},
		},
		me: map[string]*LeaderboardRankRow{
			LeaderboardWindowDaily: {UserID: 42, Rank: 1, DisplayCode: "A83F", Tokens: 100, Requests: 1},
		},
		honors: map[int64]map[string]LeaderboardHonorStats{
			42: {
				LeaderboardWindowDaily: {
					ChampionCount:         1,
					RunnerUpCount:         1,
					BestRank:              1,
					CurrentStreak:         1,
					CurrentRunnerUpStreak: 1,
					LongestRunnerUpStreak: 1,
					PerennialRunnerUp:     true,
					ChampionStarts:        map[string][]time.Time{},
				},
			},
		},
	}

	overview, err := NewLeaderboardService(repo).GetOverview(context.Background(), 42, "UTC")
	require.NoError(t, err)
	require.NotNil(t, overview.Daily.Me)
	require.Equal(t, 0, overview.Daily.Me.CurrentStreak)
	require.Equal(t, 0, overview.Daily.Me.CurrentRunnerUpStreak)
	require.Equal(t, 0, overview.Daily.Me.LongestRunnerUpStreak)
	require.True(t, overview.Daily.Me.PerennialRunnerUp)
}

func TestLeaderboardUpdateParticipantValidatesDisplayName(t *testing.T) {
	svc := NewLeaderboardService(&leaderboardRepoStub{})

	bad := "user@example.com"
	_, err := svc.UpdateParticipant(context.Background(), 42, UpdateLeaderboardParticipantRequest{
		IsOptedIn:   true,
		DisplayName: &bad,
	})
	require.ErrorIs(t, err, ErrLeaderboardInvalidDisplayName)

	good := "  榜单 用户  "
	status, err := svc.UpdateParticipant(context.Background(), 42, UpdateLeaderboardParticipantRequest{
		IsOptedIn:   true,
		DisplayName: &good,
	})
	require.NoError(t, err)
	require.True(t, status.IsOptedIn)
	require.Equal(t, "榜单 用户", status.DisplayName)
}

func TestLeaderboardBannedParticipantCannotOptIn(t *testing.T) {
	svc := NewLeaderboardService(&leaderboardRepoStub{
		participant: &LeaderboardParticipant{
			UserID:      42,
			IsOptedIn:   false,
			IsBanned:    true,
			DisplayCode: "A83F",
		},
	})

	_, err := svc.UpdateParticipant(context.Background(), 42, UpdateLeaderboardParticipantRequest{IsOptedIn: true})
	require.ErrorIs(t, err, ErrLeaderboardBanned)
}

func TestLeaderboardBanParticipantOptOutsUser(t *testing.T) {
	repo := &leaderboardRepoStub{}
	status, err := NewLeaderboardService(repo).BanParticipant(context.Background(), 42)

	require.NoError(t, err)
	require.True(t, status.IsBanned)
	require.False(t, status.IsOptedIn)
	require.True(t, repo.banUpdated.IsBanned)
}

func TestLeaderboardRemoveParticipantDoesNotBanUser(t *testing.T) {
	repo := &leaderboardRepoStub{}
	status, err := NewLeaderboardService(repo).RemoveParticipant(context.Background(), 42)

	require.NoError(t, err)
	require.False(t, status.IsBanned)
	require.False(t, status.IsOptedIn)
	require.Equal(t, int64(42), repo.removed.UserID)
}

func TestLeaderboardBackfillSnapshotsSinceCompletedPeriods(t *testing.T) {
	loc := time.Local
	repo := &leaderboardRepoStub{snapshotAffected: 2}
	svc := NewLeaderboardService(repo)
	since := time.Date(2026, 6, 1, 10, 0, 0, 0, loc)
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, loc)

	result, err := svc.SnapshotHistoricalPeriodsSince(context.Background(), since, now)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 5, result.PeriodCount)
	require.Equal(t, int64(10), result.InsertedRows)
	require.Len(t, repo.snapshotCalls, 5)
	require.Equal(t, LeaderboardWindowDaily, repo.snapshotCalls[0].window)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc), repo.snapshotCalls[0].start)
	require.Equal(t, time.Date(2026, 6, 6, 0, 0, 0, 0, loc), repo.snapshotCalls[4].end)
}

func TestLeaderboardBackfillDoesNotSnapshotCurrentMonth(t *testing.T) {
	loc := time.Local
	repo := &leaderboardRepoStub{snapshotAffected: 1}
	svc := NewLeaderboardService(repo)
	since := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, loc)

	result, err := svc.SnapshotHistoricalPeriodsSince(context.Background(), since, now)

	require.NoError(t, err)
	require.NotNil(t, result)
	for _, call := range repo.snapshotCalls {
		require.NotEqual(t, LeaderboardWindowMonthly, call.window, "current month must not be snapshotted before month end")
	}
}

func TestLeaderboardBackfillRejectsInvalidRange(t *testing.T) {
	svc := NewLeaderboardService(&leaderboardRepoStub{})
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.Local)

	_, err := svc.SnapshotHistoricalPeriodsSince(context.Background(), now, now)

	require.ErrorIs(t, err, ErrLeaderboardBackfillInvalidRange)
}

func TestCurrentLeaderboardStreak(t *testing.T) {
	latest := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	require.Equal(t, 3, currentLeaderboardStreak([]time.Time{
		latest,
		latest.AddDate(0, 0, -1),
		latest.AddDate(0, 0, -2),
	}, latest, LeaderboardWindowDaily))
	require.Equal(t, 1, currentLeaderboardStreak([]time.Time{
		latest,
		latest.AddDate(0, 0, -2),
	}, latest, LeaderboardWindowDaily))
	require.Equal(t, 0, currentLeaderboardStreak([]time.Time{
		latest.AddDate(0, 0, -1),
	}, latest, LeaderboardWindowDaily))
}
