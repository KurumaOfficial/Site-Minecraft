package service

import (
	"context"
	"net/mail"
	"net/url"
	"strings"
	"sync"

	"holo-site-backend/internal/config"
	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

type SiteSettingsService struct {
	cfg      config.Config
	repo     repository.SiteSettingsRepository
	mu       sync.RWMutex
	settings domain.SiteSettings
}

func NewSiteSettingsService(ctx context.Context, cfg config.Config, repo repository.SiteSettingsRepository) (*SiteSettingsService, error) {
	service := &SiteSettingsService{
		cfg:  cfg,
		repo: repo,
	}

	if err := service.bootstrap(ctx); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *SiteSettingsService) Get() domain.SiteSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.settings
}

func (s *SiteSettingsService) Upsert(ctx context.Context, input domain.SiteSettingsInput) (domain.SiteSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := normalizeSiteSettings(input, defaultSiteSettings(s.cfg))
	if err != nil {
		return domain.SiteSettings{}, err
	}
	if err := s.repo.Save(ctx, settings); err != nil {
		return domain.SiteSettings{}, err
	}

	s.settings = settings
	return settings, nil
}

func (s *SiteSettingsService) bootstrap(ctx context.Context) error {
	stored, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}

	defaults := defaultSiteSettings(s.cfg)
	merged := mergeSiteSettings(stored, defaults)
	if merged != stored {
		if err := s.repo.Save(ctx, merged); err != nil {
			return err
		}
	}

	s.settings = merged
	return nil
}

func defaultSiteSettings(cfg config.Config) domain.SiteSettings {
	return domain.SiteSettings{
		ContactTitle:       "Остались вопросы? Напиши нам в соц.сетях:",
		DiscordURL:         cfg.Branding.DiscordURL,
		DiscordText:        "А ты уже есть в нашем Discord-сервере?\nСкорее подключайся!",
		DiscordButtonLabel: "Подключиться",
		VKURL:              cfg.Branding.VKURL,
		VKText:             "Наш Telegram-канал: новости, анонсы и\nбыстрые обновления по серверу",
		VKButtonLabel:      "Перейти",
		SupportEmail:       cfg.Branding.SupportEmail,
		SupportText:        "Имеются другие вопросы?\nПиши на нашу почту:",
	}
}

func mergeSiteSettings(current, defaults domain.SiteSettings) domain.SiteSettings {
	if strings.TrimSpace(current.ContactTitle) == "" {
		current.ContactTitle = defaults.ContactTitle
	}
	if strings.TrimSpace(current.DiscordURL) == "" {
		current.DiscordURL = defaults.DiscordURL
	}
	if strings.TrimSpace(current.DiscordText) == "" {
		current.DiscordText = defaults.DiscordText
	}
	if strings.TrimSpace(current.DiscordButtonLabel) == "" {
		current.DiscordButtonLabel = defaults.DiscordButtonLabel
	}
	if strings.TrimSpace(current.VKURL) == "" {
		current.VKURL = defaults.VKURL
	}
	if strings.TrimSpace(current.VKText) == "" {
		current.VKText = defaults.VKText
	}
	if strings.TrimSpace(current.VKButtonLabel) == "" {
		current.VKButtonLabel = defaults.VKButtonLabel
	}
	if strings.TrimSpace(current.SupportEmail) == "" {
		current.SupportEmail = defaults.SupportEmail
	}
	if strings.TrimSpace(current.SupportText) == "" {
		current.SupportText = defaults.SupportText
	}

	return current
}

func normalizeSiteSettings(input domain.SiteSettingsInput, defaults domain.SiteSettings) (domain.SiteSettings, error) {
	settings := mergeSiteSettings(domain.SiteSettings{
		ContactTitle:       strings.TrimSpace(input.ContactTitle),
		DiscordURL:         strings.TrimSpace(input.DiscordURL),
		DiscordText:        strings.TrimSpace(input.DiscordText),
		DiscordButtonLabel: strings.TrimSpace(input.DiscordButtonLabel),
		VKURL:              strings.TrimSpace(input.VKURL),
		VKText:             strings.TrimSpace(input.VKText),
		VKButtonLabel:      strings.TrimSpace(input.VKButtonLabel),
		SupportEmail:       strings.TrimSpace(input.SupportEmail),
		SupportText:        strings.TrimSpace(input.SupportText),
	}, defaults)

	if err := validateSiteSettings(settings); err != nil {
		return domain.SiteSettings{}, err
	}

	return settings, nil
}

func validateSiteSettings(settings domain.SiteSettings) error {
	if err := validateExternalURL(settings.DiscordURL); err != nil {
		return domain.NewBadRequest("Укажите корректную ссылку Discord.")
	}
	if err := validateExternalURL(settings.VKURL); err != nil {
		return domain.NewBadRequest("Укажите корректную ссылку Telegram.")
	}

	email := strings.TrimSpace(settings.SupportEmail)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return domain.NewBadRequest("Укажите корректную почту поддержки.")
		}
	}

	if len(strings.TrimSpace(settings.DiscordButtonLabel)) > 32 || len(strings.TrimSpace(settings.VKButtonLabel)) > 32 {
		return domain.NewBadRequest("Подписи кнопок должны быть короче 32 символов.")
	}

	return nil
}

func validateExternalURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return err
	}

	switch parsed.Scheme {
	case "https", "http":
		return nil
	default:
		return domain.NewBadRequest("invalid URL scheme")
	}
}
