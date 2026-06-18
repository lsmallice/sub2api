package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func tierSelectionRequestedKey(selection *service.OpenAIAccountTierSelection) string {
	if selection == nil {
		return ""
	}
	return selection.RequestedTierKey
}

func tierSelectionActualKey(selection *service.OpenAIAccountTierSelection) string {
	if selection == nil {
		return ""
	}
	return selection.ActualTierKey
}

func tierSelectionRateMultiplier(selection *service.OpenAIAccountTierSelection) *float64 {
	if selection == nil {
		return nil
	}
	return selection.TierRateMultiplier
}

func (h *OpenAIGatewayHandler) reportOpenAIServiceTierResult(c *gin.Context, apiKey *service.APIKey, selection *service.OpenAIAccountTierSelection, requestedModel string, success bool, firstTokenMs *int) {
	if h == nil || h.gatewayService == nil || c == nil || selection == nil {
		return
	}
	h.gatewayService.ReportOpenAIServiceTierResult(c.Request.Context(), apiKey, selection, requestedModel, success, firstTokenMs)
}

func excludeFailedOpenAIServiceTier(excluded map[string]struct{}, selection *service.OpenAIAccountTierSelection) bool {
	key := tierSelectionActualKey(selection)
	if key == "" {
		return false
	}
	excluded[key] = struct{}{}
	return true
}

func openAIServiceTierSelectionContext(c *gin.Context, excluded map[string]struct{}) context.Context {
	if c == nil || c.Request == nil {
		return context.Background()
	}
	return service.WithExcludedOpenAIServiceTierKeys(c.Request.Context(), excluded)
}
