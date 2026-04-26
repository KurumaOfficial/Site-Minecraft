package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

var nicknamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
var productSlugPattern = regexp.MustCompile(`^[a-z0-9-]{2,64}$`)
var periodCodePattern = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
var orderIDPattern = regexp.MustCompile(`^ESTELAR-\d{8}-[A-F0-9]{8}$`)

var orderStatusLabels = map[string]string{
	"pending":  "Ожидает ручной выдачи",
	"review":   "На проверке администратора",
	"issued":   "Выдано вручную",
	"rejected": "Отклонено",
}

type OrderService struct {
	catalog    *CatalogService
	promos     *PromoService
	repository repository.OrderRepository
}

func NewOrderService(catalog *CatalogService, promos *PromoService, repository repository.OrderRepository) *OrderService {
	return &OrderService{
		catalog:    catalog,
		promos:     promos,
		repository: repository,
	}
}

func (s *OrderService) Quote(request domain.QuoteRequest) (domain.Quote, error) {
	item, period, err := s.resolveItemAndPeriod(request.ProductSlug, request.PeriodCode)
	if err != nil {
		return domain.Quote{}, err
	}

	quantity, unitPrice, unitLabel, variablePrice, err := resolveOrderQuantity(item, period, request.Quantity)
	if err != nil {
		return domain.Quote{}, err
	}

	promo := s.promos.Resolve(request.PromoCode)
	basePrice := unitPrice * quantity
	discountAmount := calculateDiscount(basePrice, promo)
	finalPrice := basePrice - discountAmount
	if finalPrice < 0 {
		finalPrice = 0
	}

	return domain.Quote{
		ProductSlug:    item.Slug,
		ProductName:    item.Name,
		PeriodCode:     period.Code,
		PeriodLabel:    period.Label,
		Quantity:       quantity,
		UnitPrice:      unitPrice,
		UnitLabel:      unitLabel,
		VariablePrice:  variablePrice,
		BasePrice:      basePrice,
		DiscountAmount: discountAmount,
		FinalPrice:     finalPrice,
		Currency:       item.Currency,
		Promo:          promo,
	}, nil
}

func (s *OrderService) Create(ctx context.Context, request domain.CreateOrderRequest) (domain.Order, error) {
	nickname := strings.TrimSpace(request.Nickname)
	if nickname == "" {
		return domain.Order{}, domain.NewBadRequest("Укажите ник получателя.")
	}

	if !nicknamePattern.MatchString(nickname) {
		return domain.Order{}, domain.NewBadRequest("Ник должен содержать от 3 до 16 символов: латиница, цифры или _.")
	}

	item, period, err := s.resolveItemAndPeriod(request.ProductSlug, request.PeriodCode)
	if err != nil {
		return domain.Order{}, err
	}

	quantity, unitPrice, unitLabel, variablePrice, err := resolveOrderQuantity(item, period, request.Quantity)
	if err != nil {
		return domain.Order{}, err
	}

	promo := s.promos.Resolve(request.PromoCode)
	basePrice := unitPrice * quantity
	discountAmount := calculateDiscount(basePrice, promo)
	finalPrice := basePrice - discountAmount
	if finalPrice < 0 {
		finalPrice = 0
	}

	now := time.Now().UTC()
	order := domain.Order{
		ID:             newOrderID(),
		ProductSlug:    item.Slug,
		ProductName:    item.Name,
		Category:       item.Category,
		CategoryLabel:  item.CategoryLabel,
		Nickname:       nickname,
		PeriodCode:     period.Code,
		PeriodLabel:    period.Label,
		Quantity:       quantity,
		UnitPrice:      unitPrice,
		UnitLabel:      unitLabel,
		VariablePrice:  variablePrice,
		BasePrice:      basePrice,
		DiscountAmount: discountAmount,
		FinalPrice:     finalPrice,
		Currency:       item.Currency,
		Promo:          promo,
		Status:         "pending",
		StatusLabel:    orderStatusLabels["pending"],
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repository.Save(ctx, order); err != nil {
		return domain.Order{}, domain.NewInternal("Не удалось сохранить заказ.")
	}

	if promo.Applied {
		_ = s.promos.MarkUsed(ctx, promo.Code)
	}

	return order, nil
}

func (s *OrderService) GetByID(ctx context.Context, id string) (domain.Order, error) {
	orderID := strings.TrimSpace(id)
	if orderID == "" {
		return domain.Order{}, domain.NewBadRequest("Укажите номер заказа.")
	}

	if len(orderID) > 32 || !orderIDPattern.MatchString(orderID) {
		return domain.Order{}, domain.NewBadRequest("Некорректный формат номера заказа.")
	}

	order, err := s.repository.GetByID(ctx, orderID)
	if err != nil {
		if _, ok := err.(*domain.AppError); ok {
			return domain.Order{}, err
		}
		return domain.Order{}, domain.NewInternal("Не удалось получить заказ.")
	}

	return order, nil
}

func (s *OrderService) List(ctx context.Context, filter domain.OrderListFilter) ([]domain.Order, error) {
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Status != "" {
		if _, found := orderStatusLabels[filter.Status]; !found {
			return nil, domain.NewBadRequest("Недопустимый статус фильтра.")
		}
	}

	filter.Query = strings.TrimSpace(filter.Query)
	if len(filter.Query) > 80 {
		filter.Query = filter.Query[:80]
	}

	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}

	return s.repository.List(ctx, filter)
}

