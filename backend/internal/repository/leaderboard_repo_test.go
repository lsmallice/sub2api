package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLeaderboardRepositoryGetRankingReturnsTopAndMe(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	start := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{"user_id", "rank", "display_name", "display_code", "avatar_url", "tokens", "requests"}).
		AddRow(int64(7), 1, "", "BEEF", "https://cdn.example.com/bob.png", int64(2000), int64(4)).
		AddRow(int64(42), 11, "Alice", "A83F", "https://cdn.example.com/alice.png", int64(500), int64(1))
	mock.ExpectQuery("leaderboard_usage_daily").
		WithArgs(end, start, 10, int64(42)).
		WillReturnRows(rows)

	repo := NewLeaderboardRepository(db)
	top, me, err := repo.GetRanking(context.Background(), service.LeaderboardWindowDaily, start, end, 10, 42)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, top, 1)
	require.Equal(t, int64(7), top[0].UserID)
	require.Equal(t, "BEEF", top[0].DisplayCode)
	require.Equal(t, "https://cdn.example.com/bob.png", top[0].AvatarURL)
	require.NotNil(t, me)
	require.Equal(t, int64(42), me.UserID)
	require.Equal(t, 11, me.Rank)
	require.Equal(t, "Alice", me.DisplayName)
	require.Equal(t, "https://cdn.example.com/alice.png", me.AvatarURL)
}

func TestLeaderboardRepositorySnapshotPeriodIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	start := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM leaderboard_period_results").
		WithArgs(service.LeaderboardWindowDaily, start).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("WITH ranked AS").
		WithArgs(service.LeaderboardWindowDaily, start, end).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	repo := NewLeaderboardRepository(db)
	affected, err := repo.SnapshotPeriod(context.Background(), service.LeaderboardWindowDaily, start, end, 10)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLeaderboardRepositoryRebuildHonorStatsMaterializesCompletedPeriods(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	cutoff := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	latest := map[string]time.Time{
		service.LeaderboardWindowDaily:   time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
		service.LeaderboardWindowWeekly:  time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		service.LeaderboardWindowMonthly: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM leaderboard_user_honors").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("period_end <= \\$1").
		WithArgs(cutoff, latest[service.LeaderboardWindowDaily], latest[service.LeaderboardWindowWeekly], latest[service.LeaderboardWindowMonthly]).
		WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectCommit()

	repo := NewLeaderboardRepository(db)
	affected, err := repo.RebuildHonorStats(context.Background(), cutoff, latest)
	require.NoError(t, err)
	require.Equal(t, int64(4), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLeaderboardRepositoryRebuildHonorStatsSelectsUniquePerennialRunnerUpByRunnerUpCount(t *testing.T) {
	src, err := os.ReadFile("leaderboard_repo.go")
	require.NoError(t, err)
	sql := string(src)

	require.Contains(t, sql, "perennial_runner_up_leaders AS")
	require.Contains(t, sql, "ROW_NUMBER() OVER")
	require.Contains(t, sql, "PARTITION BY h.period_window")
	require.Contains(t, sql, "h.runner_up_count DESC")
	require.Contains(t, sql, "WHERE h.runner_up_count > 0")
	require.Contains(t, sql, "WHERE rn = 1")
	require.NotContains(t, sql, "MAX(longest_runner_up_streak) OVER (PARTITION BY period_window)")
	require.Equal(t, 1, strings.Count(sql, "COALESCE(pru.user_id IS NOT NULL, false)"))
}

func TestLeaderboardRepositoryRebuildHonorStatsMaterializesCurrentRunnerUpStreak(t *testing.T) {
	src, err := os.ReadFile("leaderboard_repo.go")
	require.NoError(t, err)
	sql := string(src)

	require.Contains(t, sql, "current_runner_up_streaks AS")
	require.Contains(t, sql, "COUNT(*)::int AS current_runner_up_streak")
	require.Contains(t, sql, "ORDER BY rup.period_start DESC")
	require.Contains(t, sql, "WHERE period_start = latest_start - ((rn - 1) * step)")
	require.Contains(t, sql, "COALESCE(crus.current_runner_up_streak, 0)")
	require.Contains(t, sql, "LEFT JOIN current_runner_up_streaks crus")
}

func TestLeaderboardRepositoryGetHonorStatsReadsMaterializedHonors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"user_id",
		"period_window",
		"top_appearances",
		"champion_count",
		"runner_up_count",
		"third_place_count",
		"best_rank",
		"current_streak",
		"current_runner_up_streak",
		"longest_runner_up_streak",
		"perennial_runner_up",
	}).
		AddRow(int64(42), service.LeaderboardWindowMonthly, 3, 1, 2, 1, 1, 0, 2, 4, true)
	mock.ExpectQuery("leaderboard_user_honors").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	repo := NewLeaderboardRepository(db)
	honors, err := repo.GetHonorStats(context.Background(), []int64{42})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, 1, honors[42][service.LeaderboardWindowMonthly].ChampionCount)
	require.Equal(t, 2, honors[42][service.LeaderboardWindowMonthly].RunnerUpCount)
	require.Equal(t, 1, honors[42][service.LeaderboardWindowMonthly].ThirdPlaceCount)
	require.Equal(t, 3, honors[42][service.LeaderboardWindowMonthly].TopAppearances)
	require.Equal(t, 0, honors[42][service.LeaderboardWindowMonthly].CurrentStreak)
	require.Equal(t, 2, honors[42][service.LeaderboardWindowMonthly].CurrentRunnerUpStreak)
	require.Equal(t, 4, honors[42][service.LeaderboardWindowMonthly].LongestRunnerUpStreak)
	require.True(t, honors[42][service.LeaderboardWindowMonthly].PerennialRunnerUp)
}
