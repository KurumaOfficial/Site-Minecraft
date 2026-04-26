package domain

import "time"

type VisitEvent struct {
	Page      string `json:"page"`
	Path      string `json:"path,omitempty"`
	VisitorID string `json:"visitorId,omitempty"`
	Referrer  string `json:"referrer,omitempty"`
}

type VisitAnalyticsDay struct {
	Date         string                    `json:"date"`
	Views        int                       `json:"views"`
	PageViews    map[string]int            `json:"pageViews"`
	PageVisitors map[string]map[string]bool `json:"pageVisitors"`
	Visitors     map[string]bool           `json:"visitors"`
}

type VisitAnalyticsState struct {
	Days map[string]VisitAnalyticsDay `json:"days"`
}

type VisitTrendPoint struct {
	Date   string `json:"date"`
	Label  string `json:"label"`
	Views  int    `json:"views"`
	Unique int    `json:"unique"`
}

type PageVisitMetric struct {
	Page   string `json:"page"`
	Label  string `json:"label"`
	Views  int    `json:"views"`
	Unique int    `json:"unique"`
}

type VisitAnalyticsSummary struct {
	TodayViews       int               `json:"todayViews"`
	TodayUnique      int               `json:"todayUnique"`
	Last7DaysViews   int               `json:"last7DaysViews"`
	Last7DaysUnique  int               `json:"last7DaysUnique"`
	TotalViews       int               `json:"totalViews"`
	TotalUnique      int               `json:"totalUnique"`
	Trend            []VisitTrendPoint `json:"trend"`
	Pages            []PageVisitMetric `json:"pages"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}
