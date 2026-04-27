// Автор: Kuruma
package repository

import (
	"context"

	"holo-site-backend/internal/domain"
)

type CatalogRepository interface {
	Load(ctx context.Context) ([]domain.CatalogItem, error)
	SaveAll(ctx context.Context, items []domain.CatalogItem) error
}

type PromoRepository interface {
	Load(ctx context.Context) ([]domain.PromoCode, error)
	SaveAll(ctx context.Context, promos []domain.PromoCode) error
}

type SiteSettingsRepository interface {
	Load(ctx context.Context) (domain.SiteSettings, error)
	Save(ctx context.Context, settings domain.SiteSettings) error
}

type VisitAnalyticsRepository interface {
	Load(ctx context.Context) (domain.VisitAnalyticsState, error)
	Save(ctx context.Context, state domain.VisitAnalyticsState) error
}

type OrderRepository interface {
	Save(ctx context.Context, order domain.Order) error
	GetByID(ctx context.Context, id string) (domain.Order, error)
	List(ctx context.Context, filter domain.OrderListFilter) ([]domain.Order, error)
	Update(ctx context.Context, id string, update domain.OrderStatusUpdate, handledBy string) (domain.Order, error)
}
