package domain

import "time"

type PromoCode struct {
	Code            string     `json:"code"`
	DiscountPercent int        `json:"discountPercent"`
	IsActive        bool       `json:"isActive"`
	UsageLimit      *int       `json:"usageLimit,omitempty"`
	TimesUsed       int        `json:"timesUsed"`
	StartsAt        *time.Time `json:"startsAt,omitempty"`
	EndsAt          *time.Time `json:"endsAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type PromoCodeInput struct {
	Code            string     `json:"code"`
	DiscountPercent int        `json:"discountPercent"`
	IsActive        bool       `json:"isActive"`
	UsageLimit      *int       `json:"usageLimit"`
	StartsAt        *time.Time `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
}
