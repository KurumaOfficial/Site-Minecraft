package domain

import "time"

type PromoResult struct {
	Code    string `json:"code,omitempty"`
	Applied bool   `json:"applied"`
	Message string `json:"message,omitempty"`
	Percent int    `json:"percent,omitempty"`
}

type QuoteRequest struct {
	ProductSlug string `json:"productSlug"`
	PeriodCode  string `json:"periodCode"`
	PromoCode   string `json:"promoCode"`
	Quantity    int    `json:"quantity"`
}

type Quote struct {
	ProductSlug    string      `json:"productSlug"`
	ProductName    string      `json:"productName"`
	PeriodCode     string      `json:"periodCode"`
	PeriodLabel    string      `json:"periodLabel"`
	Quantity       int         `json:"quantity"`
	UnitPrice      int         `json:"unitPrice"`
	UnitLabel      string      `json:"unitLabel,omitempty"`
	VariablePrice  bool        `json:"variablePrice"`
	BasePrice      int         `json:"basePrice"`
	DiscountAmount int         `json:"discountAmount"`
	FinalPrice     int         `json:"finalPrice"`
	Currency       string      `json:"currency"`
	Promo          PromoResult `json:"promo"`
}

type CreateOrderRequest struct {
	ProductSlug string `json:"productSlug"`
	PeriodCode  string `json:"periodCode"`
	Nickname    string `json:"nickname"`
	PromoCode   string `json:"promoCode"`
	Quantity    int    `json:"quantity"`
}

type Order struct {
	ID             string      `json:"id"`
	ProductSlug    string      `json:"productSlug"`
	ProductName    string      `json:"productName"`
	Category       string      `json:"category"`
	CategoryLabel  string      `json:"categoryLabel"`
	Nickname       string      `json:"nickname"`
	PeriodCode     string      `json:"periodCode"`
	PeriodLabel    string      `json:"periodLabel"`
	Quantity       int         `json:"quantity"`
	UnitPrice      int         `json:"unitPrice"`
	UnitLabel      string      `json:"unitLabel,omitempty"`
	VariablePrice  bool        `json:"variablePrice"`
	BasePrice      int         `json:"basePrice"`
	DiscountAmount int         `json:"discountAmount"`
	FinalPrice     int         `json:"finalPrice"`
	Currency       string      `json:"currency"`
	Promo          PromoResult `json:"promo"`
	Status         string      `json:"status"`
	StatusLabel    string      `json:"statusLabel"`
	AdminNote      string      `json:"adminNote,omitempty"`
	HandledBy      string      `json:"handledBy,omitempty"`
	HandledAt      *time.Time  `json:"handledAt,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

type OrderListFilter struct {
	Status string
	Query  string
	Limit  int
}

type OrderStatusUpdate struct {
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel,omitempty"`
	AdminNote   string `json:"adminNote"`
}
