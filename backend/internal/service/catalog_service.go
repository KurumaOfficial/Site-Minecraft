// Автор: Kuruma
package service

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

var catalogSlugPattern = regexp.MustCompile(`^[a-z0-9-]{2,64}$`)

type catalogRuntimeRule struct {
	VariablePrice bool
	MinQuantity   int
	MaxQuantity   int
	QuantityStep  int
	UnitLabel     string
	CategoryLabel string
	PeriodCode    string
	PeriodLabel   string
}

var catalogRuntimeRules = map[string]catalogRuntimeRule{
	"unban": {
		CategoryLabel: "Услуга",
		PeriodCode:    "forever",
		PeriodLabel:   "Навсегда",
	},
	"unmute": {
		CategoryLabel: "Услуга",
		PeriodCode:    "forever",
		PeriodLabel:   "Навсегда",
	},
	"immunity": {
		CategoryLabel: "Услуга",
		PeriodCode:    "forever",
		PeriodLabel:   "Навсегда",
	},
	"donate-currency": {
		VariablePrice: true,
		MinQuantity:   1,
		MaxQuantity:   500,
		QuantityStep:  1,
		UnitLabel:     "пак.",
		CategoryLabel: "Валюта",
		PeriodCode:    "units",
		PeriodLabel:   "1 пакет = 100 EST",
	},
}

type CatalogService struct {
	repo        repository.CatalogRepository
	mu          sync.RWMutex
	items       []domain.CatalogItem
	itemsBySlug map[string]domain.CatalogItem
}

func NewCatalogService(ctx context.Context, repo repository.CatalogRepository) (*CatalogService, error) {
	service := &CatalogService{
		repo:        repo,
		items:       make([]domain.CatalogItem, 0),
		itemsBySlug: make(map[string]domain.CatalogItem),
	}

	if err := service.bootstrap(ctx); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *CatalogService) List() []domain.CatalogItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]domain.CatalogItem, 0, len(s.items))
	for _, item := range s.items {
		if !item.IsActive {
			continue
		}
		items = append(items, cloneCatalogItem(item))
	}

	return items
}

func (s *CatalogService) ListAll() []domain.CatalogItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]domain.CatalogItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, cloneCatalogItem(item))
	}

	return items
}

func (s *CatalogService) FindBySlug(slug string) (domain.CatalogItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, found := s.itemsBySlug[slug]
	if !found || !item.IsActive {
		return domain.CatalogItem{}, false
	}

	return cloneCatalogItem(item), true
}

func (s *CatalogService) Upsert(ctx context.Context, input domain.CatalogItemInput) (domain.CatalogItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, err := normalizeCatalogInput(input)
	if err != nil {
		return domain.CatalogItem{}, err
	}

	existingIndex := -1
	for index, current := range s.items {
		if current.Slug == item.Slug {
			existingIndex = index
			item.ExternalID = current.ExternalID
			if strings.TrimSpace(input.CategoryLabel) == "" {
				item.CategoryLabel = current.CategoryLabel
			}
			break
		}
	}

	if item.ExternalID == "" {
		item.ExternalID = item.Slug
	}
	item.ID = item.ExternalID

	if existingIndex >= 0 {
		s.items[existingIndex] = item
	} else {
		s.items = append(s.items, item)
	}

	s.sortLocked()
	if err := s.persistLocked(ctx); err != nil {
		return domain.CatalogItem{}, err
	}

	return cloneCatalogItem(applyCatalogRuntimeRules(item)), nil
}

func (s *CatalogService) bootstrap(ctx context.Context) error {
	items, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		items = defaultCatalog()
		if err := s.repo.SaveAll(ctx, items); err != nil {
			return err
		}
	} else {
		var changed bool
		items, changed = mergeCatalogWithDefaults(items)
		if changed {
			if err := s.repo.SaveAll(ctx, items); err != nil {
				return err
			}
		}
	}

	s.items = items
	s.sortLocked()
	return nil
}

func (s *CatalogService) persistLocked(ctx context.Context) error {
	if err := s.repo.SaveAll(ctx, s.items); err != nil {
		return err
	}

	return nil
}

