package repository

import (
	"context"
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
	mock.ExpectQuery("WITH ranked AS").
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

	mock.ExpectExec("WITH ranked AS").
		WithArgs(service.LeaderboardWindowDaily, start, end, 10).
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := NewLeaderboardRepository(db)
	affected, err := repo.SnapshotPeriod(context.Background(), service.LeaderboardWindowDaily, start, end, 10)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)
	require.NoError(t, mock.ExpectationsWereMet())
}
