package repository

import (
	"encoding/json"
	"strings"

	"holo-site-backend/internal/domain"
)

type orderMetadata struct {
	Quantity      int    `json:"quantity"`
	UnitPrice     int    `json:"unit_price"`
	UnitLabel     string `json:"unit_label"`
	VariablePrice bool   `json:"variable_price"`
}

func buildOrderMetadata(order domain.Order) map[string]any {
	metadata := map[string]any{
		"quantity":       maxOrderInt(order.Quantity, 1),
		"unit_price":     maxOrderInt(order.UnitPrice, order.BasePrice),
		"variable_price": order.VariablePrice,
	}

	if label := strings.TrimSpace(order.UnitLabel); label != "" {
		metadata["unit_label"] = label
	}

	return metadata
}

func applyOrderMetadata(order *domain.Order, raw []byte) {
	if order == nil {
		return
	}

	var payload orderMetadata
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}

	order.Quantity = maxOrderInt(payload.Quantity, 1)
	order.UnitPrice = maxOrderInt(payload.UnitPrice, order.BasePrice)
	order.UnitLabel = strings.TrimSpace(payload.UnitLabel)
	order.VariablePrice = payload.VariablePrice
}

func maxOrderInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}
