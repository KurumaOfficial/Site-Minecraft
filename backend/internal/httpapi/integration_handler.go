package httpapi

import (
	"strings"

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

// InitPayment — публичный эндпоинт. Принимает ID существующего заказа и
// возвращает либо инструкцию для ручной очереди, либо redirectUrl на
// форму оплаты внешнего провайдера (например, ЮKassa).
func (h *IntegrationHandler) InitPayment(c *fiber.Ctx) error {
	orderID := c.Params("id")
	order, err := h.orders.GetByID(c.UserContext(), orderID)
	if err != nil {
		return writeError(c, err)
	}

	resp, err := h.integrations.InitPayment(c.UserContext(), order)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(resp)
}

// YooKassaWebhook — endpoint, на который ЮKassa отправляет уведомления о
// смене статуса платежа. Безопасность:
//  1. Получаем тело уведомления, извлекаем payment.id.
//  2. Делаем GET /v3/payments/{id} с Basic Auth (shopId:secretKey) к API
//     ЮKassa и читаем актуальный статус — это надёжнее, чем доверять
//     произвольному телу запроса.
//  3. Если status=succeeded и order_id совпадает — переводим заказ в
//     "issued" и вызываем плагин Minecraft через DispatchOrderIssued.
//  4. Всегда отвечаем 200 OK, даже если заказ уже выдан — иначе ЮKassa
//     будет повторно слать уведомление.
func (h *IntegrationHandler) YooKassaWebhook(c *fiber.Ctx) error {
	var notification struct {
		Event  string `json:"event"`
		Object struct {
			ID       string            `json:"id"`
			Status   string            `json:"status"`
			Metadata map[string]string `json:"metadata"`
		} `json:"object"`
	}
	if err := c.BodyParser(&notification); err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": false, "reason": "bad_body"})
	}

	if notification.Object.ID == "" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": false, "reason": "missing_payment_id"})
	}

	verification, err := h.integrations.FetchYooKassaPayment(c.UserContext(), notification.Object.ID)
	if err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": false, "reason": "fetch_failed"})
	}
	if !verification.Paid {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true, "ignored": true, "status": verification.Status})
	}

	orderID := strings.TrimSpace(verification.OrderID)
	if orderID == "" {
		orderID = strings.TrimSpace(notification.Object.Metadata["order_id"])
	}
	if orderID == "" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": false, "reason": "missing_order_id"})
	}

	if err := h.orders.MarkPaid(c.UserContext(), orderID, verification); err != nil {
		// Не возвращаем 5xx — иначе ЮKassa будет повторять; однако
		// логически это уже не наша вина.
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": false, "reason": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
}

func paymentProviderCatalog() []fiber.Map {
	return []fiber.Map{
		{
			"id":          domain.PaymentProviderManual,
			"label":       "Ручная выдача",
			"ready":       true,
			"description": "Заявка попадает в очередь админ-панели. Подходит для запуска без подключённой платежной системы.",
		},
		{
			"id":          domain.PaymentProviderYooKassa,
			"label":       "ЮKassa",
			"ready":       true,
			"description": "Реальная интеграция с YooKassa (Yandex.Pay): создание платежа, редирект покупателя на форму, webhook с проверкой статуса через API провайдера.",
		},
	}
}

func maskToken(value string) string {
	if value == "" {
		return ""
	}
	return "••••••••"
}
