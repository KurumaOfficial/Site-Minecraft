package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/service"
)

// IntegrationHandler обслуживает раздел «API» и «Платежи» админ-панели
// (заготовка для автоматической выдачи привилегий и подключения провайдеров
// оплаты).
type IntegrationHandler struct {
	integrations *service.IntegrationService
	orders       *service.OrderService
}

func NewIntegrationHandler(integrations *service.IntegrationService, orders *service.OrderService) *IntegrationHandler {
	return &IntegrationHandler{integrations: integrations, orders: orders}
}

func (h *IntegrationHandler) Get(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"integrations": h.integrations.SafeIntegrations(),
		"payments":     h.integrations.SafePayments(),
		"providers":    paymentProviderCatalog(),
	})
}

func (h *IntegrationHandler) SaveIntegrations(c *fiber.Ctx) error {
	var input domain.IntegrationsInput
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	cfg, err := h.integrations.UpsertIntegrations(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}
	cfg.PrivilegesEndpoint.Token = maskToken(cfg.PrivilegesEndpoint.Token)
	cfg.CasesEndpoint.Token = maskToken(cfg.CasesEndpoint.Token)
	cfg.CurrencyEndpoint.Token = maskToken(cfg.CurrencyEndpoint.Token)
	cfg.WebhookSecret = maskToken(cfg.WebhookSecret)
	return c.JSON(fiber.Map{"integrations": cfg})
}

func (h *IntegrationHandler) SavePayments(c *fiber.Ctx) error {
	var input domain.PaymentSettingsInput
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	settings, err := h.integrations.UpsertPayments(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}
	settings.SecretKey = maskToken(settings.SecretKey)
	return c.JSON(fiber.Map{"payments": settings})
}

func (h *IntegrationHandler) Test(c *fiber.Ctx) error {
	var input domain.IntegrationTestRequest
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	result, err := h.integrations.Test(c.UserContext(), input.Category)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"result": result})
}

func paymentProviderCatalog() []fiber.Map {
	return []fiber.Map{
		{"id": domain.PaymentProviderManual, "label": "Ручная выдача", "ready": true, "description": "Заявка попадает в очередь админ-панели. Подходит для запуска без подключенной платежной системы."},
		{"id": domain.PaymentProviderYooKassa, "label": "ЮKassa", "ready": false, "description": "Заготовка под подключение ЮKassa (Yandex.Pay)."},
		{"id": domain.PaymentProviderCloudPayments, "label": "CloudPayments", "ready": false, "description": "Заготовка под CloudPayments."},
		{"id": domain.PaymentProviderLava, "label": "Lava", "ready": false, "description": "Заготовка под Lava."},
		{"id": domain.PaymentProviderTinkoff, "label": "Tinkoff Pay", "ready": false, "description": "Заготовка под Tinkoff Pay."},
	}
}

func maskToken(value string) string {
	if value == "" {
		return ""
	}
	return "••••••••"
}
