// Автор: Kuruma
package service

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

var promoCodePatternStrict = regexp.MustCompile(`^[A-Z0-9_-]{3,32}$`)

type PromoService struct {
	repo   repository.PromoRepository
	mu     sync.RWMutex
	promos []domain.PromoCode
	byCode map[string]domain.PromoCode
}

func NewPromoService(ctx context.Context, repo repository.PromoRepository, seeded map[string]int) (*PromoService, error) {
	service := &PromoService{
		repo:   repo,
		promos: make([]domain.PromoCode, 0),
		byCode: make(map[string]domain.PromoCode),
	}

	if err := service.bootstrap(ctx, seeded); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *PromoService) Resolve(rawCode string) domain.PromoResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return domain.PromoResult{}
	}

	if len(code) > 32 || !promoCodePatternStrict.MatchString(code) {
		return domain.PromoResult{
			Applied: false,
			Message: "Промокод не найден.",
		}
	}

	promo, found := s.byCode[code]
	if !found || !promo.IsActive {
		return domain.PromoResult{
			Code:    code,
			Applied: false,
			Message: "Промокод не найден.",
		}
	}

	now := time.Now().UTC()
	if promo.StartsAt != nil && promo.StartsAt.After(now) {
		return domain.PromoResult{
			Code:    code,
			Applied: false,
			Message: "Промокод еще не активен.",
		}
	}

	if promo.EndsAt != nil && promo.EndsAt.Before(now) {
		return domain.PromoResult{
			Code:    code,
			Applied: false,
			Message: "Срок действия промокода завершен.",
		}
	}

	if promo.UsageLimit != nil && promo.TimesUsed >= *promo.UsageLimit {
		return domain.PromoResult{
			Code:    code,
			Applied: false,
			Message: "Лимит использования промокода исчерпан.",
		}
	}

	return domain.PromoResult{
		Code:    code,
		Applied: true,
		Message: "Промокод применен: скидка " + strconv.Itoa(promo.DiscountPercent) + "%.",
		Percent: promo.DiscountPercent,
	}
}

func (s *PromoService) MarkUsed(ctx context.Context, code string) error {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	promo, found := s.byCode[normalized]
	if !found {
		return nil
	}

	promo.TimesUsed++
	promo.UpdatedAt = time.Now().UTC()

	for index := range s.promos {
		if s.promos[index].Code == normalized {
			s.promos[index] = promo
			break
		}
	}
	s.byCode[normalized] = promo

	return s.repo.SaveAll(ctx, s.promos)
}

func (s *PromoService) List() []domain.PromoCode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]domain.PromoCode, 0, len(s.promos))
	for _, promo := range s.promos {
		items = append(items, promo)
	}

	return items
}

// FindByCode возвращает копию промокода по коду или nil, если такого нет.
// Используется для аудит-журнала: снимок «до изменения».
func (s *PromoService) FindByCode(code string) *domain.PromoCode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if promo, ok := s.byCode[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return &promo
	}
	return nil
}

func (s *PromoService) Upsert(ctx context.Context, input domain.PromoCodeInput) (domain.PromoCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	promo, err := normalizePromoInput(input)
	if err != nil {
		return domain.PromoCode{}, err
	}

	existingIndex := -1
	for index, current := range s.promos {
		if current.Code == promo.Code {
			existingIndex = index
			promo.TimesUsed = current.TimesUsed
			promo.CreatedAt = current.CreatedAt
			break
		}
	}

	now := time.Now().UTC()
	if promo.CreatedAt.IsZero() {
		promo.CreatedAt = now
	}
	promo.UpdatedAt = now

	if existingIndex >= 0 {
		s.promos[existingIndex] = promo
	} else {
		s.promos = append(s.promos, promo)
	}

	slices.SortFunc(s.promos, func(left, right domain.PromoCode) int {
		return strings.Compare(left.Code, right.Code)
	})

	s.byCode = make(map[string]domain.PromoCode, len(s.promos))
	for _, current := range s.promos {
		s.byCode[current.Code] = current
	}

	if err := s.repo.SaveAll(ctx, s.promos); err != nil {
		return domain.PromoCode{}, err
	}

	return promo, nil
}

func (s *PromoService) bootstrap(ctx context.Context, seeded map[string]int) error {
	promos, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}

	if len(promos) == 0 {
		now := time.Now().UTC()
		for code, percent := range seeded {
			promos = append(promos, domain.PromoCode{
				Code:            strings.ToUpper(strings.TrimSpace(code)),
				DiscountPercent: percent,
				IsActive:        true,
				CreatedAt:       now,
				UpdatedAt:       now,
			})
		}

		slices.SortFunc(promos, func(left, right domain.PromoCode) int {
			return strings.Compare(left.Code, right.Code)
		})

		if err := s.repo.SaveAll(ctx, promos); err != nil {
			return err
		}
	}

	s.promos = promos
	s.byCode = make(map[string]domain.PromoCode, len(promos))
	for _, promo := range promos {
		s.byCode[promo.Code] = promo
	}

	return nil
}

func normalizePromoInput(input domain.PromoCodeInput) (domain.PromoCode, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if !promoCodePatternStrict.MatchString(code) {
		return domain.PromoCode{}, domain.NewBadRequest("Укажите корректный код промокода.")
	}

	if input.DiscountPercent <= 0 || input.DiscountPercent > 100 {
		return domain.PromoCode{}, domain.NewBadRequest("Размер скидки должен быть от 1 до 100 процентов.")
	}

	if input.UsageLimit != nil && *input.UsageLimit <= 0 {
		return domain.PromoCode{}, domain.NewBadRequest("Лимит использования должен быть больше нуля.")
	}

	return domain.PromoCode{
		Code:            code,
		DiscountPercent: input.DiscountPercent,
		IsActive:        input.IsActive,
		UsageLimit:      input.UsageLimit,
		StartsAt:        input.StartsAt,
		EndsAt:          input.EndsAt,
	}, nil
}
