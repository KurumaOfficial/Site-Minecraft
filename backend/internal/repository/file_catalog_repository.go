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

type FileCatalogRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileCatalogRepository(dataDir string) (*FileCatalogRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &FileCatalogRepository{
		path: filepath.Join(dataDir, "catalog.json"),
	}, nil
}

func (r *FileCatalogRepository) Load(_ context.Context) ([]domain.CatalogItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.CatalogItem{}, nil
	}
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []domain.CatalogItem{}, nil
	}

	var items []domain.CatalogItem
	if err := json.Unmarshal(content, &items); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *FileCatalogRepository) SaveAll(_ context.Context, items []domain.CatalogItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	encoded, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.path)
}
