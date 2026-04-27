// Автор: Kuruma
package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/service"
)

type AnalyticsHandler struct {
	service *service.VisitAnalyticsService
}

func NewAnalyticsHandler(service *service.VisitAnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) Track(c *fiber.Ctx) error {
	var input domain.VisitEvent
	if err := parseJSONBody(c, &input); err != nil {
		return writeError(c, err)
	}

	if strings.TrimSpace(input.VisitorID) == "" {
		input.VisitorID = fallbackVisitorID(c)
	}

	if err := h.service.Track(c.UserContext(), input); err != nil {
		return writeError(c, err)
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func fallbackVisitorID(c *fiber.Ctx) string {
	payload := strings.TrimSpace(c.IP()) + "|" + strings.TrimSpace(c.Get(fiber.HeaderUserAgent))
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}
