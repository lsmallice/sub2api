package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLeaderboardHandlerOverviewRedactsIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	optedInAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	repo := &handlerLeaderboardRepoStub{
		participant: &service.LeaderboardParticipant{
			UserID:      42,
			IsOptedIn:   true,
			DisplayName: "榜单用户",
			DisplayCode: "A83F",
			OptedInAt:   &optedInAt,
		},
		top: map[string][]service.LeaderboardRankRow{
			service.LeaderboardWindowDaily: {
				{UserID: 42, Rank: 1, DisplayName: "榜单用户", DisplayCode: "A83F", Tokens: 1234, Requests: 2},
			},
		},
		me: map[string]*service.LeaderboardRankRow{
			service.LeaderboardWindowDaily: {UserID: 42, Rank: 1, DisplayName: "榜单用户", DisplayCode: "A83F", Tokens: 1234, Requests: 2},
		},
		honors: map[int64]map[string]service.LeaderboardHonorStats{},
	}
	handler := NewLeaderboardHandler(service.NewLeaderboardService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/leaderboard/overview", handler.GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/leaderboard/overview?timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "user_id")
	require.NotContains(t, body, "email")
	require.NotContains(t, body, "api_key")
	require.Contains(t, body, "榜单用户")
	require.Contains(t, body, "1234")
}

func TestLeaderboardHandlerUpdateMe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &handlerLeaderboardRepoStub{}
	handler := NewLeaderboardHandler(service.NewLeaderboardService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.PUT("/leaderboard/me", handler.UpdateMe)

	req := httptest.NewRequest(http.MethodPut, "/leaderboard/me", strings.NewReader(`{"is_opted_in":true,"display_name":"  Alice  "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.upserted.IsOptedIn)
	require.Equal(t, "Alice", repo.upserted.DisplayName)

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			IsOptedIn  bool   `json:"is_opted_in"`
			PublicName string `json:"public_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)
	require.True(t, envelope.Data.IsOptedIn)
	require.Equal(t, "Alice", envelope.Data.PublicName)
}

func TestLeaderboardHandlerUpdateMeRejectsBannedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &handlerLeaderboardRepoStub{
		participant: &service.LeaderboardParticipant{
			UserID:      42,
			IsBanned:    true,
			DisplayCode: "A83F",
		},
	}
	handler := NewLeaderboardHandler(service.NewLeaderboardService(repo))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.PUT("/leaderboard/me", handler.UpdateMe)

	req := httptest.NewRequest(http.MethodPut, "/leaderboard/me", strings.NewReader(`{"is_opted_in":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "LEADERBOARD_BANNED")
}

func TestLeaderboardHandlerAdminBanParticipant(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &handlerLeaderboardRepoStub{}
	handler := NewLeaderboardHandler(service.NewLeaderboardService(repo))
	router := gin.New()
	router.POST("/admin/users/:id/leaderboard/ban", handler.AdminBanParticipant)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/42/leaderboard/ban", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.banUpdated.IsBanned)
	require.Equal(t, int64(42), repo.banUpdated.UserID)
	require.Contains(t, rec.Body.String(), `"is_banned":true`)
}

type handlerLeaderboardRepoStub struct {
	participant *service.LeaderboardParticipant
	top         map[string][]service.LeaderboardRankRow
	me          map[string]*service.LeaderboardRankRow
	honors      map[int64]map[string]service.LeaderboardHonorStats
	upserted    service.LeaderboardParticipantUpsert
	removed     service.LeaderboardParticipantRemove
	banUpdated  service.LeaderboardParticipantBanUpdate
}

func (r *handlerLeaderboardRepoStub) GetParticipant(_ context.Context, userID int64) (*service.LeaderboardParticipant, error) {
	return r.participant, nil
}

func (r *handlerLeaderboardRepoStub) UpsertParticipant(_ context.Context, input service.LeaderboardParticipantUpsert) (*service.LeaderboardParticipant, error) {
	r.upserted = input
	optedInAt := &input.Now
	if !input.IsOptedIn {
		optedInAt = nil
	}
	return &service.LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   input.IsOptedIn,
		DisplayName: input.DisplayName,
		DisplayCode: "A83F",
		OptedInAt:   optedInAt,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *handlerLeaderboardRepoStub) RemoveParticipant(_ context.Context, input service.LeaderboardParticipantRemove) (*service.LeaderboardParticipant, error) {
	r.removed = input
	return &service.LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   false,
		IsBanned:    false,
		DisplayCode: input.DisplayCode,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *handlerLeaderboardRepoStub) SetParticipantBanStatus(_ context.Context, input service.LeaderboardParticipantBanUpdate) (*service.LeaderboardParticipant, error) {
	r.banUpdated = input
	return &service.LeaderboardParticipant{
		UserID:      input.UserID,
		IsOptedIn:   false,
		IsBanned:    input.IsBanned,
		DisplayCode: input.DisplayCode,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}, nil
}

func (r *handlerLeaderboardRepoStub) GetRanking(_ context.Context, window string, _ time.Time, _ time.Time, _ int, _ int64) ([]service.LeaderboardRankRow, *service.LeaderboardRankRow, error) {
	return r.top[window], r.me[window], nil
}

func (r *handlerLeaderboardRepoStub) GetHonorStats(_ context.Context, userIDs []int64) (map[int64]map[string]service.LeaderboardHonorStats, error) {
	if r.honors == nil {
		return map[int64]map[string]service.LeaderboardHonorStats{}, nil
	}
	return r.honors, nil
}

func (r *handlerLeaderboardRepoStub) RebuildUsageDaily(context.Context, time.Time, time.Time) (int64, error) {
	return 0, nil
}

func (r *handlerLeaderboardRepoStub) RebuildHonorStats(context.Context, time.Time, map[string]time.Time) (int64, error) {
	return 0, nil
}

func (r *handlerLeaderboardRepoStub) SnapshotPeriod(context.Context, string, time.Time, time.Time, int) (int64, error) {
	return 0, nil
}
