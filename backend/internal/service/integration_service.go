package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

// IntegrationService управляет конфигурацией исходящих webhook'ов плагина
// Minecraft и платёжного провайдера. На бэкенде он также отвечает за
// отправку webhook'ов при изменении статуса заказа.
type IntegrationService struct {
	repo   repository.IntegrationRepository
	client *http.Client

	mu           sync.RWMutex
	integrations domain.IntegrationsConfig
	payments     domain.PaymentSettings
}

func NewIntegrationService(ctx context.Context, repo repository.IntegrationRepository) (*IntegrationService, error) {
	service := &IntegrationService{
		repo: repo,
		client: &http.Client{
			Timeout: 6 * time.Second,
		},
	}

	if err := service.bootstrap(ctx); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *IntegrationService) bootstrap(ctx context.Context) error {
	bundle, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}

	if bundle.Payments.Provider == "" {
		bundle.Payments.Provider = domain.PaymentProviderManual
	}

	s.integrations = bundle.Integrations
	s.payments = bundle.Payments
	return nil
}

func (s *IntegrationService) Integrations() domain.IntegrationsConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.integrations
}

func (s *IntegrationService) Payments() domain.PaymentSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.payments
}

// SafeIntegrations возвращает копию настроек webhook'ов с замаскированными
// секретами — чтобы не выводить их в открытом виде в админке.
func (s *IntegrationService) SafeIntegrations() domain.IntegrationsConfig {
	cfg := s.Integrations()
	cfg.PrivilegesEndpoint.Token = maskSecret(cfg.PrivilegesEndpoint.Token)
	cfg.CasesEndpoint.Token = maskSecret(cfg.CasesEndpoint.Token)
	cfg.CurrencyEndpoint.Token = maskSecret(cfg.CurrencyEndpoint.Token)
	cfg.WebhookSecret = maskSecret(cfg.WebhookSecret)
	return cfg
}

// SafePayments возвращает платёжные настройки с замаскированными ключами.
func (s *IntegrationService) SafePayments() domain.PaymentSettings {
	p := s.Payments()
	p.SecretKey = maskSecret(p.SecretKey)
	return p
}

func (s *IntegrationService) UpsertIntegrations(ctx context.Context, input domain.IntegrationsInput) (domain.IntegrationsConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := normalizeIntegrations(input, s.integrations)
	if err != nil {
		return domain.IntegrationsConfig{}, err
	}
	cfg.UpdatedAt = time.Now().UTC()

	if err := s.repo.SaveIntegrations(ctx, cfg); err != nil {
		return domain.IntegrationsConfig{}, err
	}

	s.integrations = cfg
	return cfg, nil
}

func (s *IntegrationService) UpsertPayments(ctx context.Context, input domain.PaymentSettingsInput) (domain.PaymentSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := normalizePayments(input, s.payments)
	if err != nil {
		return domain.PaymentSettings{}, err
	}
	settings.UpdatedAt = time.Now().UTC()

	if err := s.repo.SavePayments(ctx, settings); err != nil {
		return domain.PaymentSettings{}, err
	}

	s.payments = settings
	return settings, nil
}

// Test отправляет пробный webhook на сконфигурированный URL — чтобы
// администратор мог проверить связку из админ-панели.
func (s *IntegrationService) Test(ctx context.Context, category string) (domain.IntegrationTestResult, error) {
	endpoint, err := s.endpointForCategory(category)
	if err != nil {
		return domain.IntegrationTestResult{}, err
	}
	if !endpoint.Enabled {
		return domain.IntegrationTestResult{}, domain.NewBadRequest("Эндпоинт выключен в настройках.")
	}
	if strings.TrimSpace(endpoint.URL) == "" {
		return domain.IntegrationTestResult{}, domain.NewBadRequest("URL не указан.")
	}

	payload := map[string]any{
		"event":    "integration.test",
		"category": category,
		"sentAt":   time.Now().UTC(),
	}

	resp, err := s.deliver(ctx, endpoint, payload)
	if err != nil {
		return domain.IntegrationTestResult{
			OK:      false,
			Message: err.Error(),
		}, nil
	}

	return domain.IntegrationTestResult{
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Message:    fmt.Sprintf("Получен ответ %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode)),
	}, nil
}

// DispatchOrderIssued отправляет событие выдачи заказа на соответствующий
// webhook (best-effort: ошибки логируются, но не валят заказ).
func (s *IntegrationService) DispatchOrderIssued(ctx context.Context, order domain.Order) {
	endpoint, err := s.endpointForCategory(order.Category)
	if err != nil || !endpoint.Enabled || strings.TrimSpace(endpoint.URL) == "" {
		return
	}

	payload := map[string]any{
		"event":         "order.issued",
		"orderId":       order.ID,
		"productSlug":   order.ProductSlug,
		"productName":   order.ProductName,
		"category":      order.Category,
		"categoryLabel": order.CategoryLabel,
		"nickname":      order.Nickname,
		"periodCode":    order.PeriodCode,
		"periodLabel":   order.PeriodLabel,
		"quantity":      order.Quantity,
		"finalPrice":    order.FinalPrice,
		"currency":      order.Currency,
		"handledBy":     order.HandledBy,
		"createdAt":     order.CreatedAt,
		"sentAt":        time.Now().UTC(),
	}

	// Best effort: ошибки игнорируем, ручная очередь остаётся
	// источником истины.
	_, _ = s.deliver(ctx, endpoint, payload)
}

func (s *IntegrationService) endpointForCategory(category string) (domain.IntegrationEndpoint, error) {
	cfg := s.Integrations()
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "privilege":
		return cfg.PrivilegesEndpoint, nil
	case "case":
		return cfg.CasesEndpoint, nil
	case "currency":
		return cfg.CurrencyEndpoint, nil
	default:
		return domain.IntegrationEndpoint{}, domain.NewBadRequest("Неизвестная категория интеграции.")
	}
}

