package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

func imageGenerationRequestLogFields(classification service.RequestCapabilityClassification) []zap.Field {
	if !classification.IsImageGeneration {
		return nil
	}
	return []zap.Field{
		zap.String("request_capability", "image_generation"),
		zap.String("image_generation_source", classification.ImageGenerationSource),
	}
}

func selectedImageGenerationAccountLogFields(account *service.Account, classification service.RequestCapabilityClassification) []zap.Field {
	if !classification.IsImageGeneration {
		return nil
	}
	supportsImageGeneration := false
	if account != nil {
		supportsImageGeneration = account.SupportsImageGeneration
	}
	return []zap.Field{
		zap.Bool("selected_account_supports_image_generation", supportsImageGeneration),
	}
}
