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

// IntegrationRepository хранит настройки внешних webhook-интеграций и
// выбранного платёжного провайдера. На текущий момент используется
// файловая реализация (data/integrations.json) — переезд в Supabase можно
// сделать позже без изменения интерфейса.
type IntegrationRepository interface {
	Load(ctx context.Context) (IntegrationsBundle, error)
	SaveIntegrations(ctx context.Context, integrations domain.IntegrationsConfig) error
	SavePayments(ctx context.Context, payments domain.PaymentSettings) error
}

// IntegrationsBundle — то, что лежит в одном json-файле: webhook'и + платежи.
type IntegrationsBundle struct {
	Integrations domain.IntegrationsConfig `json:"integrations"`
	Payments     domain.PaymentSettings    `json:"payments"`
}

type FileIntegrationRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileIntegrationRepository(dataDir string) (*FileIntegrationRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	return &FileIntegrationRepository{
		path: filepath.Join(dataDir, "integrations.json"),
	}, nil
}

func (r *FileIntegrationRepository) Load(_ context.Context) (IntegrationsBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return IntegrationsBundle{}, nil
	}
	if err != nil {
		return IntegrationsBundle{}, err
	}
	if len(content) == 0 {
		return IntegrationsBundle{}, nil
	}

	var bundle IntegrationsBundle
	if err := json.Unmarshal(content, &bundle); err != nil {
		return IntegrationsBundle{}, err
	}

	return bundle, nil
}

func (r *FileIntegrationRepository) SaveIntegrations(ctx context.Context, integrations domain.IntegrationsConfig) error {
	bundle, err := r.Load(ctx)
	if err != nil {
		return err
	}
	bundle.Integrations = integrations
	return r.write(bundle)
}

func (r *FileIntegrationRepository) SavePayments(ctx context.Context, payments domain.PaymentSettings) error {
	bundle, err := r.Load(ctx)
	if err != nil {
		return err
	}
	bundle.Payments = payments
	return r.write(bundle)
}

func (r *FileIntegrationRepository) write(bundle IntegrationsBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.path)
}
