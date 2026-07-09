package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	openAIResponsesEndpoint          = "/v1/responses"
	openAIResponsesCompactEndpoint   = "/v1/responses/compact"
	imageGenerationPermissionMessage = "Image generation is not enabled for this group"
	noImageCapableAccountMessage     = "No image-capable OpenAI account is configured for this group"

	ImageGenerationSourceNone           = "none"
	ImageGenerationSourceImagesAPI      = "images_api"
	ImageGenerationSourceResponsesTool  = "responses_tool"
	ImageGenerationSourceChatImageModel = "chat_image_model"
	ImageGenerationSourceChatModalities = "chat_image_modalities"
	ImageGenerationSourceGenericModel   = "image_model"
)

var ErrNoImageCapableAccount = errors.New("no image-capable OpenAI account configured")

type RequestCapabilityClassification struct {
	IsImageGeneration     bool
	ImageGenerationSource string
}

// ImageGenerationPermissionMessage returns the stable end-user error text for disabled groups.
func ImageGenerationPermissionMessage() string {
	return imageGenerationPermissionMessage
}

func NoImageCapableAccountMessage() string {
	return noImageCapableAccountMessage
}

// GroupAllowsImageGeneration preserves ungrouped-key behavior and enforces the flag when a group is present.
func GroupAllowsImageGeneration(group *Group) bool {
	return group == nil || group.AllowImageGeneration
}

// IsImageGenerationIntent classifies requests that can produce generated images.
func IsImageGenerationIntent(endpoint string, requestedModel string, body []byte) bool {
	return ClassifyRequestCapability(endpoint, requestedModel, body).IsImageGeneration
}

func ClassifyRequestCapability(endpoint string, requestedModel string, body []byte) RequestCapabilityClassification {
	if IsImageGenerationEndpoint(endpoint) {
		return imageGenerationClassification(ImageGenerationSourceImagesAPI)
	}
	isChatCompletions := isOpenAIChatCompletionsEndpoint(endpoint)
	if isOpenAIImageGenerationModel(requestedModel) {
		return imageGenerationClassification(imageModelGenerationSource(isChatCompletions))
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return noImageGenerationClassification()
	}
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); isOpenAIImageGenerationModel(model) {
		return imageGenerationClassification(imageModelGenerationSource(isChatCompletions))
	}
	if isChatCompletions && openAIJSONModalitiesContainImage(gjson.GetBytes(body, "modalities")) {
		return imageGenerationClassification(ImageGenerationSourceChatModalities)
	}
	if openAIJSONToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice")) {
		return imageGenerationClassification(ImageGenerationSourceResponsesTool)
	}
	if openAIJSONInputContainsImageGenTool(gjson.GetBytes(body, "input")) {
		return imageGenerationClassification(ImageGenerationSourceResponsesTool)
	}
	return noImageGenerationClassification()
}

// IsImageGenerationIntentMap is the map-backed variant used after service-side request mutation.
func IsImageGenerationIntentMap(endpoint string, requestedModel string, reqBody map[string]any) bool {
	return ClassifyRequestCapabilityMap(endpoint, requestedModel, reqBody).IsImageGeneration
}

func ClassifyRequestCapabilityMap(endpoint string, requestedModel string, reqBody map[string]any) RequestCapabilityClassification {
	if IsImageGenerationEndpoint(endpoint) {
		return imageGenerationClassification(ImageGenerationSourceImagesAPI)
	}
	isChatCompletions := isOpenAIChatCompletionsEndpoint(endpoint)
	if isOpenAIImageGenerationModel(requestedModel) {
		return imageGenerationClassification(imageModelGenerationSource(isChatCompletions))
	}
	if reqBody == nil {
		return noImageGenerationClassification()
	}
	if isOpenAIImageGenerationModel(firstNonEmptyString(reqBody["model"])) {
		return imageGenerationClassification(imageModelGenerationSource(isChatCompletions))
	}
	if isChatCompletions && openAIAnyModalitiesContainImage(reqBody["modalities"]) {
		return imageGenerationClassification(ImageGenerationSourceChatModalities)
	}
	if openAIAnyToolChoiceSelectsImageGeneration(reqBody["tool_choice"]) {
		return imageGenerationClassification(ImageGenerationSourceResponsesTool)
	}
	if openAIAnyToolsContainImageGenNamespace(reqBody["tools"]) {
		return imageGenerationClassification(ImageGenerationSourceResponsesTool)
	}
	if openAIAnyInputContainsImageGenTool(reqBody["input"]) {
		return imageGenerationClassification(ImageGenerationSourceResponsesTool)
	}
	return noImageGenerationClassification()
}

