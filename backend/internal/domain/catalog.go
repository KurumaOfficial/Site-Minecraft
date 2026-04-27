// Автор: Kuruma
package domain

type PeriodOption struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Price   int    `json:"price"`
	Default bool   `json:"default"`
}

type CatalogItem struct {
	ID            string         `json:"id"`
	ExternalID    string         `json:"externalId,omitempty"`
	Slug          string         `json:"slug"`
	Name          string         `json:"name"`
	Category      string         `json:"category"`
	CategoryLabel string         `json:"categoryLabel"`
	Price         int            `json:"price"`
	Currency      string         `json:"currency"`
	Image         string         `json:"image"`
	Summary       string         `json:"summary"`
	Highlights    []string       `json:"highlights"`
	Periods       []PeriodOption `json:"periods"`
	VariablePrice bool           `json:"variablePrice,omitempty"`
	MinQuantity   int            `json:"minQuantity,omitempty"`
	MaxQuantity   int            `json:"maxQuantity,omitempty"`
	QuantityStep  int            `json:"quantityStep,omitempty"`
	UnitLabel     string         `json:"unitLabel,omitempty"`
	Sort          int            `json:"sort"`
	IsActive      bool           `json:"isActive"`
}

type CatalogItemInput struct {
	ID            string   `json:"id"`
	ExternalID    string   `json:"externalId"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	CategoryLabel string   `json:"categoryLabel"`
	Price         int      `json:"price"`
	Currency      string   `json:"currency"`
	Image         string   `json:"image"`
	Summary       string   `json:"summary"`
	Highlights    []string `json:"highlights"`
	Sort          int      `json:"sort"`
	IsActive      bool     `json:"isActive"`
}
