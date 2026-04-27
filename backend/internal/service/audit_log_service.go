// Автор: Kuruma
package service

import (
	"context"
	"log"

	"holo-site-backend/internal/domain"
)

// AuditLogRepository — контракт хранилища журнала.
type AuditLogRepository interface {
	Append(ctx context.Context, entry domain.AuditLogEntry) (domain.AuditLogEntry, error)
	List(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLogEntry, error)
}

// AuditLogService — бизнес-логика журнала действий администраторов.
//
// Сервис намеренно не возвращает ошибки в Record(): мы НЕ блокируем
// основное действие из-за проблем с журналом. Если запись не удалась
// (диск, квота) — пишем в server log, но операция администратора
// (выдача заявки, изменение каталога) проходит штатно.
type AuditLogService struct {
	repo AuditLogRepository
}

func NewAuditLogService(repo AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

// Record фиксирует действие. Все аргументы опциональны кроме action.
func (s *AuditLogService) Record(
	ctx context.Context,
	actorID, actorName, action, subject, summary string,
	before, after map[string]any,
	ip, userAgent string,
) {
	if s == nil || s.repo == nil || action == "" {
		return
	}
	entry := domain.AuditLogEntry{
		ActorID:   actorID,
		ActorName: actorName,
		Action:    action,
		Subject:   subject,
		Summary:   summary,
		Before:    before,
		After:     after,
		IP:        ip,
		UserAgent: userAgent,
	}
	if _, err := s.repo.Append(ctx, entry); err != nil {
		log.Printf("audit: failed to append entry action=%s: %v", action, err)
	}
}

// List возвращает последние действия (с учётом фильтра).
func (s *AuditLogService) List(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLogEntry, error) {
	if s == nil || s.repo == nil {
		return []domain.AuditLogEntry{}, nil
	}
	return s.repo.List(ctx, filter)
}