func imageGenerationClassification(source string) RequestCapabilityClassification {
	source = strings.TrimSpace(source)
	if source == "" {
		source = ImageGenerationSourceNone
	}
	return RequestCapabilityClassification{
		IsImageGeneration:     true,
		ImageGenerationSource: source,
	}
}

func noImageGenerationClassification() RequestCapabilityClassification {
	return RequestCapabilityClassification{
		IsImageGeneration:     false,
		ImageGenerationSource: ImageGenerationSourceNone,
	}
}

func imageModelGenerationSource(isChatCompletions bool) string {
	if isChatCompletions {
		return ImageGenerationSourceChatImageModel
	}
	return ImageGenerationSourceGenericModel
}

// IsImageGenerationEndpoint identifies dedicated generated-image endpoints.
func IsImageGenerationEndpoint(endpoint string) bool {
	switch normalizeImageGenerationEndpoint(endpoint) {
	case "/v1/images/generations", "/v1/images/edits", "/images/generations", "/images/edits":
		return true
	default:
		return false
	}
}

func isOpenAIChatCompletionsEndpoint(endpoint string) bool {
	normalized := normalizeImageGenerationEndpoint(endpoint)
	return normalized == "/v1/chat/completions" || normalized == "/chat/completions"
}

func openAIJSONModalitiesContainImage(modalities gjson.Result) bool {
	if !modalities.IsArray() {
		return false
	}
	found := false
	modalities.ForEach(func(_, item gjson.Result) bool {
		if strings.EqualFold(strings.TrimSpace(item.String()), "image") {
			found = true
			return false
		}
		return true
	})
	return found
}

func openAIAnyModalitiesContainImage(modalities any) bool {
	values, ok := modalities.([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(firstNonEmptyString(value)), "image") {
			return true
		}
	}
	return false
}

func openAIAnyToolsContainImageGenNamespace(tools any) bool {
	values, ok := tools.([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		tool, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if isImageGenNamespaceToolMap(tool) {
			return true
		}
	}
	return false
}

func openAIAnyInputContainsImageGenTool(input any) bool {
	items, ok := input.([]any)
	if !ok {
		return false
	}
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if firstNonEmptyString(item["type"]) != "additional_tools" {
			continue
		}
		if openAIAnyToolsContainImageGenNamespace(item["tools"]) {
			return true
		}
	}
	return false
}

func isImageGenNamespaceToolMap(tool map[string]any) bool {
	return firstNonEmptyString(tool["type"]) == "namespace" &&
		firstNonEmptyString(tool["name"]) == "image_gen"
}

func normalizeImageGenerationEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(strings.ToLower(endpoint))
	if endpoint == "" {
		return ""
	}
	endpoint = strings.TrimPrefix(endpoint, "https://api.openai.com")
	if idx := strings.IndexByte(endpoint, '?'); idx >= 0 {
		endpoint = endpoint[:idx]
	}
	return strings.TrimRight(endpoint, "/")
}

func openAIJSONToolsContainImageGeneration(tools gjson.Result) bool {
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, item gjson.Result) bool {
		if openAIJSONString(item.Get("type")) == "image_generation" {
			found = true
			return false
		}
		if isImageGenNamespaceTool(item) {
			found = true
			return false
		}
		return true
	})
	return found
}

