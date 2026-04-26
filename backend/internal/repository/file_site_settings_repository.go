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

type FileSiteSettingsRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileSiteSettingsRepository(dataDir string) (*FileSiteSettingsRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &FileSiteSettingsRepository{
		path: filepath.Join(dataDir, "site-settings.json"),
	}, nil
}

func (r *FileSiteSettingsRepository) Load(_ context.Context) (domain.SiteSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.SiteSettings{}, nil
	}
	if err != nil {
		return domain.SiteSettings{}, err
	}

	if len(content) == 0 {
		return domain.SiteSettings{}, nil
	}

	var settings domain.SiteSettings
	if err := json.Unmarshal(content, &settings); err != nil {
		return domain.SiteSettings{}, err
	}

	return settings, nil
}

func (r *FileSiteSettingsRepository) Save(_ context.Context, settings domain.SiteSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.path)
}
