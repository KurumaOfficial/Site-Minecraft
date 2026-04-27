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

type FilePromoRepository struct {
	path string
	mu   sync.Mutex
}

func NewFilePromoRepository(dataDir string) (*FilePromoRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &FilePromoRepository{
		path: filepath.Join(dataDir, "promos.json"),
	}, nil
}

func (r *FilePromoRepository) Load(_ context.Context) ([]domain.PromoCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.PromoCode{}, nil
	}
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []domain.PromoCode{}, nil
	}

	var promos []domain.PromoCode
	if err := json.Unmarshal(content, &promos); err != nil {
		return nil, err
	}

	return promos, nil
}

func (r *FilePromoRepository) SaveAll(_ context.Context, promos []domain.PromoCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	encoded, err := json.MarshalIndent(promos, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.path)
}