// isImageGenNamespaceTool detects the Codex namespace-style image generation
// tool declaration: { "type": "namespace", "name": "image_gen", ... }.
// Codex /image uses this instead of the flat { "type": "image_generation" }.
func isImageGenNamespaceTool(tool gjson.Result) bool {
	return openAIJSONString(tool.Get("type")) == "namespace" &&
		openAIJSONString(tool.Get("name")) == "image_gen"
}

// openAIJSONInputContainsImageGenTool scans Responses input items for
// additional_tools entries that declare the image_gen namespace. This covers
// the "Responses Lite" format where tools are embedded inside input items
// rather than top-level tools.
func openAIJSONInputContainsImageGenTool(input gjson.Result) bool {
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if openAIJSONString(item.Get("type")) != "additional_tools" {
			return true
		}
		tools := item.Get("tools")
		if !tools.IsArray() {
			return true
		}
		tools.ForEach(func(_, tool gjson.Result) bool {
			if isImageGenNamespaceTool(tool) {
				found = true
				return false
			}
			return true
		})
		return !found
	})
	return found
}

func openAIRequestBodyHasImageGenerationTool(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	return openAIJSONToolsContainImageGeneration(gjson.GetBytes(body, "tools"))
}

func stripOpenAIImageGenerationToolsFromBody(body []byte) ([]byte, bool, error) {
	if !openAIRequestBodyHasImageGenerationTool(body) {
		return body, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return nil, false, err
	}
	if !stripOpenAIImageGenerationTools(reqBody) {
		return body, false, nil
	}
	stripped, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, err
	}
	return stripped, true, nil
}

func openAIRequestBodyImageGenerationToolNeedsNormalization(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return false
	}
	needsNormalization := false
	tools.ForEach(func(_, item gjson.Result) bool {
		if openAIJSONString(item.Get("type")) != "image_generation" {
			return true
		}
		// 只有旧字段需要迁移时才进入 map 修改，纯计费读取保持 raw 路径。
		if item.Get("format").Exists() || item.Get("compression").Exists() {
			needsNormalization = true
			return false
		}
		return true
	})
	return needsNormalization
}

func openAIJSONToolChoiceSelectsImageGeneration(choice gjson.Result) bool {
	if !choice.Exists() {
		return false
	}
	if choice.Type == gjson.String {
		return strings.TrimSpace(choice.String()) == "image_generation"
	}
	if !choice.IsObject() {
		return false
	}
	if strings.TrimSpace(choice.Get("type").String()) == "image_generation" {
		return true
	}
	if strings.TrimSpace(choice.Get("tool.type").String()) == "image_generation" {
		return true
	}
	if strings.TrimSpace(choice.Get("function.name").String()) == "image_generation" {
		return true
	}
	return false
}

func openAIAnyToolChoiceSelectsImageGeneration(choice any) bool {
	switch v := choice.(type) {
	case string:
		return strings.TrimSpace(v) == "image_generation"
	case map[string]any:
		if strings.TrimSpace(firstNonEmptyString(v["type"])) == "image_generation" {
			return true
		}
		if tool, ok := v["tool"].(map[string]any); ok && strings.TrimSpace(firstNonEmptyString(tool["type"])) == "image_generation" {
			return true
		}
		if fn, ok := v["function"].(map[string]any); ok && strings.TrimSpace(firstNonEmptyString(fn["name"])) == "image_generation" {
			return true
		}
	}
	return false
}

func getAPIKeyFromContext(c interface{ Get(string) (any, bool) }) *APIKey {
	if c == nil {
		return nil
	}
	v, exists := c.Get("api_key")
	if !exists {
		return nil
	}
	apiKey, _ := v.(*APIKey)
	return apiKey
}

func apiKeyGroup(apiKey *APIKey) *Group {
	if apiKey == nil {
		return nil
	}
	return apiKey.Group
}

