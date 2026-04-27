// Автор: Kuruma
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type BrandingConfig struct {
	ProjectName      string
	ServerName       string
	Tagline          string
	SiteURL          string
	AdminRedirectURL string
	SupportEmail     string
	DiscordURL       string
	VKURL            string
	OfferURL         string
	PrivacyURL       string
}

type DatabaseConfig struct {
	Driver      string
	URL         string
	AutoMigrate bool
	SchemaPath  string
}

type SupabaseConfig struct {
	URL            string
	PublishableKey string
	AnonKey        string
	SecretKey      string
	ServiceRoleKey string
}

type HTTPConfig struct {
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	RequestTimeout     time.Duration
	ShutdownTimeout    time.Duration
	Concurrency        int
	BodyLimit          int
	ReadBufferSize     int
	WriteBufferSize    int
	StaticCacheTTL     time.Duration
	StaticMaxAge       int
	EnableRequestLog   bool
	DisableStartupInfo bool
}

type SecurityConfig struct {
	AllowOrigins            string
	EnableTrustedProxyCheck bool
	TrustedProxies          []string
	APIRateLimitMax         int
	APIRateLimitWindow      time.Duration
	OrderRateLimitMax       int
	OrderRateLimitWindow    time.Duration
	AdminRateLimitMax       int
	AdminRateLimitWindow    time.Duration
}

type StorageConfig struct {
	FileSnapshotEvery int
}

type AdminConfig struct {
	AllowedEmails     []string
	AllowedDiscordIDs []string
	AuthCacheTTL      time.Duration
	LocalBypass       bool
}

type Config struct {
	AppName    string
	Host       string
	Port       string
	StaticDir  string
	DataDir    string
	PromoCodes map[string]int
	Branding   BrandingConfig
	Database   DatabaseConfig
	Supabase   SupabaseConfig
	HTTP       HTTPConfig
	Security   SecurityConfig
	Storage    StorageConfig
	Admin      AdminConfig
}

