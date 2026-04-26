package domain

type AdminIdentity struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	Provider  string `json:"provider"`
}

type AdminDashboard struct {
	Identity       AdminIdentity `json:"identity"`
	Analytics      VisitAnalyticsSummary `json:"analytics"`
	TotalProducts  int           `json:"totalProducts"`
	ActiveProducts int           `json:"activeProducts"`
	TotalPromos    int           `json:"totalPromos"`
	ActivePromos   int           `json:"activePromos"`
	PendingOrders  int           `json:"pendingOrders"`
	IssuedOrders   int           `json:"issuedOrders"`
	RecentOrders   []Order       `json:"recentOrders"`
	Promos         []PromoCode   `json:"promos"`
	Catalog        []CatalogItem `json:"catalog"`
	Settings       SiteSettings  `json:"settings"`
}
