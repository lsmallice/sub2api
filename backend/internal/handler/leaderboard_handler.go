package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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
