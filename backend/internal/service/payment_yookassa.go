package service

// YooKassa Payments API v3 integration.
// Документация: https://yookassa.ru/developers/api
//
// Поток оплаты:
//   1. Пользователь оформляет заказ → backend сохраняет его со статусом "pending".
//   2. Backend создаёт платеж в YooKassa (POST /v3/payments) с уникальным
//      Idempotence-Key = order.ID и redirect-confirmation, получает
//      confirmation_url.
//   3. Frontend получает redirectUrl и перенаправляет покупателя в YooKassa.
//   4. YooKassa уведомляет backend webhook'ом (event = payment.succeeded).
//      Backend ПЕРЕПРОВЕРЯЕТ платеж через GET /v3/payments/{id} с Basic Auth
//      (shopId:secretKey) — это надежнее, чем доверять полю статуса в теле
//      webhook'а, и защищает от подмены источника.
//   5. Если статус подтвержден (succeeded), backend:
//        - переводит заказ в "issued"
//        - вызывает webhook плагина (DispatchOrderIssued)
//        - возвращает 200 OK YooKassa (иначе она будет повторять)

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"holo-site-backend/internal/domain"
)

// yooKassaBaseURL — продакшен и тест используют один URL, режим
// определяется ключами магазина.
const yooKassaBaseURL = "https://api.yookassa.ru/v3"

// YooKassaPayment — упрощенная модель ответа API YooKassa, в которой нас
// интересует только статус, ID и confirmation URL.
type yooKassaPayment struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Paid         bool   `json:"paid"`
	Confirmation struct {
		Type            string `json:"type"`
		ConfirmationURL string `json:"confirmation_url"`
	} `json:"confirmation"`
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Metadata map[string]string `json:"metadata"`
}

// CreateYooKassaPayment создает платеж в YooKassa и возвращает URL формы
// оплаты, на которую нужно перенаправить покупателя.
// sbp=true принудительно проводит платеж через Систему Быстрых Платежей
// (payment_method_data.type=sbp) — покупатель оплачивает QR-кодом из
// мобильного приложения своего банка.
func (s *IntegrationService) createYooKassaPayment(ctx context.Context, order domain.Order, settings domain.PaymentSettings, sbp bool) (string, string, error) {
	shopID := strings.TrimSpace(settings.ShopID)
	secret := strings.TrimSpace(settings.SecretKey)
	if shopID == "" || secret == "" {
		return "", "", domain.NewBadRequest("ЮKassa: заполните Shop ID и Secret Key в настройках оплаты.")
	}

	returnURL := strings.TrimSpace(settings.ReturnURL)
	if returnURL == "" {
		return "", "", domain.NewBadRequest("ЮKassa: заполните URL возврата (Return URL) в настройках оплаты.")
	}

	// Заголовок Idempotence-Key защищает от двойного списания, если
	// фронт повторит запрос. Используем order.ID + короткий nonce, чтобы
	// при повторных попытках на одном заказе мы могли пересоздавать
	// неоплаченный платеж — иначе зависший pending заблокировал бы заказ.
	nonce := make([]byte, 4)
	_, _ = rand.Read(nonce)
	idempotenceKey := order.ID + "-" + hex.EncodeToString(nonce)

	body := map[string]any{
		"amount": map[string]string{
			"value":    fmt.Sprintf("%d.00", order.FinalPrice),
			"currency": defaultIfEmpty(order.Currency, "RUB"),
		},
		"capture": true,
		"confirmation": map[string]string{
			"type":       "redirect",
			"return_url": fmt.Sprintf("%s?order=%s", returnURL, order.ID),
		},
		"description": fmt.Sprintf("ESTELAR.SU — %s (%s) для %s",
			order.ProductName, order.PeriodLabel, order.Nickname),
		"metadata": map[string]string{
			"order_id":     order.ID,
			"product_slug": order.ProductSlug,
			"category":     order.Category,
			"nickname":     order.Nickname,
		},
	}
	if sbp {
		body["payment_method_data"] = map[string]string{"type": "sbp"}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		yooKassaBaseURL+"/payments", bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.SetBasicAuth(shopID, secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotence-Key", idempotenceKey)
	req.Header.Set("User-Agent", "estelar-backend/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("yookassa: транспортная ошибка: %v", err)
		return "", "", domain.NewBadRequest("ЮKassa недоступна. Проверьте интернет и повторите попытку.")
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", domain.NewBadRequest("ЮKassa: не удалось прочитать ответ.")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Подробности — только в лог backend'а, клиенту даём короткое
		// сообщение без утечки секретов из тела ошибки.
		log.Printf("yookassa: %d %s", resp.StatusCode, string(raw))
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "", "", domain.NewBadRequest("ЮKassa отвергла Shop ID или Secret Key. Проверьте настройки оплаты.")
		case http.StatusBadRequest:
			return "", "", domain.NewBadRequest("ЮKassa отклонила параметры платежа. Проверьте Return URL и сумму заказа.")
		default:
			return "", "", domain.NewBadRequest("ЮKassa сейчас недоступна, попробуйте через минуту.")
		}
	}

	var payment yooKassaPayment
	if err := json.Unmarshal(raw, &payment); err != nil {
		log.Printf("yookassa: некорректный JSON в ответе: %v / %s", err, string(raw))
		return "", "", domain.NewBadRequest("ЮKassa вернула неожиданный ответ.")
	}

	if payment.Confirmation.ConfirmationURL == "" {
		return "", "", errors.New("yookassa: confirmation_url пуст")
	}

	return payment.ID, payment.Confirmation.ConfirmationURL, nil
}

// FetchYooKassaPayment повторно загружает платеж из API YooKassa и
// возвращает его актуальный статус. Используется в обработчике webhook
// для проверки подлинности уведомления.
func (s *IntegrationService) FetchYooKassaPayment(ctx context.Context, paymentID string) (domain.PaymentVerification, error) {
	settings := s.Payments()
	shopID := strings.TrimSpace(settings.ShopID)
	secret := strings.TrimSpace(settings.SecretKey)
	if shopID == "" || secret == "" {
		return domain.PaymentVerification{}, domain.NewBadRequest("ЮKassa не настроена.")
	}
	if strings.TrimSpace(paymentID) == "" {
		return domain.PaymentVerification{}, domain.NewBadRequest("paymentID пустой.")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		yooKassaBaseURL+"/payments/"+paymentID, nil)
	if err != nil {
		return domain.PaymentVerification{}, err
	}
	req.SetBasicAuth(shopID, secret)
	req.Header.Set("User-Agent", "estelar-backend/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return domain.PaymentVerification{}, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.PaymentVerification{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.PaymentVerification{}, fmt.Errorf("ЮKassa ответила %d: %s", resp.StatusCode, string(raw))
	}

	var payment yooKassaPayment
	if err := json.Unmarshal(raw, &payment); err != nil {
		return domain.PaymentVerification{}, err
	}

	return domain.PaymentVerification{
		Provider:  domain.PaymentProviderYooKassa,
		PaymentID: payment.ID,
		Status:    payment.Status,
		Paid:      payment.Status == "succeeded" && payment.Paid,
		OrderID:   payment.Metadata["order_id"],
		FetchedAt: time.Now().UTC(),
	}, nil
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