func (s *CatalogService) sortLocked() {
	slices.SortFunc(s.items, func(left, right domain.CatalogItem) int {
		if left.Sort == right.Sort {
			return strings.Compare(left.Name, right.Name)
		}
		return left.Sort - right.Sort
	})

	s.itemsBySlug = make(map[string]domain.CatalogItem, len(s.items))
	for index, item := range s.items {
		decorated := applyCatalogRuntimeRules(item)
		s.items[index] = decorated
		s.itemsBySlug[decorated.Slug] = decorated
	}
}

func cloneCatalogItem(item domain.CatalogItem) domain.CatalogItem {
	cloned := item
	cloned.Highlights = slices.Clone(item.Highlights)
	cloned.Periods = slices.Clone(item.Periods)
	return cloned
}

func normalizeCatalogInput(input domain.CatalogItemInput) (domain.CatalogItem, error) {
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if !catalogSlugPattern.MatchString(slug) {
		return domain.CatalogItem{}, domain.NewBadRequest("Укажите корректный slug товара.")
	}

	name := strings.TrimSpace(input.Name)
	if len(name) < 2 || len(name) > 64 {
		return domain.CatalogItem{}, domain.NewBadRequest("Название товара должно содержать от 2 до 64 символов.")
	}

	category := strings.ToLower(strings.TrimSpace(input.Category))
	if category != "privilege" && category != "case" {
		return domain.CatalogItem{}, domain.NewBadRequest("Категория товара должна быть privilege или case.")
	}

	price := input.Price
	if price < 0 || price > 1000000 {
		return domain.CatalogItem{}, domain.NewBadRequest("Цена товара должна быть в допустимом диапазоне.")
	}

	summary := strings.TrimSpace(input.Summary)
	if len(summary) < 10 || len(summary) > 400 {
		return domain.CatalogItem{}, domain.NewBadRequest("Описание товара должно содержать от 10 до 400 символов.")
	}

	image := strings.TrimSpace(input.Image)
	if image == "" {
		return domain.CatalogItem{}, domain.NewBadRequest("Укажите изображение товара.")
	}

	highlights := make([]string, 0, len(input.Highlights))
	for _, highlight := range input.Highlights {
		text := strings.TrimSpace(highlight)
		if text == "" {
			continue
		}
		if len(text) > 120 {
			return domain.CatalogItem{}, domain.NewBadRequest("Каждый пункт описания должен быть короче 120 символов.")
		}
		highlights = append(highlights, text)
	}

	if len(highlights) == 0 {
		highlights = []string{
			fmt.Sprintf("Фиксированная цена: %d руб.", price),
		}
	}

	categoryLabel := strings.TrimSpace(input.CategoryLabel)
	if categoryLabel == "" {
		if category == "case" {
			categoryLabel = "Кейс"
		} else {
			categoryLabel = "Привилегия"
		}
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "RUB"
	}

	isActive := input.IsActive
	if !input.IsActive && input.ID == "" && input.ExternalID == "" && input.Sort == 0 && price == 0 && len(highlights) == 1 {
		isActive = true
	}

	return domain.CatalogItem{
		ID:            strings.TrimSpace(input.ID),
		ExternalID:    strings.TrimSpace(input.ExternalID),
		Slug:          slug,
		Name:          name,
		Category:      category,
		CategoryLabel: categoryLabel,
		Price:         price,
		Currency:      currency,
		Image:         image,
		Summary:       summary,
		Highlights:    highlights,
		Periods: []domain.PeriodOption{
			{Code: "forever", Label: "Навсегда", Price: price, Default: true},
		},
		Sort:     max(input.Sort, 1),
		IsActive: isActive,
	}, nil
}

