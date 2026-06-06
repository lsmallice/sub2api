package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LeaderboardHandler struct {
	leaderboardService *service.LeaderboardService
}

func NewLeaderboardHandler(leaderboardService *service.LeaderboardService) *LeaderboardHandler {
	return &LeaderboardHandler{leaderboardService: leaderboardService}
}

func (h *LeaderboardHandler) GetOverview(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	overview, err := h.leaderboardService.GetOverview(c.Request.Context(), subject.UserID, c.Query("timezone"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

func (h *LeaderboardHandler) GetMe(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	me, err := h.leaderboardService.GetParticipant(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, me)
}

type updateLeaderboardMeRequest struct {
	IsOptedIn   bool    `json:"is_opted_in"`
	DisplayName *string `json:"display_name"`
}

func (h *LeaderboardHandler) UpdateMe(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req updateLeaderboardMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	me, err := h.leaderboardService.UpdateParticipant(c.Request.Context(), subject.UserID, service.UpdateLeaderboardParticipantRequest{
		IsOptedIn:   req.IsOptedIn,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, me)
}

func (h *LeaderboardHandler) AdminRemoveParticipant(c *gin.Context) {
	userID, ok := parseLeaderboardUserID(c)
	if !ok {
		return
	}
	status, err := h.leaderboardService.RemoveParticipant(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *LeaderboardHandler) AdminBanParticipant(c *gin.Context) {
	userID, ok := parseLeaderboardUserID(c)
	if !ok {
		return
	}
	status, err := h.leaderboardService.BanParticipant(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *LeaderboardHandler) AdminUnbanParticipant(c *gin.Context) {
	userID, ok := parseLeaderboardUserID(c)
	if !ok {
		return
	}
	status, err := h.leaderboardService.UnbanParticipant(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

type adminLeaderboardBackfillRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func (h *LeaderboardHandler) AdminBackfillSnapshots(c *gin.Context) {
	var req adminLeaderboardBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	start, err := parseLeaderboardBackfillBoundary(req.Start, false)
	if err != nil {
		response.BadRequest(c, "Invalid start time")
		return
	}

	end := timezone.Now()
	if strings.TrimSpace(req.End) != "" {
		end, err = parseLeaderboardBackfillBoundary(req.End, true)
		if err != nil {
			response.BadRequest(c, "Invalid end time")
			return
		}
	}

	result, err := h.leaderboardService.SnapshotHistoricalPeriodsSince(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseLeaderboardUserID(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return 0, false
	}
	return userID, true
}

func parseLeaderboardBackfillBoundary(raw string, isEnd bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty time")
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, timezone.Location())
	if err != nil {
		return time.Time{}, err
	}
	if isEnd {
		return parsed.Add(24 * time.Hour), nil
	}
	return parsed, nil
}