type OpenAIResponsesImageBillingConfig struct {
	Model     string
	SizeTier  string
	InputSize string
}

func resolveOpenAIResponsesImageBillingConfigDetailed(reqBody map[string]any, fallbackModel string) (OpenAIResponsesImageBillingConfig, error) {
	imageModel := ""
	imageSize := ""
	hasImageTool := false
	if reqBody != nil {
		rawTools, _ := reqBody["tools"].([]any)
		for _, rawTool := range rawTools {
			toolMap, ok := rawTool.(map[string]any)
			if !ok || strings.TrimSpace(firstNonEmptyString(toolMap["type"])) != "image_generation" {
				continue
			}
			hasImageTool = true
			imageModel = strings.TrimSpace(firstNonEmptyString(toolMap["model"]))
			imageSize = strings.TrimSpace(firstNonEmptyString(toolMap["size"]))
			break
		}
		if imageSize == "" {
			imageSize = strings.TrimSpace(firstNonEmptyString(reqBody["size"]))
		}
	}
	if imageModel == "" && reqBody != nil {
		bodyModel := strings.TrimSpace(firstNonEmptyString(reqBody["model"]))
		if isOpenAIImageBillingModelAlias(bodyModel) || !hasImageTool {
			imageModel = bodyModel
		}
	}
	if imageModel == "" && hasImageTool {
		imageModel = "gpt-image-2"
	}
	if imageModel == "" {
		imageModel = strings.TrimSpace(fallbackModel)
	}
	sizeTier := normalizeOpenAIImageSizeTier(imageSize)
	return OpenAIResponsesImageBillingConfig{
		Model:     imageModel,
		SizeTier:  sizeTier,
		InputSize: imageSize,
	}, nil
}

func resolveOpenAIResponsesImageBillingConfigFromBody(body []byte, fallbackModel string) (string, string, error) {
	cfg, err := resolveOpenAIResponsesImageBillingConfigDetailedFromBody(body, fallbackModel)
	if err != nil {
		return "", "", err
	}
	return cfg.Model, cfg.SizeTier, nil
}

func resolveOpenAIResponsesImageBillingConfigDetailedFromBody(body []byte, fallbackModel string) (OpenAIResponsesImageBillingConfig, error) {
	imageModel := ""
	imageSize := ""
	hasImageTool := false
	if len(body) > 0 && gjson.ValidBytes(body) {
		tools := gjson.GetBytes(body, "tools")
		if tools.IsArray() {
			tools.ForEach(func(_, item gjson.Result) bool {
				if openAIJSONString(item.Get("type")) != "image_generation" {
					return true
				}
				hasImageTool = true
				imageModel = openAIJSONString(item.Get("model"))
				imageSize = openAIJSONString(item.Get("size"))
				return false
			})
		}
		if imageSize == "" {
			imageSize = openAIJSONString(gjson.GetBytes(body, "size"))
		}
		if imageModel == "" {
			bodyModel := openAIJSONString(gjson.GetBytes(body, "model"))
			if isOpenAIImageBillingModelAlias(bodyModel) || !hasImageTool {
				imageModel = bodyModel
			}
		}
	}
	if imageModel == "" && hasImageTool {
		imageModel = "gpt-image-2"
	}
	if imageModel == "" {
		imageModel = strings.TrimSpace(fallbackModel)
	}
	return OpenAIResponsesImageBillingConfig{
		Model:     imageModel,
		SizeTier:  normalizeOpenAIImageSizeTier(imageSize),
		InputSize: imageSize,
	}, nil
}

func isOpenAIImageBillingModelAlias(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return false
	}
	return isOpenAIImageGenerationModel(normalized) || strings.Contains(normalized, "image")
}

func openAIJSONString(value gjson.Result) string {
	if value.Type != gjson.String {
		return ""
	}
	return strings.TrimSpace(value.String())
}
