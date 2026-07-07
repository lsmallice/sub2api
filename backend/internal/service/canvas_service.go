package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrCanvasDisabled        = infraerrors.Forbidden("CANVAS_DISABLED", "canvas integration is disabled")
	ErrCanvasTicketInvalid   = infraerrors.Unauthorized("CANVAS_TICKET_INVALID", "canvas sso ticket is invalid")
	ErrCanvasTicketExpired   = infraerrors.Unauthorized("CANVAS_TICKET_EXPIRED", "canvas sso ticket has expired")
	ErrCanvasKeyNotEligible  = infraerrors.Forbidden("CANVAS_KEY_NOT_ELIGIBLE", "api key is not eligible for canvas image generation")
	ErrCanvasInternalAuth    = infraerrors.Unauthorized("CANVAS_INTERNAL_UNAUTHORIZED", "canvas internal token is invalid")
	ErrCanvasNoInternalToken = infraerrors.ServiceUnavailable("CANVAS_INTERNAL_TOKEN_NOT_CONFIGURED", "canvas internal token is not configured")
)

type CanvasImageKey struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	MaskedKey     string     `json:"masked_key"`
	GroupName     string     `json:"group_name,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	Quota         float64    `json:"quota"`
	QuotaUsed     float64    `json:"quota_used"`
	ImageEligible bool       `json:"image_eligible"`
}

type CanvasSSOTicket struct {
	Ticket      string    `json:"ticket"`
	RedirectURL string    `json:"redirect_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type CanvasSSOUser struct {
	ID        int64  `json:"id"`
	Email     string `json:"email,omitempty"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role,omitempty"`
}

type canvasTicketState struct {
	UserID    int64
	ExpiresAt time.Time
	Used      bool
}

type CanvasService struct {
	apiKeyRepo  APIKeyRepository
	userRepo    UserRepository
	accountRepo AccountRepository
	cfg         *config.Config
	mu          sync.Mutex
	tickets     map[string]canvasTicketState
}

func NewCanvasService(apiKeyRepo APIKeyRepository, userRepo UserRepository, accountRepo AccountRepository, cfg *config.Config) *CanvasService {
	return &CanvasService{
		apiKeyRepo:  apiKeyRepo,
		userRepo:    userRepo,
		accountRepo: accountRepo,
		cfg:         cfg,
		tickets:     make(map[string]canvasTicketState),
	}
}

func (s *CanvasService) CreateSSOTicket(ctx context.Context, userID int64) (*CanvasSSOTicket, error) {
	if !s.enabled() {
		return nil, ErrCanvasDisabled
	}
	token, err := randomURLToken(32)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.ticketTTL())
	s.mu.Lock()
	s.cleanupExpiredTicketsLocked(time.Now())
	s.tickets[token] = canvasTicketState{UserID: userID, ExpiresAt: expiresAt}
	s.mu.Unlock()
	return &CanvasSSOTicket{
		Ticket:      token,
		RedirectURL: s.canvasCallbackURL(token),
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *CanvasService) ExchangeSSOTicket(ctx context.Context, ticket string) (*CanvasSSOUser, error) {
	if !s.enabled() {
		return nil, ErrCanvasDisabled
	}
	ticket = strings.TrimSpace(ticket)
	now := time.Now()
	s.mu.Lock()
	state, ok := s.tickets[ticket]
	if ok {
		delete(s.tickets, ticket)
	}
	s.cleanupExpiredTicketsLocked(now)
	s.mu.Unlock()
	if !ok || state.Used {
		return nil, ErrCanvasTicketInvalid
	}
	if !state.ExpiresAt.After(now) {
		return nil, ErrCanvasTicketExpired
	}
	user, err := s.userRepo.GetByID(ctx, state.UserID)
	if err != nil {
		return nil, err
	}
	result := &CanvasSSOUser{ID: user.ID, Email: user.Email, Username: user.Username, Role: user.Role}
	avatar, err := s.userRepo.GetUserAvatar(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if avatar != nil {
		result.AvatarURL = strings.TrimSpace(avatar.URL)
	}
	return result, nil
}

func (s *CanvasService) ListImageKeys(ctx context.Context, userID int64) ([]CanvasImageKey, error) {
	if !s.enabled() {
		return nil, ErrCanvasDisabled
	}
	keys, _, err := s.apiKeyRepo.ListByUserID(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1000, SortBy: "created_at", SortOrder: "desc"}, APIKeyListFilters{})
	if err != nil {
		return nil, err
	}
	items := make([]CanvasImageKey, 0, len(keys))
	for i := range keys {
		key := &keys[i]
		if err := s.checkImageKeyEligibility(ctx, userID, key); err != nil {
			continue
		}
		items = append(items, canvasImageKeyFromAPIKey(key))
	}
	return items, nil
}

func (s *CanvasService) ResolveImageAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	if !s.enabled() {
		return nil, ErrCanvasDisabled
	}
	key, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, err
	}
	if err := s.checkImageKeyEligibility(ctx, userID, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *CanvasService) ValidateInternalToken(token string) error {
	configured := strings.TrimSpace(s.canvasConfig().InternalServiceToken)
	if configured == "" {
		return ErrCanvasNoInternalToken
	}
	if strings.TrimSpace(token) != configured {
		return ErrCanvasInternalAuth
	}
	return nil
}

func (s *CanvasService) checkImageKeyEligibility(ctx context.Context, userID int64, key *APIKey) error {
	if key == nil || key.UserID != userID {
		return ErrCanvasKeyNotEligible
	}
	if !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return ErrCanvasKeyNotEligible
	}
	if !GroupAllowsImageGeneration(key.Group) {
		return ErrCanvasKeyNotEligible
	}
	accounts, err := s.schedulableOpenAIAccounts(ctx, key.GroupID)
	if err != nil {
		return err
	}
	for i := range accounts {
		if accounts[i].SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic) {
			return nil
		}
	}
	return ErrCanvasKeyNotEligible
}

func (s *CanvasService) schedulableOpenAIAccounts(ctx context.Context, groupID *int64) ([]Account, error) {
	if groupID == nil {
		return s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformOpenAI)
	}
	return s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformOpenAI)
}

func (s *CanvasService) enabled() bool {
	return s.canvasConfig().Enabled
}

func (s *CanvasService) ticketTTL() time.Duration {
	seconds := s.canvasConfig().TicketTTLSeconds
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func (s *CanvasService) canvasConfig() config.CanvasConfig {
	if s.cfg == nil {
		return config.CanvasConfig{}
	}
	return s.cfg.Canvas
}

func (s *CanvasService) canvasCallbackURL(ticket string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(s.canvasConfig().BaseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/auth/sub2api/callback?ticket=" + ticket
}

func (s *CanvasService) cleanupExpiredTicketsLocked(now time.Time) {
	for ticket, state := range s.tickets {
		if !state.ExpiresAt.After(now) {
			delete(s.tickets, ticket)
		}
	}
}

func canvasImageKeyFromAPIKey(key *APIKey) CanvasImageKey {
	groupName := ""
	if key.Group != nil {
		groupName = key.Group.Name
	}
	return CanvasImageKey{
		ID:            key.ID,
		Name:          key.Name,
		MaskedKey:     maskAPIKey(key.Key),
		GroupName:     groupName,
		ExpiresAt:     key.ExpiresAt,
		Quota:         key.Quota,
		QuotaUsed:     key.QuotaUsed,
		ImageEligible: true,
	}
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func randomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
