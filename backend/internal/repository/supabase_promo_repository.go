package repository

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"time"

	"holo-site-backend/internal/domain"
)

type SupabasePromoRepository struct {
	client *SupabaseRESTClient
}

type supabasePromoRow struct {
	Code            string     `json:"code"`
	DiscountPercent int        `json:"discount_percent"`
	IsActive        bool       `json:"is_active"`
	UsageLimit      *int       `json:"usage_limit"`
	TimesUsed       int        `json:"times_used"`
	StartsAt        *time.Time `json:"starts_at"`
	EndsAt          *time.Time `json:"ends_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func NewSupabasePromoRepository(client *SupabaseRESTClient) *SupabasePromoRepository {
	return &SupabasePromoRepository{client: client}
}

func (r *SupabasePromoRepository) Load(ctx context.Context) ([]domain.PromoCode, error) {
	query := url.Values{}
	query.Set("select", "code,discount_percent,is_active,usage_limit,times_used,starts_at,ends_at,created_at,updated_at")
	query.Set("order", "code.asc")

	var rows []supabasePromoRow
	if err := r.client.Get(ctx, "store_promo_codes", query, &rows); err != nil {
		return nil, err
	}

	promos := make([]domain.PromoCode, 0, len(rows))
	for _, row := range rows {
		promos = append(promos, domain.PromoCode{
			Code:            strings.ToUpper(strings.TrimSpace(row.Code)),
			DiscountPercent: row.DiscountPercent,
			IsActive:        row.IsActive,
			UsageLimit:      row.UsageLimit,
			TimesUsed:       row.TimesUsed,
			StartsAt:        row.StartsAt,
			EndsAt:          row.EndsAt,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		})
	}

	return promos, nil
}

func (r *SupabasePromoRepository) SaveAll(ctx context.Context, promos []domain.PromoCode) error {
	deleteQuery := url.Values{}
	deleteQuery.Set("code", "not.is.null")
	if err := r.client.Delete(ctx, "store_promo_codes", deleteQuery); err != nil {
		return err
	}

	sortedPromos := append([]domain.PromoCode(nil), promos...)
	sort.SliceStable(sortedPromos, func(i, j int) bool {
		return sortedPromos[i].Code < sortedPromos[j].Code
	})

	now := time.Now().UTC()
	payload := make([]map[string]any, 0, len(sortedPromos))
	for _, promo := range sortedPromos {
		createdAt := promo.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		updatedAt := promo.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}

		payload = append(payload, map[string]any{
			"code":             strings.ToUpper(strings.TrimSpace(promo.Code)),
			"discount_percent": promo.DiscountPercent,
			"is_active":        promo.IsActive,
			"usage_limit":      promo.UsageLimit,
			"times_used":       promo.TimesUsed,
			"starts_at":        promo.StartsAt,
			"ends_at":          promo.EndsAt,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
		})
	}

	if len(payload) == 0 {
		return nil
	}

	return r.client.Post(ctx, "store_promo_codes", nil, payload, nil)
}
