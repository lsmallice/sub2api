package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GroupRateTier struct {
	ID             int64          `json:"id"`
	GroupID        int64          `json:"group_id"`
	TierKey        string         `json:"tier_key"`
	DisplayName    string         `json:"display_name"`
	RateMultiplier float64        `json:"rate_multiplier"`
	Priority       int            `json:"priority"`
	Enabled        bool           `json:"enabled"`
	IsDefault      bool           `json:"is_default"`
	FallbackPolicy map[string]any `json:"fallback_policy"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type GroupRateTierRepository interface {
	ListActiveByGroupID(ctx context.Context, groupID int64) ([]GroupRateTier, error)
}

type GroupRateTierAdminRepository interface {
	GroupRateTierRepository
	ListByGroupID(ctx context.Context, groupID int64) ([]GroupRateTier, error)
	SyncGroupRateTiers(ctx context.Context, groupID int64, tiers []GroupRateTierInput) error
}

type GroupRateTierInput struct {
	TierKey        string         `json:"tier_key"`
	DisplayName    string         `json:"display_name"`
	RateMultiplier float64        `json:"rate_multiplier"`
	Priority       int            `json:"priority"`
	Enabled        bool           `json:"enabled"`
	IsDefault      bool           `json:"is_default"`
	FallbackPolicy map[string]any `json:"fallback_policy"`
}

type GroupTierHealthEvent struct {
	GroupID        int64
	TierKey        string
	ModelKey       string
	Capability     string
	OldState       string
	NewState       string
	Reason         string
	ObservedTTFTMs *int
	SampleCount    int
	Metadata       map[string]any
}

type GroupTierHealthEventRecorder interface {
	RecordGroupTierHealthEvent(ctx context.Context, event GroupTierHealthEvent) error
}

var ErrNoAvailableServiceTierAccounts = errors.New("no available OpenAI accounts for requested service tier")

type openAIServiceTierCandidate struct {
	TierKey        string
	DisplayName    string
	RateMultiplier float64
	MultiTier      bool
	FallbackPolicy map[string]any
}

type OpenAIAccountTierSelection struct {
	Selection          *AccountSelectionResult
	Decision           OpenAIAccountScheduleDecision
	RequestedTierKey   string
	ActualTierKey      string
	TierRateMultiplier *float64
	GroupID            int64
	ModelKey           string
	Capability         string
	TierProbe          bool
	HealthState        string
	FallbackPolicy     map[string]any
}

const (
	openAIServiceTierHealthStateHealthy  = "healthy"
	openAIServiceTierHealthStateDegraded = "degraded"
	openAIServiceTierHealthStateProbing  = "probing"

	defaultOpenAIServiceTierCooldown          = 5 * time.Minute
	defaultOpenAIServiceTierSlowSampleLimit   = 1
	defaultOpenAIServiceTierErrorSampleLimit  = 1
	defaultOpenAIServiceTierRecoverySuccesses = 2
)

type openAIServiceTierHealthState struct {
	mu             sync.Mutex
	state          string
	degradedUntil  time.Time
	slowSamples    int
	errorSamples   int
	probeSuccesses int
	updatedAt      time.Time
}

type openAIServiceTierHealthPolicy struct {
	Enabled               bool
	FirstTokenThresholdMs int
	DegradeAfterSlow      int
	DegradeAfterErrors    int
	Cooldown              time.Duration
	RecoverySuccesses     int
}

type openAIServiceTierExcludedKeysContextKey struct{}

// WithExcludedOpenAIServiceTierKeys returns a context that skips the provided
// tier keys during multi-tier account selection for the current request only.
func WithExcludedOpenAIServiceTierKeys(ctx context.Context, keys map[string]struct{}) context.Context {
	if ctx == nil || len(keys) == 0 {
		return ctx
	}
	normalized := make(map[string]struct{}, len(keys))
	for key := range keys {
		if normalizedKey := normalizeTierKey(key); normalizedKey != "" {
			normalized[normalizedKey] = struct{}{}
		}
	}
	if len(normalized) == 0 {
		return ctx
	}
	return context.WithValue(ctx, openAIServiceTierExcludedKeysContextKey{}, normalized)
}

func openAIServiceTierKeyExcludedFromContext(ctx context.Context, key string) bool {
	if ctx == nil {
		return false
	}
	excluded, ok := ctx.Value(openAIServiceTierExcludedKeysContextKey{}).(map[string]struct{})
	if !ok || len(excluded) == 0 {
		return false
	}
	_, ok = excluded[normalizeTierKey(key)]
	return ok
}

// normalizeTierKey canonicalizes custom service tier keys such as pro, plus, or pro2.
// Empty keys intentionally keep legacy single-tier behavior.
func normalizeTierKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func accountSupportsServiceTier(account *Account, requiredTierKey string) bool {
	requiredTierKey = normalizeTierKey(requiredTierKey)
	if requiredTierKey == "" {
		return true
	}
	return account != nil && normalizeTierKey(account.ServiceTierKey) == requiredTierKey
}

func normalizeJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	normalized := make(map[string]any, len(value))
	for key, item := range value {
		normalized[key] = item
	}
	return normalized
}

func resolveOpenAIServiceTierHealthPolicyFromMap(policyMap map[string]any) openAIServiceTierHealthPolicy {
	policyMap = normalizeJSONMap(policyMap)
	threshold := intFromPolicy(policyMap,
		"first_token_threshold_ms",
		"ttft_threshold_ms",
		"first_token_timeout_ms",
		"ttft_timeout_ms",
	)
	errorLimit := intFromPolicy(policyMap,
		"degrade_after_errors",
		"error_threshold",
		"error_sample_threshold",
	)
	degradeEnabled := boolFromPolicy(policyMap, "degrade_enabled", "health_enabled")
	if threshold > 0 || errorLimit > 0 {
		degradeEnabled = true
	}
	if !degradeEnabled {
		return openAIServiceTierHealthPolicy{}
	}

	slowLimit := intFromPolicy(policyMap, "degrade_after_slow_samples", "slow_sample_threshold", "ttft_sample_threshold")
	if slowLimit <= 0 {
		slowLimit = defaultOpenAIServiceTierSlowSampleLimit
	}
	if errorLimit <= 0 {
		errorLimit = defaultOpenAIServiceTierErrorSampleLimit
	}
	cooldownSeconds := intFromPolicy(policyMap, "cooldown_seconds", "recovery_cooldown_seconds", "probe_after_seconds")
	cooldown := defaultOpenAIServiceTierCooldown
	if cooldownSeconds > 0 {
		cooldown = time.Duration(cooldownSeconds) * time.Second
	}
	recoverySuccesses := intFromPolicy(policyMap, "recovery_successes", "probe_successes", "recover_after_successes")
	if recoverySuccesses <= 0 {
		recoverySuccesses = defaultOpenAIServiceTierRecoverySuccesses
	}
	return openAIServiceTierHealthPolicy{
		Enabled:               true,
		FirstTokenThresholdMs: threshold,
		DegradeAfterSlow:      slowLimit,
		DegradeAfterErrors:    errorLimit,
		Cooldown:              cooldown,
		RecoverySuccesses:     recoverySuccesses,
	}
}

func mergeTierFallbackPolicy(base map[string]any, override map[string]any) map[string]any {
	merged := normalizeJSONMap(base)
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func intFromPolicy(policy map[string]any, keys ...string) int {
	if len(policy) == 0 {
		return 0
	}
	for _, key := range keys {
		value, ok := policy[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float32:
			return int(v)
		case float64:
			return int(v)
		case jsonNumberLike:
			i, _ := strconv.Atoi(v.String())
			return i
		case string:
			i, _ := strconv.Atoi(strings.TrimSpace(v))
			return i
		}
	}
	return 0
}

type jsonNumberLike interface {
	String() string
}

func boolFromPolicy(policy map[string]any, keys ...string) bool {
	if len(policy) == 0 {
		return false
	}
	for _, key := range keys {
		value, ok := policy[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v
		case string:
			parsed, _ := strconv.ParseBool(strings.TrimSpace(v))
			return parsed
		}
	}
	return false
}

func openAIServiceTierHealthModelKey(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

func openAIServiceTierHealthCapabilityKey(requiredTransport OpenAIUpstreamTransport, requiredCapability OpenAIEndpointCapability, requiredImageCapability OpenAIImagesCapability, requireCompact bool) string {
	parts := make([]string, 0, 4)
	if requiredCapability != "" {
		parts = append(parts, string(requiredCapability))
	}
	if requiredImageCapability != "" {
		parts = append(parts, "image:"+string(requiredImageCapability))
	}
	if requiredTransport != "" && requiredTransport != OpenAIUpstreamTransportAny {
		parts = append(parts, "transport:"+string(requiredTransport))
	}
	if requireCompact {
		parts = append(parts, "compact")
	}
	return strings.Join(parts, "|")
}

func openAIServiceTierHealthMapKey(groupID int64, tierKey, modelKey, capability string) string {
	return fmt.Sprintf("%d|%s|%s|%s", groupID, normalizeTierKey(tierKey), openAIServiceTierHealthModelKey(modelKey), strings.TrimSpace(capability))
}

func (s *OpenAIGatewayService) getOpenAIServiceTierHealthState(groupID int64, tierKey, modelKey, capability string) *openAIServiceTierHealthState {
	if s == nil {
		return nil
	}
	key := openAIServiceTierHealthMapKey(groupID, tierKey, modelKey, capability)
	actual, _ := s.openaiServiceTierHealth.LoadOrStore(key, &openAIServiceTierHealthState{
		state:     openAIServiceTierHealthStateHealthy,
		updatedAt: time.Now(),
	})
	state, _ := actual.(*openAIServiceTierHealthState)
	return state
}

func (s *OpenAIGatewayService) prepareOpenAIServiceTierForSelection(
	ctx context.Context,
	groupID int64,
	tierKey string,
	modelKey string,
	capability string,
	apiKey *APIKey,
	fallbackPolicy map[string]any,
) (allowed bool, probe bool, stateName string) {
	policy := resolveOpenAIServiceTierHealthPolicyFromMap(fallbackPolicy)
	if !policy.Enabled {
		return true, false, openAIServiceTierHealthStateHealthy
	}
	state := s.getOpenAIServiceTierHealthState(groupID, tierKey, modelKey, capability)
	if state == nil {
		return true, false, openAIServiceTierHealthStateHealthy
	}
	now := time.Now()
	var event *GroupTierHealthEvent
	state.mu.Lock()
	if state.state == "" {
		state.state = openAIServiceTierHealthStateHealthy
	}
	if state.state == openAIServiceTierHealthStateDegraded {
		if now.Before(state.degradedUntil) {
			stateName = state.state
			state.mu.Unlock()
			return false, false, stateName
		}
		oldState := state.state
		state.state = openAIServiceTierHealthStateProbing
		state.probeSuccesses = 0
		state.errorSamples = 0
		state.slowSamples = 0
		state.updatedAt = now
		stateName = state.state
		event = &GroupTierHealthEvent{
			GroupID:     groupID,
			TierKey:     normalizeTierKey(tierKey),
			ModelKey:    openAIServiceTierHealthModelKey(modelKey),
			Capability:  strings.TrimSpace(capability),
			OldState:    oldState,
			NewState:    state.state,
			Reason:      "cooldown_elapsed",
			SampleCount: 0,
			Metadata: map[string]any{
				"cooldown_seconds": int(policy.Cooldown.Seconds()),
			},
		}
		state.mu.Unlock()
		s.recordOpenAIServiceTierHealthEvent(ctx, event)
		return true, true, stateName
	}
	stateName = state.state
	probe = state.state == openAIServiceTierHealthStateProbing
	state.mu.Unlock()
	return true, probe, stateName
}

func (s *OpenAIGatewayService) ReportOpenAIServiceTierResult(
	ctx context.Context,
	apiKey *APIKey,
	selection *OpenAIAccountTierSelection,
	requestedModel string,
	success bool,
	firstTokenMs *int,
) {
	if s == nil || selection == nil || selection.GroupID == 0 || normalizeTierKey(selection.ActualTierKey) == "" {
		return
	}
	fallbackPolicy := selection.FallbackPolicy
	if fallbackPolicy == nil && apiKey != nil {
		fallbackPolicy = apiKey.TierFallbackPolicy
	}
	policy := resolveOpenAIServiceTierHealthPolicyFromMap(fallbackPolicy)
	if !policy.Enabled {
		return
	}
	modelKey := openAIServiceTierHealthModelKey(selection.ModelKey)
	if modelKey == "" {
		modelKey = openAIServiceTierHealthModelKey(requestedModel)
	}
	capability := strings.TrimSpace(selection.Capability)
	tierKey := normalizeTierKey(selection.ActualTierKey)
	state := s.getOpenAIServiceTierHealthState(selection.GroupID, tierKey, modelKey, capability)
	if state == nil {
		return
	}

	now := time.Now()
	var event *GroupTierHealthEvent
	state.mu.Lock()
	if state.state == "" {
		state.state = openAIServiceTierHealthStateHealthy
	}
	if !success {
		state.errorSamples++
		state.slowSamples = 0
		if state.state == openAIServiceTierHealthStateProbing || state.errorSamples >= policy.DegradeAfterErrors {
			event = transitionOpenAIServiceTierHealthLocked(state, selection.GroupID, tierKey, modelKey, capability, openAIServiceTierHealthStateDegraded, "upstream_error", firstTokenMs, state.errorSamples, now, policy)
		} else {
			state.updatedAt = now
		}
		state.mu.Unlock()
		s.recordOpenAIServiceTierHealthEvent(ctx, event)
		return
	}

	state.errorSamples = 0
	slow := firstTokenMs != nil && policy.FirstTokenThresholdMs > 0 && *firstTokenMs > policy.FirstTokenThresholdMs
	if slow {
		state.slowSamples++
		if state.state == openAIServiceTierHealthStateProbing || state.slowSamples >= policy.DegradeAfterSlow {
			event = transitionOpenAIServiceTierHealthLocked(state, selection.GroupID, tierKey, modelKey, capability, openAIServiceTierHealthStateDegraded, "first_token_slow", firstTokenMs, state.slowSamples, now, policy)
		} else {
			state.updatedAt = now
		}
		state.mu.Unlock()
		s.recordOpenAIServiceTierHealthEvent(ctx, event)
		return
	}

	state.slowSamples = 0
	switch state.state {
	case openAIServiceTierHealthStateProbing:
		state.probeSuccesses++
		if state.probeSuccesses >= policy.RecoverySuccesses {
			event = transitionOpenAIServiceTierHealthLocked(state, selection.GroupID, tierKey, modelKey, capability, openAIServiceTierHealthStateHealthy, "probe_success", firstTokenMs, state.probeSuccesses, now, policy)
		} else {
			state.updatedAt = now
		}
	case openAIServiceTierHealthStateHealthy:
		state.updatedAt = now
	}
	state.mu.Unlock()
	s.recordOpenAIServiceTierHealthEvent(ctx, event)
}

func transitionOpenAIServiceTierHealthLocked(
	state *openAIServiceTierHealthState,
	groupID int64,
	tierKey string,
	modelKey string,
	capability string,
	newState string,
	reason string,
	firstTokenMs *int,
	sampleCount int,
	now time.Time,
	policy openAIServiceTierHealthPolicy,
) *GroupTierHealthEvent {
	oldState := state.state
	if oldState == "" {
		oldState = openAIServiceTierHealthStateHealthy
	}
	state.state = newState
	state.updatedAt = now
	switch newState {
	case openAIServiceTierHealthStateDegraded:
		state.degradedUntil = now.Add(policy.Cooldown)
		state.probeSuccesses = 0
	case openAIServiceTierHealthStateHealthy:
		state.degradedUntil = time.Time{}
		state.slowSamples = 0
		state.errorSamples = 0
		state.probeSuccesses = 0
	}
	return &GroupTierHealthEvent{
		GroupID:        groupID,
		TierKey:        normalizeTierKey(tierKey),
		ModelKey:       openAIServiceTierHealthModelKey(modelKey),
		Capability:     strings.TrimSpace(capability),
		OldState:       oldState,
		NewState:       newState,
		Reason:         reason,
		ObservedTTFTMs: firstTokenMs,
		SampleCount:    sampleCount,
		Metadata: map[string]any{
			"first_token_threshold_ms": policy.FirstTokenThresholdMs,
			"degrade_after_slow":       policy.DegradeAfterSlow,
			"degrade_after_errors":     policy.DegradeAfterErrors,
			"cooldown_seconds":         int(policy.Cooldown.Seconds()),
			"recovery_successes":       policy.RecoverySuccesses,
			"degraded_until":           state.degradedUntil.UTC().Format(time.RFC3339Nano),
		},
	}
}

func (s *OpenAIGatewayService) recordOpenAIServiceTierHealthEvent(ctx context.Context, event *GroupTierHealthEvent) {
	if s == nil || event == nil || s.groupRateTierRepo == nil {
		return
	}
	recorder, ok := s.groupRateTierRepo.(GroupTierHealthEventRecorder)
	if !ok {
		return
	}
	if err := recorder.RecordGroupTierHealthEvent(ctx, *event); err != nil {
		// Health audit should never break routing.
		slog.Warn("openai_service_tier_health.record_event_failed", "group_id", event.GroupID, "tier_key", event.TierKey, "error", err)
	}
}

func (s *OpenAIGatewayService) resolveOpenAIServiceTierCandidates(ctx context.Context, apiKey *APIKey) ([]openAIServiceTierCandidate, string, error) {
	if s == nil || s.groupRateTierRepo == nil || apiKey == nil || apiKey.GroupID == nil {
		return legacyOpenAIServiceTierCandidates(), "", nil
	}
	tiers, err := s.groupRateTierRepo.ListActiveByGroupID(ctx, *apiKey.GroupID)
	if err != nil {
		return nil, "", err
	}
	if len(tiers) == 0 {
		return legacyOpenAIServiceTierCandidates(), "", nil
	}

	tierByKey := make(map[string]GroupRateTier, len(tiers))
	orderedKeys := make([]string, 0, len(tiers))
	var defaultKey string
	for _, tier := range tiers {
		tier.TierKey = normalizeTierKey(tier.TierKey)
		if tier.TierKey == "" {
			continue
		}
		if _, exists := tierByKey[tier.TierKey]; exists {
			continue
		}
		tierByKey[tier.TierKey] = tier
		orderedKeys = append(orderedKeys, tier.TierKey)
		if tier.IsDefault && defaultKey == "" {
			defaultKey = tier.TierKey
		}
	}
	if len(orderedKeys) == 0 {
		return legacyOpenAIServiceTierCandidates(), "", nil
	}

	requestedKey := normalizeTierKey(apiKey.PreferredTierKey)
	if _, ok := tierByKey[requestedKey]; requestedKey == "" || !ok {
		requestedKey = defaultKey
	}
	if requestedKey == "" {
		requestedKey = orderedKeys[0]
	}
	effectiveFallbackPolicy := map[string]any{}
	if requestedTier, ok := tierByKey[requestedKey]; ok {
		effectiveFallbackPolicy = mergeTierFallbackPolicy(requestedTier.FallbackPolicy, nil)
	}
	effectiveFallbackPolicy = mergeTierFallbackPolicy(effectiveFallbackPolicy, apiKey.TierFallbackPolicy)
	effectiveFallbackPolicy = sanitizeTierFallbackPolicy(effectiveFallbackPolicy, requestedKey, tierByKey)

	candidateKeys := []string{requestedKey}
	if apiKey.TierFallbackEnabled {
		for _, key := range tierFallbackPolicyOrder(effectiveFallbackPolicy) {
			key = normalizeTierKey(key)
			if key == "" || key == requestedKey {
				continue
			}
			if _, ok := tierByKey[key]; ok && !containsTierKey(candidateKeys, key) {
				candidateKeys = append(candidateKeys, key)
			}
		}
		for _, key := range orderedKeys {
			if key == "" || containsTierKey(candidateKeys, key) {
				continue
			}
			candidateKeys = append(candidateKeys, key)
		}
	}

	candidates := make([]openAIServiceTierCandidate, 0, len(candidateKeys))
	for _, key := range candidateKeys {
		if openAIServiceTierKeyExcludedFromContext(ctx, key) {
			continue
		}
		tier, ok := tierByKey[key]
		if !ok {
			continue
		}
		rateMultiplier := tier.RateMultiplier
		if rateMultiplier < 0 {
			rateMultiplier = 0
		}
		candidates = append(candidates, openAIServiceTierCandidate{
			TierKey:        key,
			DisplayName:    tier.DisplayName,
			RateMultiplier: rateMultiplier,
			MultiTier:      true,
			FallbackPolicy: effectiveFallbackPolicy,
		})
	}
	if len(candidates) == 0 {
		return legacyOpenAIServiceTierCandidates(), "", nil
	}
	return candidates, requestedKey, nil
}

func legacyOpenAIServiceTierCandidates() []openAIServiceTierCandidate {
	return []openAIServiceTierCandidate{{MultiTier: false}}
}

func tierFallbackPolicyOrder(policy map[string]any) []string {
	if len(policy) == 0 {
		return nil
	}
	for _, key := range []string{"fallback_order", "fallback_tiers", "order", "tiers"} {
		if order := stringSliceFromAny(policy[key]); len(order) > 0 {
			return order
		}
	}
	return nil
}

func sanitizeTierFallbackPolicy(policy map[string]any, requestedKey string, tierByKey map[string]GroupRateTier) map[string]any {
	if len(policy) == 0 {
		return policy
	}
	order := tierFallbackPolicyOrder(policy)
	if len(order) == 0 {
		return policy
	}
	requestedKey = normalizeTierKey(requestedKey)
	seen := make(map[string]struct{}, len(order))
	sanitizedOrder := make([]string, 0, len(order))
	for _, key := range order {
		key = normalizeTierKey(key)
		if key == "" || key == requestedKey {
			continue
		}
		if _, ok := tierByKey[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sanitizedOrder = append(sanitizedOrder, key)
	}
	sanitized := normalizeJSONMap(policy)
	if len(sanitizedOrder) == 0 {
		delete(sanitized, "fallback_order")
		delete(sanitized, "fallback_tiers")
		delete(sanitized, "order")
		delete(sanitized, "tiers")
		return sanitized
	}
	sanitized["fallback_order"] = sanitizedOrder
	delete(sanitized, "fallback_tiers")
	delete(sanitized, "order")
	delete(sanitized, "tiers")
	return sanitized
}

func stringSliceFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func containsTierKey(keys []string, key string) bool {
	for _, item := range keys {
		if item == key {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) SelectAccountWithTierRoutingForCapability(
	ctx context.Context,
	apiKey *APIKey,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	platformOverride ...string,
) (*OpenAIAccountTierSelection, error) {
	platform := PlatformOpenAI
	if len(platformOverride) > 0 {
		platform = platformOverride[0]
	}
	return s.selectAccountWithTierRouting(ctx, apiKey, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, requiredCapability, "", requireCompact, platform, false)
}

func (s *OpenAIGatewayService) SelectAccountWithTierRoutingForCapabilityWithMove(
	ctx context.Context,
	apiKey *APIKey,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	previousResponseCanMove bool,
	platformOverride ...string,
) (*OpenAIAccountTierSelection, error) {
	platform := PlatformOpenAI
	if len(platformOverride) > 0 {
		platform = platformOverride[0]
	}
	return s.selectAccountWithTierRouting(ctx, apiKey, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, requiredCapability, "", requireCompact, platform, previousResponseCanMove)
}

func (s *OpenAIGatewayService) SelectAccountWithTierRoutingForImageIntent(
	ctx context.Context,
	apiKey *APIKey,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
	platformOverride ...string,
) (*OpenAIAccountTierSelection, error) {
	platform := PlatformOpenAI
	if len(platformOverride) > 0 {
		platform = platformOverride[0]
	}
	return s.selectAccountWithTierRouting(ctx, apiKey, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, OpenAIEndpointCapabilityChatCompletions, OpenAIImagesCapabilityNative, requireCompact, platform, false)
}

func (s *OpenAIGatewayService) SelectAccountWithTierRoutingForImages(
	ctx context.Context,
	apiKey *APIKey,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIImagesCapability,
) (*OpenAIAccountTierSelection, error) {
	selection, err := s.selectAccountWithTierRouting(ctx, apiKey, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", requiredCapability, false, PlatformOpenAI, false)
	if err == nil && selection != nil && selection.Selection != nil && selection.Selection.Account != nil {
		return selection, nil
	}
	if requiredCapability == OpenAIImagesCapabilityNative {
		return s.selectAccountWithTierRouting(ctx, apiKey, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", OpenAIImagesCapabilityBasic, false, PlatformOpenAI, false)
	}
	return selection, err
}

func (s *OpenAIGatewayService) selectAccountWithTierRouting(
	ctx context.Context,
	apiKey *APIKey,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requiredImageCapability OpenAIImagesCapability,
	requireCompact bool,
	platform string,
	previousResponseCanMove bool,
) (*OpenAIAccountTierSelection, error) {
	var groupID *int64
	if apiKey != nil {
		groupID = apiKey.GroupID
	}
	candidates, requestedTierKey, err := s.resolveOpenAIServiceTierCandidates(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	modelKey := openAIServiceTierHealthModelKey(requestedModel)
	capability := openAIServiceTierHealthCapabilityKey(requiredTransport, requiredCapability, requiredImageCapability, requireCompact)

	var lastErr error
	var lastDecision OpenAIAccountScheduleDecision
	for _, candidate := range candidates {
		healthState := openAIServiceTierHealthStateHealthy
		tierProbe := false
		if groupID != nil && candidate.MultiTier {
			allowed, probe, stateName := s.prepareOpenAIServiceTierForSelection(ctx, *groupID, candidate.TierKey, modelKey, capability, apiKey, candidate.FallbackPolicy)
			healthState = stateName
			tierProbe = probe
			if !allowed {
				continue
			}
		}
		selection, decision, selectErr := s.selectAccountWithScheduler(
			ctx,
			groupID,
			previousResponseID,
			sessionHash,
			requestedModel,
			excludedIDs,
			requiredTransport,
			requiredCapability,
			requiredImageCapability,
			candidate.TierKey,
			requireCompact,
			platform,
			previousResponseCanMove,
		)
		lastDecision = decision
		if selectErr != nil {
			lastErr = selectErr
			continue
		}
		if selection == nil || selection.Account == nil {
			continue
		}
		result := &OpenAIAccountTierSelection{
			Selection:        selection,
			Decision:         decision,
			RequestedTierKey: requestedTierKey,
		}
		if candidate.MultiTier {
			result.ActualTierKey = candidate.TierKey
			multiplier := candidate.RateMultiplier
			result.TierRateMultiplier = &multiplier
			if groupID != nil {
				result.GroupID = *groupID
			}
			result.ModelKey = modelKey
			result.Capability = capability
			result.TierProbe = tierProbe
			result.HealthState = healthState
			result.FallbackPolicy = candidate.FallbackPolicy
		}
		return result, nil
	}
	if lastErr != nil {
		return &OpenAIAccountTierSelection{
			Decision:         lastDecision,
			RequestedTierKey: requestedTierKey,
		}, lastErr
	}
	return &OpenAIAccountTierSelection{
		Decision:         lastDecision,
		RequestedTierKey: requestedTierKey,
	}, ErrNoAvailableServiceTierAccounts
}
