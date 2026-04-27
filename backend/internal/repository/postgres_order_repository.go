// Автор: Kuruma
package repository

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"holo-site-backend/internal/domain"
)

type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{pool: pool}
}

func (r *PostgresOrderRepository) Save(ctx context.Context, order domain.Order) error {
	_, err := r.pool.Exec(ctx, `
		insert into store_orders (
			public_id,
			product_slug,
			product_name,
			category,
			category_label,
			nickname,
			period_code,
			period_label,
			base_price,
			discount_amount,
			final_price,
			currency,
			promo_code,
			promo_applied,
			promo_message,
			promo_percent,
			status,
			status_label,
			admin_note,
			handled_by,
			handled_at,
			metadata,
			created_at,
			updated_at
		) values (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24
		)
	`,
		order.ID,
		order.ProductSlug,
		order.ProductName,
		order.Category,
		order.CategoryLabel,
		order.Nickname,
		order.PeriodCode,
		order.PeriodLabel,
		order.BasePrice,
		order.DiscountAmount,
		order.FinalPrice,
		order.Currency,
		nullableString(order.Promo.Code),
		order.Promo.Applied,
		nullableString(order.Promo.Message),
		nullableInt(order.Promo.Percent),
		order.Status,
		order.StatusLabel,
		nullableString(order.AdminNote),
		nullableString(order.HandledBy),
		order.HandledAt,
		buildOrderMetadata(order),
		order.CreatedAt,
		order.UpdatedAt,
	)

	return err
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	row, err := r.fetchOrder(ctx, `
		select
			public_id,
			product_slug,
			product_name,
			category,
			category_label,
			nickname,
			period_code,
			period_label,
			base_price,
			discount_amount,
			final_price,
			currency,
			promo_code,
			promo_applied,
			promo_message,
			promo_percent,
			status,
			status_label,
			admin_note,
			handled_by,
			handled_at,
			metadata,
			created_at,
			updated_at
		from store_orders
		where public_id = $1
	`, id)
	if err != nil {
		if isNotFound(err) {
			return domain.Order{}, domain.NewNotFound("Заказ не найден.")
		}
		return domain.Order{}, err
	}

	return row, nil
}

func (r *PostgresOrderRepository) List(ctx context.Context, filter domain.OrderListFilter) ([]domain.Order, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	query := "%" + strings.ToLower(strings.TrimSpace(filter.Query)) + "%"
	status := strings.TrimSpace(filter.Status)

	rows, err := r.pool.Query(ctx, `
		select
			public_id,
			product_slug,
			product_name,
			category,
			category_label,
			nickname,
			period_code,
			period_label,
			base_price,
			discount_amount,
			final_price,
			currency,
			promo_code,
			promo_applied,
			promo_message,
			promo_percent,
			status,
			status_label,
			admin_note,
			handled_by,
			handled_at,
			metadata,
			created_at,
			updated_at
		from store_orders
		where ($1 = '' or status = $1)
		  and ($2 = '%%' or lower(public_id) like $2 or lower(nickname) like $2 or lower(product_name) like $2 or lower(product_slug) like $2)
		order by created_at desc
		limit $3
	`, status, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, order)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return items, nil
}

func (r *PostgresOrderRepository) Update(ctx context.Context, id string, update domain.OrderStatusUpdate, handledBy string) (domain.Order, error) {
	now := time.Now().UTC()

	row, err := r.fetchOrder(ctx, `
		update store_orders
		set
			status = $2,
			status_label = $3,
			admin_note = $4,
			handled_by = case when $5 = '' then handled_by else $5 end,
			handled_at = case when $5 = '' then handled_at else $6 end,
			updated_at = $6
		where public_id = $1
		returning
			public_id,
			product_slug,
			product_name,
			category,
			category_label,
			nickname,
			period_code,
			period_label,
			base_price,
			discount_amount,
			final_price,
			currency,
			promo_code,
			promo_applied,
			promo_message,
			promo_percent,
			status,
			status_label,
			admin_note,
			handled_by,
			handled_at,
			metadata,
			created_at,
			updated_at
	`, id, update.Status, update.StatusLabel, nullableString(update.AdminNote), handledBy, now)
	if err != nil {
		if isNotFound(err) {
			return domain.Order{}, domain.NewNotFound("Заказ не найден.")
		}
		return domain.Order{}, err
	}

	return row, nil
}

func (r *PostgresOrderRepository) fetchOrder(ctx context.Context, sql string, args ...any) (domain.Order, error) {
	row := r.pool.QueryRow(ctx, sql, args...)
	return scanOrder(row)
}

type orderScanner interface {
	Scan(dest ...any) error
}

func scanOrder(row orderScanner) (domain.Order, error) {
	var order domain.Order
	var promoCode *string
	var promoMessage *string
	var promoPercent *int
	var adminNote *string
	var handledBy *string
	var handledAt *time.Time
	var metadata []byte

	err := row.Scan(
		&order.ID,
		&order.ProductSlug,
		&order.ProductName,
		&order.Category,
		&order.CategoryLabel,
		&order.Nickname,
		&order.PeriodCode,
		&order.PeriodLabel,
		&order.BasePrice,
		&order.DiscountAmount,
		&order.FinalPrice,
		&order.Currency,
		&promoCode,
		&order.Promo.Applied,
		&promoMessage,
		&promoPercent,
		&order.Status,
		&order.StatusLabel,
		&adminNote,
		&handledBy,
		&handledAt,
		&metadata,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return domain.Order{}, err
	}

	if promoCode != nil {
		order.Promo.Code = *promoCode
	}
	if promoMessage != nil {
		order.Promo.Message = *promoMessage
	}
	if promoPercent != nil {
		order.Promo.Percent = *promoPercent
	}
	if adminNote != nil {
		order.AdminNote = *adminNote
	}
	if handledBy != nil {
		order.HandledBy = *handledBy
	}
	order.HandledAt = handledAt
	applyOrderMetadata(&order, metadata)

	return order, nil
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	value = strings.TrimSpace(value)
	return &value
}

func nullableInt(value int) *int {
	if value == 0 {
		return nil
	}

	return &value
}
