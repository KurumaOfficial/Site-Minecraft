package service

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/repository"
)

const visitAnalyticsRetentionDays = 90

var visitPageLabels = map[string]string{
	"home":     "Главная",
	"rules":    "Правила",
	"contacts": "Контакты",
}

type VisitAnalyticsService struct {
	repo  repository.VisitAnalyticsRepository
	mu    sync.RWMutex
	state domain.VisitAnalyticsState
}

func NewVisitAnalyticsService(ctx context.Context, repo repository.VisitAnalyticsRepository) (*VisitAnalyticsService, error) {
	service := &VisitAnalyticsService{
		repo: repo,
		state: domain.VisitAnalyticsState{
			Days: map[string]domain.VisitAnalyticsDay{},
		},
	}

	if err := service.bootstrap(ctx); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *VisitAnalyticsService) bootstrap(ctx context.Context) error {
	state, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	if state.Days == nil {
		state.Days = map[string]domain.VisitAnalyticsDay{}
	}

	s.state = state
	if s.trimLocked(time.Now().UTC()) {
		return s.repo.Save(ctx, s.state)
	}

	return nil
}

func (s *VisitAnalyticsService) Track(ctx context.Context, event domain.VisitEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	page := normalizeVisitPage(event.Page, event.Path)
	if page == "" {
		return nil
	}

	visitorID := strings.TrimSpace(event.VisitorID)
	if len(visitorID) < 8 || len(visitorID) > 128 {
		return nil
	}

	now := time.Now().UTC()
	dateKey := now.Format("2006-01-02")
	day := s.state.Days[dateKey]
	if day.Date == "" {
		day = domain.VisitAnalyticsDay{
			Date:         dateKey,
			PageViews:    map[string]int{},
			PageVisitors: map[string]map[string]bool{},
			Visitors:     map[string]bool{},
		}
	}
	if day.PageViews == nil {
		day.PageViews = map[string]int{}
	}
	if day.PageVisitors == nil {
		day.PageVisitors = map[string]map[string]bool{}
	}
	if day.Visitors == nil {
		day.Visitors = map[string]bool{}
	}

	day.Views++
	day.PageViews[page]++
	if day.PageVisitors[page] == nil {
		day.PageVisitors[page] = map[string]bool{}
	}
	day.PageVisitors[page][visitorID] = true
	day.Visitors[visitorID] = true
	s.state.Days[dateKey] = day
	s.trimLocked(now)

	return s.repo.Save(ctx, s.state)
}

func (s *VisitAnalyticsService) Summary() domain.VisitAnalyticsSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	todayKey := now.Format("2006-01-02")
	rangeStart := now.AddDate(0, 0, -6)

	orderedDates := make([]string, 0, len(s.state.Days))
	for dateKey := range s.state.Days {
		orderedDates = append(orderedDates, dateKey)
	}
	sort.Strings(orderedDates)

	pageViews := map[string]int{}
	pageVisitors := map[string]map[string]bool{}
	totalVisitors := map[string]bool{}
	last7Visitors := map[string]bool{}
	trendByDate := map[string]domain.VisitTrendPoint{}
	totalViews := 0
	todayViews := 0
	todayUnique := 0
	last7Views := 0

	for _, dateKey := range orderedDates {
		day := s.state.Days[dateKey]
		totalViews += day.Views

		for visitorID := range day.Visitors {
			totalVisitors[visitorID] = true
		}

		dayTime, err := time.Parse("2006-01-02", dateKey)
		if err != nil {
			continue
		}

		if dateKey == todayKey {
			todayViews = day.Views
			todayUnique = len(day.Visitors)
		}

		if dayTime.Before(rangeStart) {
			continue
		}

		last7Views += day.Views
		for visitorID := range day.Visitors {
			last7Visitors[visitorID] = true
		}
		for page, views := range day.PageViews {
			pageViews[page] += views
			if pageVisitors[page] == nil {
				pageVisitors[page] = map[string]bool{}
			}
			for visitorID := range day.PageVisitors[page] {
				pageVisitors[page][visitorID] = true
			}
		}

		trendByDate[dateKey] = domain.VisitTrendPoint{
			Date:   dateKey,
			Label:  dayTime.Format("02.01"),
			Views:  day.Views,
			Unique: len(day.Visitors),
		}
	}

	trend := make([]domain.VisitTrendPoint, 0, 7)
	for offset := 6; offset >= 0; offset-- {
		dayTime := now.AddDate(0, 0, -offset)
		dateKey := dayTime.Format("2006-01-02")
		if point, found := trendByDate[dateKey]; found {
			trend = append(trend, point)
			continue
		}
		trend = append(trend, domain.VisitTrendPoint{
			Date:   dateKey,
			Label:  dayTime.Format("02.01"),
			Views:  0,
			Unique: 0,
		})
	}

	pages := make([]domain.PageVisitMetric, 0, len(pageViews))
	for _, page := range []string{"home", "rules", "contacts"} {
		views := pageViews[page]
		if views == 0 {
			continue
		}
		pages = append(pages, domain.PageVisitMetric{
			Page:   page,
			Label:  visitPageLabels[page],
			Views:  views,
			Unique: len(pageVisitors[page]),
		})
	}
	slices.SortFunc(pages, func(left, right domain.PageVisitMetric) int {
		if left.Views == right.Views {
			return strings.Compare(left.Page, right.Page)
		}
		return right.Views - left.Views
	})

	return domain.VisitAnalyticsSummary{
		TodayViews:      todayViews,
		TodayUnique:     todayUnique,
		Last7DaysViews:  last7Views,
		Last7DaysUnique: len(last7Visitors),
		TotalViews:      totalViews,
		TotalUnique:     len(totalVisitors),
		Trend:           trend,
		Pages:           pages,
		UpdatedAt:       now,
	}
}

func (s *VisitAnalyticsService) trimLocked(now time.Time) bool {
	if s.state.Days == nil {
		s.state.Days = map[string]domain.VisitAnalyticsDay{}
	}

	changed := false
	cutoff := now.AddDate(0, 0, -(visitAnalyticsRetentionDays - 1))
	for dateKey := range s.state.Days {
		dayTime, err := time.Parse("2006-01-02", dateKey)
		if err != nil || dayTime.Before(cutoff) {
			delete(s.state.Days, dateKey)
			changed = true
		}
	}

	return changed
}

func normalizeVisitPage(page, path string) string {
	normalizedPage := strings.TrimSpace(strings.ToLower(page))
	if _, found := visitPageLabels[normalizedPage]; found {
		return normalizedPage
	}

	switch strings.TrimSpace(strings.ToLower(path)) {
	case "/", "":
		return "home"
	case "/rules":
		return "rules"
	case "/contacts":
		return "contacts"
	default:
		return ""
	}
}
