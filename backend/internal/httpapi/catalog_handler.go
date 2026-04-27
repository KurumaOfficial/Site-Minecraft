// Автор: Kuruma
package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/service"
)

type CatalogHandler struct {
	service *service.CatalogService
}

func NewCatalogHandler(service *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{service: service}
}

func (h *CatalogHandler) List(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"items": h.service.List(),
	})
}
