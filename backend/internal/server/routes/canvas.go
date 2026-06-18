package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterCanvasInternalRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	canvas := v1.Group("/internal/canvas")
	{
		canvas.POST("/sso/exchange", h.Canvas.ExchangeSSOTicket)
		canvas.GET("/users/:user_id/image-keys", h.Canvas.ListImageKeysInternal)
		canvas.POST("/api-key/resolve", h.Canvas.ResolveImageAPIKey)
	}
}
