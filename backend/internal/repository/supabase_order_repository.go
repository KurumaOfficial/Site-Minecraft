// Автор: Kuruma
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"holo-site-backend/internal/domain"
)

const supabaseOrderSelect = "public_id,product_slug,product_name,category,category_label,nickname,period_code,period_label,base_price,discount_amount,final_price,currency,promo_code,promo_applied,promo_message,promo_percent,status,status_label,admin_note,handled_by,handled_at,metadata,created_at,updated_at"

type SupabaseOrderRepository struct {
	client *SupabaseRESTClient
}

type supabaseOrderRow struct {
	PublicID       string     `json:"public_id"`
	ProductSlug    string     `json:"product_slug"`
	ProductName    string     `json:"product_name"`
	Category       string     `json:"category"`
	CategoryLabel  string     `json:"category_label"`
	Nickname       string     `json:"nickname"`
	PeriodCode     string     `json:"period_code"`
	PeriodLabel    string     `json:"period_label"`
	BasePrice      int        `json:"base_price"`
	DiscountAmount int        `json:"discount_amount"`
	FinalPrice     int        `json:"final_price"`
	Currency       string     `json:"currency"`
	PromoCode      *string    `json:"promo_code"`
	PromoApplied   bool       `json:"promo_applied"`
	PromoMessage   *string    `json:"promo_message"`
	PromoPercent   *int       `json:"promo_percent"`
	Status         string     `json:"status"`
	StatusLabel    string     `json:"status_label"`
	AdminNote      *string    `json:"admin_note"`
	HandledBy      *string    `json:"handled_by"`
	HandledAt      *time.Time `json:"handled_at"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func NewSupabaseOrderRepository(client *SupabaseRESTClient) *SupabaseOrderRepository {
	return &SupabaseOrderRepository{client: client}
}

func (r *SupabaseOrderRepository) Save(ctx context.Context, order domain.Order) error {
	payload := []map[string]any{{
		"public_id":       order.ID,
		"product_slug":    order.ProductSlug,
		"product_name":    order.ProductName,
		"category":        order.Category,
		"category_label":  order.CategoryLabel,
		"nickname":        order.Nickname,
		"period_code":     order.PeriodCode,
		"period_label":    order.PeriodLabel,
		"base_price":      order.BasePrice,
		"discount_amount": order.DiscountAmount,
		"final_price":     order.FinalPrice,
		"currency":        order.Currency,
		"promo_code":      nilIfEmpty(order.Promo.Code),
		"promo_applied":   order.Promo.Applied,
		"promo_message":   nilIfEmpty(order.Promo.Message),
		"promo_percent":   nilIfZero(order.Promo.Percent),
		"status":          order.Status,
		"status_label":    order.StatusLabel,
		"admin_note":      nilIfEmpty(order.AdminNote),
		"handled_by":      nilIfEmpty(order.HandledBy),
		"handled_at":      order.HandledAt,
		"metadata":        buildOrderMetadata(order),
		"created_at":      order.CreatedAt,
		"updated_at":      order.UpdatedAt,
	}}

	return r.client.Post(ctx, "store_orders", nil, payload, nil)
}

func (r *SupabaseOrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	query := url.Values{}
	query.Set("select", supabaseOrderSelect)
	query.Set("public_id", "eq."+id)
	query.Set("limit", "1")

	var rows []supabaseOrderRow
	if err := r.client.Get(ctx, "store_orders", query, &rows); err != nil {
		return domain.Order{}, err
	}
	if len(rows) == 0 {
		return domain.Order{}, domain.NewNotFound("Заказ не найден.")
	}

	return mapSupabaseOrder(rows[0]), nil
}

func (r *SupabaseOrderRepository) List(ctx context.Context, filter domain.OrderListFilter) ([]domain.Order, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	query := url.Values{}
	query.Set("select", supabaseOrderSelect)
	query.Set("order", "created_at.desc")
	query.Set("limit", strconv.Itoa(limit))

	if filter.Status != "" {
		query.Set("status", "eq."+filter.Status)
	}

	if term := sanitizeSupabaseSearch(filter.Query); term != "" {
		pattern := "*" + term + "*"
		query.Set("or", fmt.Sprintf("(public_id.ilike.%s,nickname.ilike.%s,product_name.ilike.%s,product_slug.ilike.%s)", pattern, pattern, pattern, pattern))
	}

	var rows []supabaseOrderRow
	if err := r.client.Get(ctx, "store_orders", query, &rows); err != nil {
		return nil, err
	}

	orders := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		orders = append(orders, mapSupabaseOrder(row))
	}

	return orders, nil
}

func (r *SupabaseOrderRepository) Update(ctx context.Context, id string, update domain.OrderStatusUpdate, handledBy string) (domain.Order, error) {
	now := time.Now().UTC()
	payload := map[string]any{
		"status":       update.Status,
		"status_label": update.StatusLabel,
		"admin_note":   nilIfEmpty(update.AdminNote),
		"updated_at":   now,
	}
	if strings.TrimSpace(handledBy) != "" {
		payload["handled_by"] = strings.TrimSpace(handledBy)
		payload["handled_at"] = now
	}

	query := url.Values{}
	query.Set("select", supabaseOrderSelect)
	query.Set("public_id", "eq."+id)

	var rows []supabaseOrderRow
	if err := r.client.Patch(ctx, "store_orders", query, payload, &rows); err != nil {
		return domain.Order{}, err
	}
	if len(rows) == 0 {
		return domain.Order{}, domain.NewNotFound("Заказ не найден.")
	}

	return mapSupabaseOrder(rows[0]), nil
}

func mapSupabaseOrder(row supabaseOrderRow) domain.Order {
	order := domain.Order{
		ID:             row.PublicID,
		ProductSlug:    row.ProductSlug,
		ProductName:    row.ProductName,
		Category:       row.Category,
		CategoryLabel:  row.CategoryLabel,
		Nickname:       row.Nickname,
		PeriodCode:     row.PeriodCode,
		PeriodLabel:    row.PeriodLabel,
		BasePrice:      row.BasePrice,
		DiscountAmount: row.DiscountAmount,
		FinalPrice:     row.FinalPrice,
		Currency:       row.Currency,
		Status:         row.Status,
		StatusLabel:    row.StatusLabel,
		HandledAt:      row.HandledAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}

	if row.PromoCode != nil {
		order.Promo.Code = *row.PromoCode
	}
	order.Promo.Applied = row.PromoApplied
	if row.PromoMessage != nil {
		order.Promo.Message = *row.PromoMessage
	}
	if row.PromoPercent != nil {
		order.Promo.Percent = *row.PromoPercent
	}
	if row.AdminNote != nil {
		order.AdminNote = *row.AdminNote
	}
	if row.HandledBy != nil {
		order.HandledBy = *row.HandledBy
	}
	applyOrderMetadata(&order, row.Metadata)

	return order
}

func sanitizeSupabaseSearch(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	for _, char := range raw {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			builder.WriteRune(char)
		case char == ' ', char == '-', char == '_':
			builder.WriteRune(char)
		}

		if builder.Len() >= 64 {
			break
		}
	}

	return strings.TrimSpace(builder.String())
}

func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	return strings.TrimSpace(value)
}

func nilIfZero(value int) any {
	if value == 0 {
		return nil
	}

	return value
}
