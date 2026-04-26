package service

import (
	"holo-site-backend/internal/config"
	"holo-site-backend/internal/domain"
)

type MetaService struct {
	cfg      config.Config
	catalog  *CatalogService
	settings *SiteSettingsService
}

func NewMetaService(cfg config.Config, catalog *CatalogService, settings *SiteSettingsService) *MetaService {
	return &MetaService{
		cfg:      cfg,
		catalog:  catalog,
		settings: settings,
	}
}

func (s *MetaService) Get() domain.ProjectMeta {
	items := s.catalog.List()
	settings := s.settings.Get()
	privilegeCount := 0
	caseCount := 0

	for _, item := range items {
		switch item.Category {
		case "privilege":
			privilegeCount++
		case "case":
			caseCount++
		}
	}

	databaseProvider := s.cfg.DatabaseProvider()
	if s.cfg.SupabaseConfigured() && !s.cfg.PersistentStorageEnabled() {
		databaseProvider = "Supabase Auth configured / local data mode"
	}

	return domain.ProjectMeta{
		ProjectName:           s.cfg.Branding.ProjectName,
		ServerName:            s.cfg.Branding.ServerName,
		AppName:               s.cfg.AppName,
		Tagline:               s.cfg.Branding.Tagline,
		SiteURL:               s.cfg.Branding.SiteURL,
		AdminRedirectURL:      s.cfg.Branding.AdminRedirectURL,
		SupportEmail:          settings.SupportEmail,
		DiscordURL:            settings.DiscordURL,
		VKURL:                 settings.VKURL,
		ContactTitle:          settings.ContactTitle,
		DiscordText:           settings.DiscordText,
		DiscordButtonLabel:    settings.DiscordButtonLabel,
		VKText:                settings.VKText,
		VKButtonLabel:         settings.VKButtonLabel,
		SupportText:           settings.SupportText,
		OfferURL:              s.cfg.Branding.OfferURL,
		PrivacyURL:            s.cfg.Branding.PrivacyURL,
		CatalogCount:          len(items),
		PrivilegeCount:        privilegeCount,
		CaseCount:             caseCount,
		StorageMode:           s.cfg.StorageMode(),
		DatabaseProvider:      databaseProvider,
		DatabaseEnabled:       s.cfg.PersistentStorageEnabled(),
		SupabaseReady:         s.cfg.SupabaseConfigured(),
		SupabaseConfigured:    s.cfg.SupabaseConfigured(),
		SupabaseURL:           s.cfg.Supabase.URL,
		SupabasePublicKey:     s.cfg.PublicSupabaseKey(),
		AdminPanelEnabled:     s.cfg.AdminPanelEnabled(),
		DiscordAuthEnabled:    s.cfg.Supabase.AuthEnabled(),
		BackendStatus:         "online",
		BackendStatusLabel:    "Fiber API online",
		PaymentMode:           "manual",
		PaymentModeLabel:      "Заказы сохраняются и попадают в очередь на ручную выдачу администратором.",
		OrderLookupEnabled:    true,
		ManualFulfillmentMode: true,
	}
}
