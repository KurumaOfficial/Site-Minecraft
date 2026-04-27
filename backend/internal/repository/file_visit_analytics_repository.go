// Автор: Kuruma
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"holo-site-backend/internal/domain"
)

type FileVisitAnalyticsRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileVisitAnalyticsRepository(dataDir string) (*FileVisitAnalyticsRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &FileVisitAnalyticsRepository{
		path: filepath.Join(dataDir, "visit-analytics.json"),
	}, nil
}

func (r *FileVisitAnalyticsRepository) Load(_ context.Context) (domain.VisitAnalyticsState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.VisitAnalyticsState{Days: map[string]domain.VisitAnalyticsDay{}}, nil
	}
	if err != nil {
		return domain.VisitAnalyticsState{}, err
	}

	if len(content) == 0 {
		return domain.VisitAnalyticsState{Days: map[string]domain.VisitAnalyticsDay{}}, nil
	}

	var state domain.VisitAnalyticsState
	if err := json.Unmarshal(content, &state); err != nil {
		return domain.VisitAnalyticsState{}, err
	}
	if state.Days == nil {
		state.Days = map[string]domain.VisitAnalyticsDay{}
	}
	return state, nil
}

func (r *FileVisitAnalyticsRepository) Save(_ context.Context, state domain.VisitAnalyticsState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if state.Days == nil {
		state.Days = map[string]domain.VisitAnalyticsDay{}
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.path)
}
