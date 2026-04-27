// Автор: Kuruma
package repository

import (
	"context"
	"net/url"
	"sort"

	"holo-site-backend/internal/domain"
)

type SupabaseCatalogRepository struct {
	client *SupabaseRESTClient
}

type supabaseCatalogItemRow struct {
	ID            string `json:"id"`
	ExternalID    string `json:"external_id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	Summary       string `json:"summary"`
	Image         string `json:"image"`
	Currency      string `json:"currency"`
	SortOrder     int    `json:"sort_order"`
	IsActive      bool   `json:"is_active"`
}

type supabaseCatalogPeriodRow struct {
	ItemID    string `json:"item_id"`
	Code      string `json:"code"`
	Label     string `json:"label"`
	Price     int    `json:"price"`
	IsDefault bool   `json:"is_default"`
	SortOrder int    `json:"sort_order"`
}

type supabaseCatalogHighlightRow struct {
	ItemID    string `json:"item_id"`
	SortOrder int    `json:"sort_order"`
	Text      string `json:"text"`
}

func NewSupabaseCatalogRepository(client *SupabaseRESTClient) *SupabaseCatalogRepository {
	return &SupabaseCatalogRepository{client: client}
}

func (r *SupabaseCatalogRepository) Load(ctx context.Context) ([]domain.CatalogItem, error) {
	itemQuery := url.Values{}
	itemQuery.Set("select", "id,external_id,slug,name,category,category_label,summary,image,currency,sort_order,is_active")
	itemQuery.Set("order", "sort_order.asc,name.asc")

	periodQuery := url.Values{}
	periodQuery.Set("select", "item_id,code,label,price,is_default,sort_order")
	periodQuery.Set("order", "sort_order.asc,code.asc")

	highlightQuery := url.Values{}
	highlightQuery.Set("select", "item_id,sort_order,text")
	highlightQuery.Set("order", "sort_order.asc,id.asc")

	var itemRows []supabaseCatalogItemRow
	if err := r.client.Get(ctx, "store_catalog_items", itemQuery, &itemRows); err != nil {
		return nil, err
	}

	var periodRows []supabaseCatalogPeriodRow
	if err := r.client.Get(ctx, "store_catalog_periods", periodQuery, &periodRows); err != nil {
		return nil, err
	}

	var highlightRows []supabaseCatalogHighlightRow
	if err := r.client.Get(ctx, "store_catalog_highlights", highlightQuery, &highlightRows); err != nil {
		return nil, err
	}

	items := make([]domain.CatalogItem, 0, len(itemRows))
	itemsByDBID := make(map[string]*domain.CatalogItem, len(itemRows))

	for _, row := range itemRows {
		item := domain.CatalogItem{
			ID:            row.ExternalID,
			ExternalID:    row.ExternalID,
			Slug:          row.Slug,
			Name:          row.Name,
			Category:      row.Category,
			CategoryLabel: row.CategoryLabel,
			Currency:      row.Currency,
			Image:         row.Image,
			Summary:       row.Summary,
			Highlights:    []string{},
			Periods:       []domain.PeriodOption{},
			Sort:          row.SortOrder,
			IsActive:      row.IsActive,
		}
		items = append(items, item)
		itemsByDBID[row.ID] = &items[len(items)-1]
	}

	for _, row := range periodRows {
		item := itemsByDBID[row.ItemID]
		if item == nil {
			continue
		}

		item.Periods = append(item.Periods, domain.PeriodOption{
			Code:    row.Code,
			Label:   row.Label,
			Price:   row.Price,
			Default: row.IsDefault,
		})
		if item.Price == 0 {
			item.Price = row.Price
		}
	}

	for _, row := range highlightRows {
		item := itemsByDBID[row.ItemID]
		if item == nil {
			continue
		}

		item.Highlights = append(item.Highlights, row.Text)
	}

	return items, nil
}

func (r *SupabaseCatalogRepository) SaveAll(ctx context.Context, items []domain.CatalogItem) error {
	deleteQuery := url.Values{}
	deleteQuery.Set("id", "not.is.null")
	if err := r.client.Delete(ctx, "store_catalog_items", deleteQuery); err != nil {
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

		payload := []map[string]any{{
			"external_id":    externalID,
			"slug":           item.Slug,
			"name":           item.Name,
			"category":       item.Category,
			"category_label": item.CategoryLabel,
			"summary":        item.Summary,
			"image":          item.Image,
			"currency":       item.Currency,
			"sort_order":     item.Sort,
			"is_active":      item.IsActive,
		}}

		var inserted []supabaseCatalogItemRow
		if err := r.client.Post(ctx, "store_catalog_items", nil, payload, &inserted); err != nil {
			return err
		}
		if len(inserted) == 0 {
			return &SupabaseRequestError{Message: "catalog insert returned no rows"}
		}

		itemID := inserted[0].ID
		periods := item.Periods
		if len(periods) == 0 {
			periods = []domain.PeriodOption{
				{Code: "forever", Label: "Навсегда", Price: item.Price, Default: true},
			}
		}

		periodPayload := make([]map[string]any, 0, len(periods))
		for index, period := range periods {
			periodPayload = append(periodPayload, map[string]any{
				"item_id":    itemID,
				"code":       period.Code,
				"label":      period.Label,
				"price":      period.Price,
				"is_default": period.Default,
				"sort_order": index + 1,
			})
		}
		if err := r.client.Post(ctx, "store_catalog_periods", nil, periodPayload, nil); err != nil {
			return err
		}

		if len(item.Highlights) == 0 {
			continue
		}

		highlightPayload := make([]map[string]any, 0, len(item.Highlights))
		for index, highlight := range item.Highlights {
			highlightPayload = append(highlightPayload, map[string]any{
				"item_id":    itemID,
				"sort_order": index + 1,
				"text":       highlight,
			})
		}
		if err := r.client.Post(ctx, "store_catalog_highlights", nil, highlightPayload, nil); err != nil {
			return err
		}
	}

	return nil
}
