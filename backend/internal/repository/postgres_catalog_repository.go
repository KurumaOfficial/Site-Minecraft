package repository

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"holo-site-backend/internal/domain"
)

type PostgresCatalogRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCatalogRepository(pool *pgxpool.Pool) *PostgresCatalogRepository {
	return &PostgresCatalogRepository{pool: pool}
}

func (r *PostgresCatalogRepository) Load(ctx context.Context) ([]domain.CatalogItem, error) {
	rows, err := r.pool.Query(ctx, `
		select
			external_id,
			slug,
			name,
			category,
			category_label,
			summary,
			image,
			currency,
			sort_order,
			is_active
		from store_catalog_items
		order by sort_order asc, name asc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.CatalogItem, 0)
	itemsBySlug := make(map[string]*domain.CatalogItem)

	for rows.Next() {
		var item domain.CatalogItem
		if err := rows.Scan(
			&item.ExternalID,
			&item.Slug,
			&item.Name,
			&item.Category,
			&item.CategoryLabel,
			&item.Summary,
			&item.Image,
			&item.Currency,
			&item.Sort,
			&item.IsActive,
		); err != nil {
			return nil, err
		}

		item.ID = item.ExternalID
		item.Price = 0
		item.Highlights = []string{}
		item.Periods = []domain.PeriodOption{}
		items = append(items, item)
		itemsBySlug[item.Slug] = &items[len(items)-1]
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	periodRows, err := r.pool.Query(ctx, `
		select
			i.slug,
			p.code,
			p.label,
			p.price,
			p.is_default
		from store_catalog_items i
		join store_catalog_periods p on p.item_id = i.id
		order by i.sort_order asc, p.sort_order asc, p.code asc
	`)
	if err != nil {
		return nil, err
	}
	defer periodRows.Close()

	for periodRows.Next() {
		var slug string
		var period domain.PeriodOption
		if err := periodRows.Scan(&slug, &period.Code, &period.Label, &period.Price, &period.Default); err != nil {
			return nil, err
		}

		item := itemsBySlug[slug]
		if item == nil {
			continue
		}
		item.Periods = append(item.Periods, period)
		if item.Price == 0 {
			item.Price = period.Price
		}
	}

	if periodRows.Err() != nil {
		return nil, periodRows.Err()
	}

	highlightRows, err := r.pool.Query(ctx, `
		select
			i.slug,
			h.text
		from store_catalog_items i
		join store_catalog_highlights h on h.item_id = i.id
		order by i.sort_order asc, h.sort_order asc, h.id asc
	`)
	if err != nil {
		return nil, err
	}
	defer highlightRows.Close()

	for highlightRows.Next() {
		var slug string
		var text string
		if err := highlightRows.Scan(&slug, &text); err != nil {
			return nil, err
		}

		item := itemsBySlug[slug]
		if item == nil {
			continue
		}
		item.Highlights = append(item.Highlights, text)
	}

	if highlightRows.Err() != nil {
		return nil, highlightRows.Err()
	}

	return items, nil
}

func (r *PostgresCatalogRepository) SaveAll(ctx context.Context, items []domain.CatalogItem) error {
	return withTransaction(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `truncate table store_catalog_highlights restart identity cascade`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `truncate table store_catalog_periods cascade`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `truncate table store_catalog_items cascade`); err != nil {
			return err
		}

		sortedItems := append([]domain.CatalogItem(nil), items...)
		sort.SliceStable(sortedItems, func(i, j int) bool {
			return sortedItems[i].Sort < sortedItems[j].Sort
		})

		for _, item := range sortedItems {
			externalID := item.ExternalID
			if externalID == "" {
				externalID = item.Slug
			}

			var itemID string
			if err := tx.QueryRow(ctx, `
				insert into store_catalog_items (
					external_id,
					slug,
					name,
					category,
					category_label,
					summary,
					image,
					currency,
					sort_order,
					is_active
				) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				returning id
			`,
				externalID,
				item.Slug,
				item.Name,
				item.Category,
				item.CategoryLabel,
				item.Summary,
				item.Image,
				item.Currency,
				item.Sort,
				item.IsActive,
			).Scan(&itemID); err != nil {
				return err
			}

			periods := item.Periods
			if len(periods) == 0 {
				periods = []domain.PeriodOption{
					{Code: "forever", Label: "Навсегда", Price: item.Price, Default: true},
				}
			}

			for index, period := range periods {
				if _, err := tx.Exec(ctx, `
					insert into store_catalog_periods (
						item_id,
						code,
						label,
						price,
						is_default,
						sort_order
					) values ($1, $2, $3, $4, $5, $6)
				`,
					itemID,
					period.Code,
					period.Label,
					period.Price,
					period.Default,
					index+1,
				); err != nil {
					return err
				}
			}

			for index, highlight := range item.Highlights {
				if _, err := tx.Exec(ctx, `
					insert into store_catalog_highlights (item_id, sort_order, text)
					values ($1, $2, $3)
				`, itemID, index+1, highlight); err != nil {
					return err
				}
			}
		}

		return nil
	})
}
