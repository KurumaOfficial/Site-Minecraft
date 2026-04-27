// Автор: Kuruma
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strings"

	"github.com/gofiber/fiber/v2"

	"holo-site-backend/internal/domain"
)

const adminIdentityKey = "adminIdentity"

func writeError(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "Внутренняя ошибка сервера."

	var appError *domain.AppError
	if errors.As(err, &appError) {
		status = appError.Status
		message = appError.Message
	}

	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status = fiberError.Code
		if status < fiber.StatusInternalServerError {
			message = fiberError.Message
		}
	}

	return c.Status(status).JSON(fiber.Map{
		"error":     message,
		"requestId": c.Get(fiber.HeaderXRequestID),
	})
}

func parseJSONBody(c *fiber.Ctx, dest any) error {
	contentType := strings.TrimSpace(c.Get(fiber.HeaderContentType))
	if contentType == "" {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "Content-Type должен быть application/json.")
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != fiber.MIMEApplicationJSON {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "Content-Type должен быть application/json.")
	}

	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return domain.NewBadRequest("Некорректное JSON-тело запроса.")
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.NewBadRequest("JSON-тело запроса должно содержать один объект.")
	}

	return nil
}
