// Автор: Kuruma
package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/service"
)

type AdminHandler struct {
	admin    *service.AdminService
	auth     *service.AdminAuthService
	catalog  *service.CatalogService
	promos   *service.PromoService
	orders   *service.OrderService
	settings *service.SiteSettingsService
}

func NewAdminHandler(
	admin *service.AdminService,
	auth *service.AdminAuthService,
	catalog *service.CatalogService,
	promos *service.PromoService,
	orders *service.OrderService,
	settings *service.SiteSettingsService,
) *AdminHandler {
	return &AdminHandler{
		admin:    admin,
		auth:     auth,
		catalog:  catalog,
		promos:   promos,
		orders:   orders,
		settings: settings,
	}
}

func (h *AdminHandler) RequireAdmin(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}

	identity, err := h.auth.ValidateToken(c.UserContext(), token)
	if err != nil {
		return writeError(c, err)
	}

	c.Locals(adminIdentityKey, identity)
	return c.Next()
}

func (h *AdminHandler) Session(c *fiber.Ctx) error {
	identity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	return c.JSON(fiber.Map{
		"identity": identity,
	})
}

func (h *AdminHandler) Dashboard(c *fiber.Ctx) error {
	identity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	dashboard, err := h.admin.Dashboard(c.UserContext(), identity)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(dashboard)
}

func (h *AdminHandler) ListCatalog(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"items": h.catalog.ListAll(),
	})
}

func (h *AdminHandler) SaveCatalog(c *fiber.Ctx) error {
	var input domain.CatalogItemInput
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	item, err := h.catalog.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(fiber.Map{
		"item": item,
	})
}

func (h *AdminHandler) ListPromos(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"promos": h.promos.List(),
	})
}

func (h *AdminHandler) SavePromo(c *fiber.Ctx) error {
	var input domain.PromoCodeInput
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	promo, err := h.promos.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(fiber.Map{
		"promo": promo,
	})
}

func (h *AdminHandler) GetSettings(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"settings": h.settings.Get(),
	})
}

func (h *AdminHandler) SaveSettings(c *fiber.Ctx) error {
	var input domain.SiteSettingsInput
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	settings, err := h.settings.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	return c.JSON(fiber.Map{
		"settings": settings,
	})
}