func (s *IntegrationService) deliver(ctx context.Context, endpoint domain.IntegrationEndpoint, payload map[string]any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "estelar-backend/1.0")

	if token := strings.TrimSpace(endpoint.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if secret := strings.TrimSpace(s.Integrations().WebhookSecret); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		req.Header.Set("X-ESTELAR-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	// Прочитываем тело, чтобы соединение можно было переиспользовать.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp, nil
}

// InitPayment возвращает заготовку платежа. Реальные провайдеры будут
// подключены позже — сейчас все запросы возвращают ручной режим.
func (s *IntegrationService) InitPayment(order domain.Order) domain.PaymentInitResponse {
	provider := s.Payments().Provider
	if provider == "" {
		provider = domain.PaymentProviderManual
	}

	if provider == domain.PaymentProviderManual {
		return domain.PaymentInitResponse{
			Provider: provider,
			OrderID:  order.ID,
			Manual:   true,
			Message:  "Ручная выдача: заявка попадет в очередь администратора.",
		}
	}

	return domain.PaymentInitResponse{
		Provider:    provider,
		OrderID:     order.ID,
		Manual:      true,
		Message:     fmt.Sprintf("Платёжный провайдер %q пока в режиме заготовки. Заявка будет обработана вручную.", provider),
		RedirectURL: "",
	}
}

func normalizeIntegrations(input domain.IntegrationsInput, current domain.IntegrationsConfig) (domain.IntegrationsConfig, error) {
	cfg := current

	priv, err := normalizeEndpoint(input.PrivilegesEndpoint, current.PrivilegesEndpoint)
	if err != nil {
		return cfg, fmt.Errorf("привилегии: %w", err)
	}
	cases, err := normalizeEndpoint(input.CasesEndpoint, current.CasesEndpoint)
	if err != nil {
		return cfg, fmt.Errorf("кейсы: %w", err)
	}
	currency, err := normalizeEndpoint(input.CurrencyEndpoint, current.CurrencyEndpoint)
	if err != nil {
		return cfg, fmt.Errorf("валюта: %w", err)
	}

	cfg.PrivilegesEndpoint = priv
	cfg.CasesEndpoint = cases
	cfg.CurrencyEndpoint = currency
	cfg.WebhookSecret = preserveSecret(input.WebhookSecret, current.WebhookSecret)
	return cfg, nil
}

func normalizeEndpoint(input domain.IntegrationEndpoint, current domain.IntegrationEndpoint) (domain.IntegrationEndpoint, error) {
	endpoint := domain.IntegrationEndpoint{
		Enabled: input.Enabled,
	}

	rawURL := strings.TrimSpace(input.URL)
	if rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return endpoint, domain.NewBadRequest("URL должен быть полным http(s)-адресом.")
		}
		endpoint.URL = parsed.String()
	}

	endpoint.Token = preserveSecret(input.Token, current.Token)

	if endpoint.Enabled && endpoint.URL == "" {
		return endpoint, domain.NewBadRequest("Чтобы включить эндпоинт, укажите URL.")
	}
	return endpoint, nil
}

func normalizePayments(input domain.PaymentSettingsInput, current domain.PaymentSettings) (domain.PaymentSettings, error) {
	provider := strings.TrimSpace(strings.ToLower(string(input.Provider)))
	switch domain.PaymentProviderID(provider) {
	case domain.PaymentProviderManual,
		domain.PaymentProviderYooKassa,
		domain.PaymentProviderCloudPayments,
		domain.PaymentProviderLava,
		domain.PaymentProviderTinkoff:
		// ok
	case "":
		provider = string(domain.PaymentProviderManual)
	default:
		return domain.PaymentSettings{}, domain.NewBadRequest("Неизвестный платёжный провайдер.")
	}

	settings := domain.PaymentSettings{
		Provider:    domain.PaymentProviderID(provider),
		TestMode:    input.TestMode,
		ShopID:      strings.TrimSpace(input.ShopID),
		PublicKey:   strings.TrimSpace(input.PublicKey),
		ReturnURL:   strings.TrimSpace(input.ReturnURL),
		WebhookURL:  strings.TrimSpace(input.WebhookURL),
		Description: strings.TrimSpace(input.Description),
	}
	settings.SecretKey = preserveSecret(input.SecretKey, current.SecretKey)

	for _, raw := range []string{settings.ReturnURL, settings.WebhookURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return domain.PaymentSettings{}, domain.NewBadRequest("URL платежной системы должен быть http(s).")
		}
	}

	if settings.Provider != domain.PaymentProviderManual {
		if settings.ShopID == "" {
			return domain.PaymentSettings{}, domain.NewBadRequest("Для выбранного провайдера нужен Shop ID.")
		}
	}

	return settings, nil
}

// preserveSecret позволяет фронту присылать пустую строку (если поле не
// меняется) или маску из звёздочек — старое значение сохраняется. Любое
// другое непустое значение трактуется как новое.
func preserveSecret(input, current string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return current
	}
	if isMaskedSecret(trimmed) {
		return current
	}
	return trimmed
}

func isMaskedSecret(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch != '•' && ch != '*' {
			return false
		}
	}
	return true
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return strings.Repeat("•", 8)
}
