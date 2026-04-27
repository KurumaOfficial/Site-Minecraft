// Автор: Kuruma
package repository

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/domain"
)

// FileAuditLogRepository пишет журнал действий в JSONL-файл.
// Формат — append-only, по одной записи в строке: это даёт нам
// одновременно и атомарность записи (одна строка = один write(2)),
// и удобство чтения с конца (последние действия — последние строки).
type FileAuditLogRepository struct {
	path string
	mu   sync.Mutex
}

func NewFileAuditLogRepository(dataDir string) (*FileAuditLogRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	return &FileAuditLogRepository{
		path: filepath.Join(dataDir, "audit.log.jsonl"),
	}, nil
}

// Append добавляет запись в журнал. ID генерируется автоматически,
// если не задан. Метод не блокирует вызывающий код надолго —
// одна I/O-операция append.
func (r *FileAuditLogRepository) Append(_ context.Context, entry domain.AuditLogEntry) (domain.AuditLogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry.ID == "" {
		entry.ID = newAuditID()
	}
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		return entry, err
	}
	encoded = append(encoded, '\n')

	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return entry, err
	}
	defer f.Close()

	if _, err := f.Write(encoded); err != nil {
		return entry, err
	}
	return entry, nil
}

// List возвращает последние записи журнала с применением фильтра.
// Реализация читает файл целиком и переворачивает порядок —
// для production нагрузки этого хватает (десятки тысяч действий
// в день укладываются в единицы мегабайт).
func (r *FileAuditLogRepository) List(_ context.Context, filter domain.AuditLogFilter) ([]domain.AuditLogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	f, err := os.Open(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.AuditLogEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []domain.AuditLogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry domain.AuditLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// битую строку игнорируем — лучше частичный журнал, чем 500.
			continue
		}
		if filter.Action != "" && !strings.HasPrefix(entry.Action, filter.Action) {
			continue
		}
		if filter.ActorID != "" && entry.ActorID != filter.ActorID {
			continue
		}
		all = append(all, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// последние N сначала
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	out := make([]domain.AuditLogEntry, len(all))
	for i, e := range all {
		out[len(all)-1-i] = e
	}
	return out, nil
}

func newAuditID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand на linux не падает; на всякий случай fallback.
		return "audit-" + time.Now().UTC().Format("20060102T150405.000000")
	}
	return "audit-" + hex.EncodeToString(b[:])
}
