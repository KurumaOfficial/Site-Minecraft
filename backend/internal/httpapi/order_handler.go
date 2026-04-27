// Автор: Kuruma
package httpapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/service"
)

type OrderHandler struct {
	service *service.OrderService
	audit   *service.AuditLogService
}

func NewOrderHandler(svc *service.OrderService, audit *service.AuditLogService) *OrderHandler {
	return &OrderHandler{service: svc, audit: audit}
}

func (h *OrderHandler) Quote(c *fiber.Ctx) error {
	var request domain.QuoteRequest
	if err := parseJSONBody(c, &request); err != nil {
		return writeError(c, err)
	}

	quote, err := h.service.Quote(request)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(quote)
}

func (h *OrderHandler) Create(c *fiber.Ctx) error {
	var request domain.CreateOrderRequest
	if err := parseJSONBody(c, &request); err != nil {
		return writeError(c, err)
	}

	order, err := h.service.Create(c.UserContext(), request)
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"order": order,
	})
}

func (h *OrderHandler) Get(c *fiber.Ctx) error {
	order, err := h.service.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(fiber.Map{
		"order": order,
	})
}

func (h *OrderHandler) List(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 250 {
		limit = 250
	}

	orders, err := h.service.List(c.UserContext(), domain.OrderListFilter{
		Status: c.Query("status"),
		Query:  c.Query("query"),
		Limit:  limit,
	})
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(fiber.Map{
		"orders": orders,
	})
}

func (h *OrderHandler) Update(c *fiber.Ctx) error {
	adminIdentity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	handledBy := adminIdentity.Name
	if handledBy == "" {
		handledBy = adminIdentity.Email
	}
	if handledBy == "" {
		handledBy = adminIdentity.ID
	}

	var update domain.OrderStatusUpdate
	if err := parseJSONBody(c, &update); err != nil {
		return writeError(c, err)
	}

	beforeOrder, _ := h.service.GetByID(c.UserContext(), c.Params("id"))
	order, err := h.service.Update(c.UserContext(), c.Params("id"), update, handledBy)
	if err != nil {
		return writeError(c, err)
	}

	h.audit.Record(
		c.UserContext(),
		adminIdentity.ID, adminIdentity.Name,
		"order.update",
		order.ID,
		"Изменён заказ "+order.ID+" → "+order.Status,
		orderSnapshot(beforeOrder),
		orderSnapshot(order),
		c.IP(), c.Get(fiber.HeaderUserAgent),
	)

	return c.JSON(fiber.Map{
		"order": order,
	})
}

func orderSnapshot(o domain.Order) map[string]any {
	if o.ID == "" {
		return nil
	}
	return map[string]any{
		"id":         o.ID,
		"nickname":   o.Nickname,
		"product":    o.ProductSlug,
		"status":     o.Status,
		"finalPrice": o.FinalPrice,
		"adminNote":  o.AdminNote,
		"handledBy":  o.HandledBy,
	}
}
