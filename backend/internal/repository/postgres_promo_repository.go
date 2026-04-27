// Автор: Kuruma
package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"holo-site-backend/internal/domain"
)

type PostgresPromoRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPromoRepository(pool *pgxpool.Pool) *PostgresPromoRepository {
	return &PostgresPromoRepository{pool: pool}
}

func (r *PostgresPromoRepository) Load(ctx context.Context) ([]domain.PromoCode, error) {
	rows, err := r.pool.Query(ctx, `
		select
			code,
			discount_percent,
			is_active,
			usage_limit,
			times_used,
			starts_at,
			ends_at,
			created_at,
			updated_at
		from store_promo_codes
		order by code asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promos := make([]domain.PromoCode, 0)
	for rows.Next() {
		var promo domain.PromoCode
		if err := rows.Scan(
			&promo.Code,
			&promo.DiscountPercent,
			&promo.IsActive,
			&promo.UsageLimit,
			&promo.TimesUsed,
			&promo.StartsAt,
			&promo.EndsAt,
			&promo.CreatedAt,
			&promo.UpdatedAt,
		); err != nil {
			return nil, err
		}

		promo.Code = strings.ToUpper(strings.TrimSpace(promo.Code))
		promos = append(promos, promo)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return promos, nil
}

func (r *PostgresPromoRepository) SaveAll(ctx context.Context, promos []domain.PromoCode) error {
	return withTransaction(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `truncate table store_promo_codes`); err != nil {
			return err
		}

		sortedPromos := append([]domain.PromoCode(nil), promos...)
		sort.SliceStable(sortedPromos, func(i, j int) bool {
			return sortedPromos[i].Code < sortedPromos[j].Code
		})

		now := time.Now().UTC()
		for _, promo := range sortedPromos {
			createdAt := promo.CreatedAt
			if createdAt.IsZero() {
				createdAt = now
			}

			updatedAt := promo.UpdatedAt
			if updatedAt.IsZero() {
				updatedAt = now
			}

			if _, err := tx.Exec(ctx, `
				insert into store_promo_codes (
					code,
					discount_percent,
					is_active,
					usage_limit,
					times_used,
					starts_at,
					ends_at,
					created_at,
					updated_at
				) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				strings.ToUpper(strings.TrimSpace(promo.Code)),
				promo.DiscountPercent,
				promo.IsActive,
				promo.UsageLimit,
				promo.TimesUsed,
				promo.StartsAt,
				promo.EndsAt,
				createdAt,
				updatedAt,
			); err != nil {
				return err
			}
		}

		return nil
	})
}
