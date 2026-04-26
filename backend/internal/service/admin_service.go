package service

import (
	"context"

	"holo-site-backend/internal/domain"
)

type AdminService struct {
	catalog  *CatalogService
	promos   *PromoService
	orders   *OrderService
	settings *SiteSettingsService
	visits   *VisitAnalyticsService
}

func NewAdminService(catalog *CatalogService, promos *PromoService, orders *OrderService, settings *SiteSettingsService, visits *VisitAnalyticsService) *AdminService {
	return &AdminService{
		catalog:  catalog,
		promos:   promos,
		orders:   orders,
		settings: settings,
		visits:   visits,
	}
}

func (s *AdminService) Dashboard(ctx context.Context, identity domain.AdminIdentity) (domain.AdminDashboard, error) {
	catalog := s.catalog.ListAll()
	promos := s.promos.List()

	recentOrders, err := s.orders.List(ctx, domain.OrderListFilter{Limit: 12})
	if err != nil {
		return domain.AdminDashboard{}, err
	}

	pendingOrders, err := s.orders.List(ctx, domain.OrderListFilter{Status: "pending", Limit: 500})
	if err != nil {
		return domain.AdminDashboard{}, err
	}

	issuedOrders, err := s.orders.List(ctx, domain.OrderListFilter{Status: "issued", Limit: 500})
	if err != nil {
		return domain.AdminDashboard{}, err
	}

	activeProducts := 0
	for _, item := range catalog {
		if item.IsActive {
			activeProducts++
		}
	}

	activePromos := 0
	for _, promo := range promos {
		if promo.IsActive {
			activePromos++
		}
	}

	return domain.AdminDashboard{
		Identity:       identity,
		Analytics:      s.visits.Summary(),
		TotalProducts:  len(catalog),
		ActiveProducts: activeProducts,
		TotalPromos:    len(promos),
		ActivePromos:   activePromos,
		PendingOrders:  len(pendingOrders),
		IssuedOrders:   len(issuedOrders),
		RecentOrders:   recentOrders,
		Promos:         promos,
		Catalog:        catalog,
		Settings:       s.settings.Get(),
	}, nil
}
