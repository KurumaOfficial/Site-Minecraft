// Автор: Kuruma
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/config"
	"holo-site-backend/internal/domain"
)

type AdminAuthService struct {
	cfg    config.Config
	client *http.Client

	mu    sync.RWMutex
	cache map[string]cachedAdminIdentity
}

type cachedAdminIdentity struct {
	identity  domain.AdminIdentity
	expiresAt time.Time
}

type supabaseUserResponse struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	AppMeta struct {
		Provider  string   `json:"provider"`
		Providers []string `json:"providers"`
	} `json:"app_metadata"`
	UserMeta struct {
		FullName  string `json:"full_name"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		UserName  string `json:"user_name"`
		Username  string `json:"preferred_username"`
	} `json:"user_metadata"`
}

func NewAdminAuthService(cfg config.Config) *AdminAuthService {
	return &AdminAuthService{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]cachedAdminIdentity),
	}
}

func (s *AdminAuthService) ValidateToken(ctx context.Context, rawToken string) (domain.AdminIdentity, error) {
	// Локальный bypass — только для разработки и только при явном
	// ADMIN_LOCAL_BYPASS=true + локальный URL. Применяется раньше Supabase,
	// чтобы можно было тестировать админку без живой Discord-сессии.
	if s.cfg.Admin.LocalBypass && s.cfg.IsLocalDevelopment() {
		return domain.AdminIdentity{
			ID:       "local-admin",
			Email:    "local@admin.dev",
			Name:     "Локальный администратор",
			Provider: "local-bypass",
		}, nil
	}

	if !s.cfg.Supabase.AuthEnabled() {
		return domain.AdminIdentity{}, domain.NewForbidden("Supabase Auth еще не настроен.")
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return domain.AdminIdentity{}, domain.NewUnauthorized("Требуется авторизация администратора.")
	}

	if identity, ok := s.getCached(token); ok {
		return identity, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.Supabase.URL+"/auth/v1/user", nil)
	if err != nil {
		return domain.AdminIdentity{}, domain.NewInternal("Не удалось подготовить запрос авторизации.")
	}

	request.Header.Set("apikey", s.cfg.Supabase.ServerKey())
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "estelar-backend/1.0")

	response, err := s.client.Do(request)
	if err != nil {
		return domain.AdminIdentity{}, domain.NewInternal("Не удалось проверить сессию администратора.")
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return domain.AdminIdentity{}, domain.NewUnauthorized("Сессия администратора истекла или недействительна.")
	}

	if response.StatusCode >= http.StatusBadRequest {
		return domain.AdminIdentity{}, domain.NewInternal("Supabase Auth временно недоступен.")
	}

	var payload supabaseUserResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return domain.AdminIdentity{}, domain.NewInternal("Не удалось прочитать ответ авторизации.")
	}

	provider := strings.ToLower(strings.TrimSpace(payload.AppMeta.Provider))
	if provider == "" && len(payload.AppMeta.Providers) > 0 {
		provider = strings.ToLower(strings.TrimSpace(payload.AppMeta.Providers[0]))
	}
	if provider != "discord" {
		return domain.AdminIdentity{}, domain.NewForbidden("Вход в админ-панель разрешен только через Discord.")
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if !s.isAllowedAdmin(payload.ID, email) {
		return domain.AdminIdentity{}, domain.NewForbidden("Этот Discord-аккаунт не допущен к админ-панели.")
	}

	name := strings.TrimSpace(payload.UserMeta.FullName)
	if name == "" {
		name = strings.TrimSpace(payload.UserMeta.Name)
	}
	if name == "" {
		name = strings.TrimSpace(payload.UserMeta.UserName)
	}
	if name == "" {
		name = strings.TrimSpace(payload.UserMeta.Username)
	}
	if name == "" {
		name = email
	}
	if name == "" {
		name = strings.TrimSpace(payload.ID)
	}

	identity := domain.AdminIdentity{
		ID:        payload.ID,
		Email:     email,
		Name:      name,
		AvatarURL: strings.TrimSpace(payload.UserMeta.AvatarURL),
		Provider:  provider,
	}

	s.setCached(token, identity)
	return identity, nil
}

func (s *AdminAuthService) isAllowedEmail(email string) bool {
	if email == "" {
		return false
	}

	for _, allowed := range s.cfg.Admin.AllowedEmails {
		if strings.EqualFold(strings.TrimSpace(allowed), email) {
			return true
		}
	}

	return false
}

func (s *AdminAuthService) isAllowedDiscordID(discordID string) bool {
	discordID = strings.TrimSpace(discordID)
	if discordID == "" {
		return false
	}

	for _, allowed := range s.cfg.Admin.AllowedDiscordIDs {
		if strings.TrimSpace(allowed) == discordID {
			return true
		}
	}

	return false
}

func (s *AdminAuthService) isAllowedAdmin(discordID, email string) bool {
	if s.isAllowedDiscordID(discordID) {
		return true
	}

	if s.isAllowedEmail(email) && len(s.cfg.Admin.AllowedDiscordIDs) == 0 {
		return true
	}

	// Local bypass is opt-in through env and must never activate on non-local URLs.
	return s.cfg.Admin.LocalBypass && s.cfg.IsLocalDevelopment()
}

func (s *AdminAuthService) getCached(token string) (domain.AdminIdentity, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, found := s.cache[token]
	if !found || time.Now().UTC().After(entry.expiresAt) {
		return domain.AdminIdentity{}, false
	}

	return entry.identity, true
}

func (s *AdminAuthService) setCached(token string, identity domain.AdminIdentity) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[token] = cachedAdminIdentity{
		identity:  identity,
		expiresAt: time.Now().UTC().Add(s.cfg.Admin.AuthCacheTTL),
	}
}