func (s *OrderService) Update(ctx context.Context, id string, update domain.OrderStatusUpdate, handledBy string) (domain.Order, error) {
	orderID := strings.TrimSpace(id)
	if orderID == "" || !orderIDPattern.MatchString(orderID) {
		return domain.Order{}, domain.NewBadRequest("Некорректный номер заказа.")
	}

	status := strings.TrimSpace(update.Status)
	label, found := orderStatusLabels[status]
	if !found {
		return domain.Order{}, domain.NewBadRequest("Недопустимый статус заказа.")
	}

	update.Status = status
	update.StatusLabel = label

	return s.repository.Update(ctx, orderID, update, strings.TrimSpace(handledBy))
}

func (s *OrderService) resolveItemAndPeriod(productSlug, periodCode string) (domain.CatalogItem, domain.PeriodOption, error) {
	slug := strings.TrimSpace(productSlug)
	if slug == "" {
		return domain.CatalogItem{}, domain.PeriodOption{}, domain.NewBadRequest("Не указан товар.")
	}

	if len(slug) > 64 || !productSlugPattern.MatchString(slug) {
		return domain.CatalogItem{}, domain.PeriodOption{}, domain.NewBadRequest("Некорректный идентификатор товара.")
	}

	item, found := s.catalog.FindBySlug(slug)
	if !found {
		return domain.CatalogItem{}, domain.PeriodOption{}, domain.NewNotFound("Товар не найден.")
	}

	code := strings.TrimSpace(periodCode)
	if code == "" {
		code = "forever"
	}

	if len(code) > 32 || !periodCodePattern.MatchString(code) {
		return domain.CatalogItem{}, domain.PeriodOption{}, domain.NewBadRequest("Некорректный код срока.")
	}

	for _, period := range item.Periods {
		if period.Code == code {
			return item, period, nil
		}
	}

	return domain.CatalogItem{}, domain.PeriodOption{}, domain.NewBadRequest("Выбранный срок недоступен.")
}

func calculateDiscount(basePrice int, promo domain.PromoResult) int {
	if !promo.Applied || promo.Percent <= 0 {
		return 0
	}

	return basePrice * promo.Percent / 100
}

func resolveOrderQuantity(item domain.CatalogItem, period domain.PeriodOption, requested int) (int, int, string, bool, error) {
	unitPrice := period.Price
	if item.VariablePrice {
		minQuantity := item.MinQuantity
		if minQuantity <= 0 {
			minQuantity = 1
		}

		maxQuantity := item.MaxQuantity
		if maxQuantity <= 0 {
			maxQuantity = 50000
		}

		step := item.QuantityStep
		if step <= 0 {
			step = 1
		}

		quantity := requested
		if quantity == 0 {
			quantity = minQuantity
		}

		if quantity < minQuantity {
			return 0, 0, "", false, domain.NewBadRequest(fmt.Sprintf("Минимальное количество: %d.", minQuantity))
		}

		if quantity > maxQuantity {
			return 0, 0, "", false, domain.NewBadRequest(fmt.Sprintf("Максимум за один заказ: %d.", maxQuantity))
		}

		if quantity%step != 0 {
			return 0, 0, "", false, domain.NewBadRequest(fmt.Sprintf("Количество должно быть кратно %d.", step))
		}

		unitLabel := strings.TrimSpace(item.UnitLabel)
		if unitLabel == "" {
			unitLabel = "единиц"
		}

		return quantity, unitPrice, unitLabel, true, nil
	}

	if requested > 1 {
		return 0, 0, "", false, domain.NewBadRequest("Для этого товара количество не настраивается.")
	}

	return 1, unitPrice, "", false, nil
}

func newOrderID() string {
	return fmt.Sprintf("ESTELAR-%s-%s", time.Now().Format("20060102"), strings.ToUpper(uuid.NewString()[:8]))
}
