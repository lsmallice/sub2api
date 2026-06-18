package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CanvasHandler struct {
	canvasService *service.CanvasService
}

func NewCanvasHandler(canvasService *service.CanvasService) *CanvasHandler {
	return &CanvasHandler{canvasService: canvasService}
}

type canvasExchangeTicketRequest struct {
	Ticket string `json:"ticket" binding:"required"`
}

type canvasResolveAPIKeyRequest struct {
	UserID   int64 `json:"user_id" binding:"required"`
	APIKeyID int64 `json:"api_key_id" binding:"required"`
}

type canvasResolveAPIKeyResponse struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
}

func (h *CanvasHandler) CreateSSOTicket(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticket, err := h.canvasService.CreateSSOTicket(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *CanvasHandler) ListImageKeys(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.canvasService.ListImageKeys(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *CanvasHandler) ListImageKeysInternal(c *gin.Context) {
	if !h.authorizeInternal(c) {
		return
	}
	userID, ok := parseCanvasUserID(c.Param("user_id"))
	if !ok {
		response.BadRequest(c, "Invalid user id")
		return
	}
	items, err := h.canvasService.ListImageKeys(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *CanvasHandler) ExchangeSSOTicket(c *gin.Context) {
	if !h.authorizeInternal(c) {
		return
	}
	var req canvasExchangeTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}
	user, err := h.canvasService.ExchangeSSOTicket(c.Request.Context(), req.Ticket)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"user": user})
}

func (h *CanvasHandler) ResolveImageAPIKey(c *gin.Context) {
	if !h.authorizeInternal(c) {
		return
	}
	var req canvasResolveAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 || req.APIKeyID <= 0 {
		response.BadRequest(c, "Invalid request")
		return
	}
	key, err := h.canvasService.ResolveImageAPIKey(c.Request.Context(), req.UserID, req.APIKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, canvasResolveAPIKeyResponse{ID: key.ID, Name: key.Name, APIKey: key.Key})
}

func (h *CanvasHandler) authorizeInternal(c *gin.Context) bool {
	token := strings.TrimSpace(c.GetHeader("X-Canvas-Internal-Token"))
	if token == "" {
		token = bearerToken(c.GetHeader("Authorization"))
	}
	if err := h.canvasService.ValidateInternalToken(token); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseCanvasUserID(value string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return id, err == nil && id > 0
}
