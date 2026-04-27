// Автор: Kuruma
package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/service"
)

type MetaHandler struct {
	service *service.MetaService
}

func NewMetaHandler(service *service.MetaService) *MetaHandler {
	return &MetaHandler{service: service}
}

func (h *MetaHandler) Get(c *fiber.Ctx) error {
	return c.JSON(h.service.Get())
}
