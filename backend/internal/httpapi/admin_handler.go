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
	audit    *service.AuditLogService
}

func NewAdminHandler(
	admin *service.AdminService,
	auth *service.AdminAuthService,
	catalog *service.CatalogService,
	promos *service.PromoService,
	orders *service.OrderService,
	settings *service.SiteSettingsService,
	audit *service.AuditLogService,
) *AdminHandler {
	return &AdminHandler{
		admin:    admin,
		auth:     auth,
		catalog:  catalog,
		promos:   promos,
		orders:   orders,
		settings: settings,
		audit:    audit,
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

	beforeItem, beforeFound := h.catalog.FindBySlug(input.Slug)
	item, err := h.catalog.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	identity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	verb := "Создан товар"
	var beforePtr *domain.CatalogItem
	if beforeFound {
		verb = "Обновлён товар"
		beforePtr = &beforeItem
	}
	h.audit.Record(
		c.UserContext(),
		identity.ID, identity.Name,
		"catalog.save",
		item.Slug,
		verb+" «"+item.Name+"»",
		catalogSnapshot(beforePtr),
		catalogSnapshot(&item),
		c.IP(), c.Get(fiber.HeaderUserAgent),
	)

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

	before := h.promos.FindByCode(input.Code)
	promo, err := h.promos.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	identity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	verb := "Создан промокод"
	if before != nil {
		verb = "Обновлён промокод"
	}
	h.audit.Record(
		c.UserContext(),
		identity.ID, identity.Name,
		"promo.save",
		promo.Code,
		verb+" "+promo.Code,
		promoSnapshot(before),
		promoSnapshot(&promo),
		c.IP(), c.Get(fiber.HeaderUserAgent),
	)

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

	before := h.settings.Get()
	settings, err := h.settings.Upsert(c.UserContext(), input)
	if err != nil {
		return writeError(c, err)
	}

	identity, _ := c.Locals(adminIdentityKey).(domain.AdminIdentity)
	h.audit.Record(
		c.UserContext(),
		identity.ID, identity.Name,
		"settings.save",
		"contacts",
		"Обновлены контакты сайта",
		settingsSnapshot(before),
		settingsSnapshot(settings),
		c.IP(), c.Get(fiber.HeaderUserAgent),
	)

	return c.JSON(fiber.Map{
		"settings": settings,
	})
}

// ListAudit возвращает последние записи журнала действий.
func (h *AdminHandler) ListAudit(c *fiber.Ctx) error {
	filter := domain.AuditLogFilter{
		Limit:   c.QueryInt("limit", 200),
		Action:  strings.TrimSpace(c.Query("action")),
		ActorID: strings.TrimSpace(c.Query("actorId")),
	}
	entries, err := h.audit.List(c.UserContext(), filter)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{
		"entries": entries,
	})
}

func catalogSnapshot(item *domain.CatalogItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"slug":     item.Slug,
		"name":     item.Name,
		"category": item.Category,
		"price":    item.Price,
		"image":    item.Image,
	}
}

func promoSnapshot(p *domain.PromoCode) map[string]any {
	if p == nil {
		return nil
	}
	return map[string]any{
		"code":            p.Code,
		"discountPercent": p.DiscountPercent,
		"isActive":        p.IsActive,
		"usageLimit":      p.UsageLimit,
		"timesUsed":       p.TimesUsed,
	}
}

func settingsSnapshot(s domain.SiteSettings) map[string]any {
	return map[string]any{
		"discordUrl":   s.DiscordURL,
		"discordText":  s.DiscordText,
		"vkUrl":        s.VKURL,
		"vkText":       s.VKText,
		"supportEmail": s.SupportEmail,
	}
}