func defaultCatalog() []domain.CatalogItem {
	return []domain.CatalogItem{
		newPrivilegeItem("privilege-vip", "vip", "VIP", 19, "assets/images/image-05-3574266306.png", "VIP-привилегия для базового донат-уровня сервера ESTELAR.SU.", 1, []string{"Команды:", "-/suiside - убить самого себя", "-/wb - открыть виртуальный верстак", "-/kit vip - получить набор привилегии", "-Возможности всех прошлых привилегий"}),
		newPrivilegeItem("privilege-crystal", "crystal", "CRYSYSTAL", 49, "assets/images/image-06-ec43498384.png", "Усиленная постоянная привилегия сервера ESTELAR с фиксированной ценой.", 2, []string{"Навсегда без продления", "Второй уровень привилегий", "Фиксированная цена: 49 руб."}),
		newPrivilegeItem("privilege-dragon", "dragon", "GALAXY", 99, "assets/images/image-07-524845b051.png", "Продвинутая постоянная привилегия ESTELAR для активных игроков.", 3, []string{"Навсегда без продления", "Продвинутая линейка магазина", "Фиксированная цена: 99 руб."}),
		newPrivilegeItem("privilege-king", "king", "KING", 179, "assets/images/image-08-cb0961288a.png", "Постоянная привилегия повышенного уровня для сервера ESTELAR.", 4, []string{"Навсегда без продления", "Повышенный уровень статуса", "Фиксированная цена: 179 руб."}),
		newPrivilegeItem("privilege-titan", "titan", "TITAN", 249, "assets/images/image-09-b8a9706aa0.png", "Мощная постоянная привилегия для основной аудитории доната ESTELAR.", 5, []string{"Навсегда без продления", "Старший уровень магазина", "Фиксированная цена: 249 руб."}),
		newPrivilegeItem("privilege-phenix", "phenix", "PHENIX", 369, "assets/images/image-10-597bf172e7.png", "Редкая постоянная привилегия премиум-уровня ESTELAR.", 6, []string{"Навсегда без продления", "Премиальная линейка", "Фиксированная цена: 369 руб."}),
		newPrivilegeItem("privilege-immortal", "immortal", "IMMORTAL", 499, "assets/images/image-11-cd3fc52735.png", "Топовая постоянная привилегия ESTELAR с высоким статусом.", 7, []string{"Навсегда без продления", "Почти максимальный уровень", "Фиксированная цена: 499 руб."}),
		newPrivilegeItem("privilege-infinity", "infinity", "INFINITY", 699, "assets/images/image-12-e7d68a881c.png", "Максимальная постоянная привилегия для игроков, которым нужен полный доступ к донат-линейке ESTELAR.SU.", 8, []string{"Навсегда без продления", "Максимальный уровень линейки", "Фиксированная цена: 699 руб."}),
		newServiceItem("service-unmute", "unmute", "РАЗМУТ", 99, "assets/images/image-15-e2262cce74.png", "РАЗМУТ НА СЕРВЕРЕ\nПри покупке размута аккаунт который вы указали в корзине размутится.", 9, []string{"Разовая услуга", "Ручная обработка заявки", "Стоимость: 99 руб."}),
		newServiceItem("service-unban", "unban", "РАЗБАН", 199, "assets/images/image-14-743744227c.png", "РАЗБАН НА СЕРВЕРЕ\nПри покупке разбана аккаунт который вы указали в корзине разбанится.", 10, []string{"Разовая услуга", "Ручная обработка заявки", "Стоимость: 199 руб."}),
		newServiceItem("service-immunity", "immunity", "ИММУНИТЕТ", 499, "assets/images/image-13-ac5211f93c.png", "ИМУНИТЕТ ОТ БАНА на неделю и только от игроков.\nПри покупке иммунитета аккаунт который вы указали в корзине будет невозможно забанить игрокам, но не админам.", 11, []string{"Срок действия: 1 неделя", "Работает только против игроков", "Администрация сохраняет полный доступ к наказаниям"}),
		newCurrencyItem("currency-donate", "donate-currency", "100 EST", 99, "assets/images/image-13-ac5211f93c.png", "100 EST — донат-валюта сервера ESTELAR.SU.\nПосле покупки на аккаунт поступает 100 EST за каждый выбранный пакет.", 12, []string{"EST — внутриигровая донат-валюта", "1 пакет = 100 EST", "Стоимость пакета: 99 руб."}),
	}
}

func newPrivilegeItem(id, slug, name string, price int, image, summary string, sort int, highlights []string) domain.CatalogItem {
	return domain.CatalogItem{
		ID:            id,
		ExternalID:    id,
		Slug:          slug,
		Name:          name,
		Category:      "privilege",
		CategoryLabel: "Привилегия",
		Price:         price,
		Currency:      "RUB",
		Image:         image,
		Summary:       summary,
		Highlights:    highlights,
		Periods: []domain.PeriodOption{
			{Code: "forever", Label: "Навсегда", Price: price, Default: true},
		},
		Sort:     sort,
		IsActive: true,
	}
}