func Load() (Config, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}

	host := getenv("HOST", "0.0.0.0")
	port := getenv("PORT", "8080")
	siteURL := normalizePublicURL(getenv("PUBLIC_SITE_URL", "http://localhost:"+port))
	adminRedirectURL := normalizePublicURL(getenv("ADMIN_REDIRECT_URL", siteURL))

	staticDir := getenv("STATIC_DIR", filepath.Join(workingDir, ".."))
	dataDir := getenv("DATA_DIR", filepath.Join(workingDir, "data"))
	schemaPath := getenv("SCHEMA_PATH", filepath.Join(workingDir, "db", "supabase", "schema.sql"))

	staticDir, err = filepath.Abs(staticDir)
	if err != nil {
		return Config{}, err
	}

	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	schemaPath, err = filepath.Abs(schemaPath)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName:    getenv("APP_NAME", "ESTELAR.SU Store"),
		Host:       host,
		Port:       port,
		StaticDir:  staticDir,
		DataDir:    dataDir,
		PromoCodes: parsePromoCodes(getenv("PROMO_CODES", "")),
		Branding: BrandingConfig{
			ProjectName:  getenv("PROJECT_NAME", "ESTELAR.SU"),
			ServerName:   getenv("SERVER_NAME", "ESTELAR.SU"),
			Tagline:      getenv("PROJECT_TAGLINE", "Официальный магазин привилегий, услуг и донат-валюты сервера ESTELAR.SU."),
			SiteURL:      siteURL,
			AdminRedirectURL: adminRedirectURL,
			SupportEmail: strings.TrimSpace(os.Getenv("SUPPORT_EMAIL")),
			DiscordURL:   defaultURL("DISCORD_URL"),
			VKURL:        defaultURL("VK_URL"),
			OfferURL:     defaultURL("OFFER_URL"),
			PrivacyURL:   defaultURL("PRIVACY_URL"),
		},
		Database: DatabaseConfig{
			Driver:      strings.ToLower(getenv("DB_DRIVER", "auto")),
			URL:         strings.TrimSpace(os.Getenv("DATABASE_URL")),
			AutoMigrate: parseBool(getenv("AUTO_MIGRATE", "true")),
			SchemaPath:  schemaPath,
		},
		Supabase: SupabaseConfig{
			URL:            strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/"),
			PublishableKey: strings.TrimSpace(os.Getenv("SUPABASE_PUBLISHABLE_KEY")),
			AnonKey:        strings.TrimSpace(os.Getenv("SUPABASE_ANON_KEY")),
			SecretKey:      strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY")),
			ServiceRoleKey: strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")),
		},
		HTTP: HTTPConfig{
			ReadTimeout:        parseDuration("READ_TIMEOUT", 10*time.Second),
			WriteTimeout:       parseDuration("WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:        parseDuration("IDLE_TIMEOUT", 30*time.Second),
			RequestTimeout:     parseDuration("REQUEST_TIMEOUT", 5*time.Second),
			ShutdownTimeout:    parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
			Concurrency:        parseInt("CONCURRENCY", 16384),
			BodyLimit:          parseInt("BODY_LIMIT_BYTES", 1*1024*1024),
			ReadBufferSize:     parseInt("READ_BUFFER_SIZE", 4096),
			WriteBufferSize:    parseInt("WRITE_BUFFER_SIZE", 4096),
			StaticCacheTTL:     parseDuration("STATIC_CACHE_TTL", 10*time.Second),
			StaticMaxAge:       parseInt("STATIC_MAX_AGE_SECONDS", 86400),
			EnableRequestLog:   parseBool(getenv("ENABLE_REQUEST_LOG", "true")),
			DisableStartupInfo: parseBool(getenv("DISABLE_STARTUP_INFO", "false")),
		},
		Security: SecurityConfig{
			AllowOrigins:            resolveAllowedOrigins(os.Getenv("CORS_ALLOW_ORIGINS"), siteURL, adminRedirectURL),
			EnableTrustedProxyCheck: parseBool(getenv("ENABLE_TRUSTED_PROXY_CHECK", "false")),
			TrustedProxies:          parseCSV(getenv("TRUSTED_PROXIES", "")),
			APIRateLimitMax:         parseInt("API_RATE_LIMIT_MAX", 300),
			APIRateLimitWindow:      parseDuration("API_RATE_LIMIT_WINDOW", time.Minute),
			OrderRateLimitMax:       parseInt("ORDER_RATE_LIMIT_MAX", 40),
			OrderRateLimitWindow:    parseDuration("ORDER_RATE_LIMIT_WINDOW", time.Minute),
			AdminRateLimitMax:       parseInt("ADMIN_RATE_LIMIT_MAX", 120),
			AdminRateLimitWindow:    parseDuration("ADMIN_RATE_LIMIT_WINDOW", time.Minute),
		},
		Storage: StorageConfig{
			FileSnapshotEvery: parseInt("FILE_SNAPSHOT_EVERY", 32),
		},
		Admin: AdminConfig{
			AllowedEmails:     parseCSV(getenv("ADMIN_ALLOWED_EMAILS", "")),
			AllowedDiscordIDs: parseCSV(getenv("ADMIN_ALLOWED_DISCORD_IDS", "")),
			AuthCacheTTL:      parseDuration("ADMIN_AUTH_CACHE_TTL", 5*time.Minute),
			LocalBypass:       parseBool(getenv("ADMIN_LOCAL_BYPASS", "false")),
		},
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Address() string {
	return c.Host + ":" + c.Port
}

func (c Config) StorageMode() string {
	if c.DatabaseEnabled() {
		return "postgres"
	}

	if c.SupabaseRESTEnabled() {
		return "supabase-rest"
	}

	return "file"
}

func (c Config) DatabaseEnabled() bool {
	if c.Database.URL == "" {
		return false
	}

	switch c.Database.Driver {
	case "", "auto", "postgres":
		return true
	default:
		return false
	}
}

func (c Config) SupabaseRESTEnabled() bool {
	if c.Database.URL != "" {
		return false
	}

	switch c.Database.Driver {
	case "supabase", "supabase-rest":
		return c.Supabase.URL != "" && c.Supabase.ServerKey() != ""
	default:
		return false
	}
}

func (c Config) PersistentStorageEnabled() bool {
	return c.DatabaseEnabled() || c.SupabaseRESTEnabled()
}

func (c Config) DatabaseProvider() string {
	if c.DatabaseEnabled() {
		return "Supabase/Postgres"
	}

	if c.SupabaseRESTEnabled() {
		return "Supabase REST"
	}

	return "Local file storage"
}

func (c Config) SupabaseConfigured() bool {
	return c.Supabase.URL != "" && c.Supabase.ClientKey() != ""
}

func (c Config) AdminPanelEnabled() bool {
	return c.Supabase.AuthEnabled() && (len(c.Admin.AllowedDiscordIDs) > 0 || len(c.Admin.AllowedEmails) > 0 || (c.Admin.LocalBypass && c.IsLocalDevelopment()))
}

func (c Config) IsLocalDevelopment() bool {
	return isLocalAddress(c.Branding.SiteURL) || isLocalAddress(c.Branding.AdminRedirectURL)
}

func (c Config) PublicSupabaseKey() string {
	return c.Supabase.ClientKey()
}

func (c SupabaseConfig) ClientKey() string {
	if c.PublishableKey != "" {
		return c.PublishableKey
	}

	return c.AnonKey
}

func (c SupabaseConfig) ServerKey() string {
	if c.SecretKey != "" {
		return c.SecretKey
	}

	return c.ServiceRoleKey
}

func (c SupabaseConfig) AuthEnabled() bool {
	return c.URL != "" && c.ClientKey() != "" && c.ServerKey() != ""
}

func defaultURL(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "#"
	}

	return value
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func normalizePublicURL(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, "/")
	if value == "" {
		return ""
	}

	return value
}

func isLocalAddress(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "localhost") || strings.Contains(value, "127.0.0.1")
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func parseInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	values := make([]string, 0)
	for _, chunk := range strings.Split(raw, ",") {
		value := strings.TrimSpace(chunk)
		if value == "" {
			continue
		}
		values = append(values, value)
	}

	return values
}

func parsePromoCodes(raw string) map[string]int {
	promoCodes := make(map[string]int)
	if raw == "" {
		return promoCodes
	}

	for _, chunk := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(chunk), ":")
		if len(parts) != 2 {
			continue
		}

		percent, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || percent <= 0 || percent > 100 {
			continue
		}

		code := strings.ToUpper(strings.TrimSpace(parts[0]))
		if code == "" {
			continue
		}

		promoCodes[code] = percent
	}

	return promoCodes
}

func validateConfig(cfg Config) error {
	switch cfg.Database.Driver {
	case "", "auto":
	case "postgres":
		if strings.TrimSpace(cfg.Database.URL) == "" {
			return fmt.Errorf("DATABASE_URL is required when DB_DRIVER=postgres")
		}
	case "supabase", "supabase-rest":
		if strings.TrimSpace(cfg.Supabase.URL) == "" {
			return fmt.Errorf("SUPABASE_URL is required when DB_DRIVER=%s", cfg.Database.Driver)
		}
		if strings.TrimSpace(cfg.Supabase.ServerKey()) == "" {
			return fmt.Errorf("SUPABASE_SECRET_KEY or SUPABASE_SERVICE_ROLE_KEY is required when DB_DRIVER=%s", cfg.Database.Driver)
		}
	default:
		return fmt.Errorf("unsupported DB_DRIVER %q", cfg.Database.Driver)
	}

	if err := validatePublicURL("PUBLIC_SITE_URL", cfg.Branding.SiteURL, cfg.IsLocalDevelopment()); err != nil {
		return err
	}
	if err := validatePublicURL("ADMIN_REDIRECT_URL", cfg.Branding.AdminRedirectURL, cfg.IsLocalDevelopment()); err != nil {
		return err
	}

	if !cfg.IsLocalDevelopment() && !cfg.PersistentStorageEnabled() {
		return fmt.Errorf("persistent storage must be configured outside local development")
	}

	if cfg.Admin.LocalBypass && !cfg.IsLocalDevelopment() {
		return fmt.Errorf("ADMIN_LOCAL_BYPASS can only be enabled for local PUBLIC_SITE_URL/ADMIN_REDIRECT_URL")
	}

	if strings.TrimSpace(cfg.Security.AllowOrigins) == "" {
		return fmt.Errorf("at least one CORS origin must be configured")
	}

	return nil
}

func validatePublicURL(name, value string, localAllowed bool) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}

	if parsed.Scheme != "https" && !(localAllowed && parsed.Scheme == "http") {
		return fmt.Errorf("%s must use https outside local development", name)
	}

	return nil
}

func resolveAllowedOrigins(raw string, siteURL string, adminRedirectURL string) string {
	candidates := parseCSV(raw)
	if len(candidates) == 0 {
		candidates = append(candidates, siteURL, adminRedirectURL)
		if siteOrigin := normalizeOrigin(siteURL); siteOrigin != "" {
			if strings.Contains(siteOrigin, "localhost") {
				candidates = append(candidates, strings.Replace(siteOrigin, "localhost", "127.0.0.1", 1))
			} else if strings.Contains(siteOrigin, "127.0.0.1") {
				candidates = append(candidates, strings.Replace(siteOrigin, "127.0.0.1", "localhost", 1))
			}
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	origins := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		origin := normalizeOrigin(candidate)
		if origin == "" {
			continue
		}
		if _, found := seen[origin]; found {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	return strings.Join(origins, ",")
}

func normalizeOrigin(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}