func newCaseItem(id, slug, name string, price int, image, summary string, sort int, highlights []string) domain.CatalogItem {
	return domain.CatalogItem{
		ID:            id,
		ExternalID:    id,
		Slug:          slug,
		Name:          name,
		Category:      "case",
		CategoryLabel: "Кейс",
		Price:         price,
		Currency:      "RUB",
		Image:         image,
		Summary:       summary,
		Highlights:    highlights,
		Periods: []domain.PeriodOption{
			{Code: "forever", Label: "Навсегда", Price: price, Default: true},
		},
		Sort:     sort,
		IsActive: true,
	}
}

func newServiceItem(id, slug, name string, price int, image, summary string, sort int, highlights []string) domain.CatalogItem {
	item := newPrivilegeItem(id, slug, name, price, image, summary, sort, highlights)
	item.CategoryLabel = "Услуга"
	return item
}

func newCurrencyItem(id, slug, name string, price int, image, summary string, sort int, highlights []string) domain.CatalogItem {
	item := newCaseItem(id, slug, name, price, image, summary, sort, highlights)
	item.CategoryLabel = "Валюта"
	item.Periods = []domain.PeriodOption{
		{Code: "units", Label: "1 пакет = 100 EST", Price: price, Default: true},
	}
	return item
}

func applyCatalogRuntimeRules(item domain.CatalogItem) domain.CatalogItem {
	rule, found := catalogRuntimeRules[item.Slug]
	if !found {
		return item
	}

	item.VariablePrice = rule.VariablePrice
	item.MinQuantity = rule.MinQuantity
	item.MaxQuantity = rule.MaxQuantity
	item.QuantityStep = rule.QuantityStep
	item.UnitLabel = rule.UnitLabel
	if rule.CategoryLabel != "" {
		item.CategoryLabel = rule.CategoryLabel
	}
	if rule.PeriodCode != "" && rule.PeriodLabel != "" {
		price := item.Price
		if price <= 0 && len(item.Periods) > 0 {
			price = item.Periods[0].Price
		}
		if price <= 0 {
			price = 1
		}
		item.Periods = []domain.PeriodOption{
			{Code: rule.PeriodCode, Label: rule.PeriodLabel, Price: price, Default: true},
		}
	}
	if item.Price <= 0 && len(item.Periods) > 0 {
		item.Price = item.Periods[0].Price
	}

	return item
}

func mergeCatalogWithDefaults(items []domain.CatalogItem) ([]domain.CatalogItem, bool) {
	merged := append([]domain.CatalogItem(nil), items...)
	indexBySlug := make(map[string]int, len(merged))
	for index, item := range merged {
		indexBySlug[item.Slug] = index
	}

	changed := false
	for _, fallback := range defaultCatalog() {
		index, found := indexBySlug[fallback.Slug]
		if !found {
			merged = append(merged, fallback)
			indexBySlug[fallback.Slug] = len(merged) - 1
			changed = true
			continue
		}

		enriched := enrichCatalogItem(merged[index], fallback)
		if !reflect.DeepEqual(merged[index], enriched) {
			merged[index] = enriched
			changed = true
		}
	}

	return merged, changed
}

func enrichCatalogItem(current, fallback domain.CatalogItem) domain.CatalogItem {
	if strings.TrimSpace(current.ID) == "" {
		current.ID = fallback.ID
	}
	if strings.TrimSpace(current.ExternalID) == "" {
		current.ExternalID = fallback.ExternalID
	}
	if strings.TrimSpace(current.Category) == "" {
		current.Category = fallback.Category
	}
	if strings.TrimSpace(current.CategoryLabel) == "" {
		current.CategoryLabel = fallback.CategoryLabel
	}
	if current.Price <= 0 {
		current.Price = fallback.Price
	}
	if strings.TrimSpace(current.Currency) == "" {
		current.Currency = fallback.Currency
	}
	if strings.TrimSpace(current.Image) == "" {
		current.Image = fallback.Image
	}
	if strings.TrimSpace(current.Summary) == "" {
		current.Summary = fallback.Summary
	}
	if len(current.Highlights) == 0 {
		current.Highlights = slices.Clone(fallback.Highlights)
	}
	if len(current.Periods) == 0 {
		current.Periods = slices.Clone(fallback.Periods)
	}
	if current.Sort <= 0 {
		current.Sort = fallback.Sort
	}

	return current
}

func max(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
