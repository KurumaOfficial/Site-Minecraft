// Автор: Kuruma
package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"holo-site-backend/internal/domain"
)

type FileOrderRepository struct {
	snapshotPath        string
	logPath             string
	snapshotEvery       int
	writesSinceSnapshot int
	file                *os.File

	mu         sync.RWMutex
	orders     []domain.Order
	ordersByID map[string]domain.Order
}

type storedOrders struct {
	Orders []domain.Order `json:"orders"`
}

func NewFileOrderRepository(dataDir string, snapshotEvery int) (*FileOrderRepository, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	if snapshotEvery <= 0 {
		snapshotEvery = 32
	}

	repository := &FileOrderRepository{
		snapshotPath:  filepath.Join(dataDir, "orders.json"),
		logPath:       filepath.Join(dataDir, "orders.jsonl"),
		snapshotEvery: snapshotEvery,
		orders:        make([]domain.Order, 0),
		ordersByID:    make(map[string]domain.Order),
	}

	if err := repository.bootstrap(); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *FileOrderRepository) Save(_ context.Context, order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.ordersByID[order.ID]; exists {
		return domain.NewInternal("Конфликт идентификатора заказа.")
	}

	order.UpdatedAt = order.CreatedAt

	if err := r.appendOrderLocked(order); err != nil {
		return err
	}

	r.orders = append(r.orders, order)
	r.ordersByID[order.ID] = order
	r.writesSinceSnapshot++

	if r.writesSinceSnapshot >= r.snapshotEvery {
		if err := r.writeSnapshotLocked(); err == nil {
			r.writesSinceSnapshot = 0
		}
	}

	return nil
}

func (r *FileOrderRepository) GetByID(_ context.Context, id string) (domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, found := r.ordersByID[id]
	if !found {
		return domain.Order{}, domain.NewNotFound("Заказ не найден.")
	}

	return order, nil
}

func (r *FileOrderRepository) List(_ context.Context, filter domain.OrderListFilter) ([]domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	status := strings.TrimSpace(filter.Status)
	items := make([]domain.Order, 0, len(r.orders))

	for i := len(r.orders) - 1; i >= 0; i-- {
		order := r.orders[i]
		if status != "" && order.Status != status {
			continue
		}

		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				order.ID,
				order.Nickname,
				order.ProductName,
				order.ProductSlug,
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}

		items = append(items, order)
		if len(items) >= limit {
			break
		}
	}

	return items, nil
}

func (r *FileOrderRepository) Update(_ context.Context, id string, update domain.OrderStatusUpdate, handledBy string) (domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, found := r.ordersByID[id]
	if !found {
		return domain.Order{}, domain.NewNotFound("Заказ не найден.")
	}

	now := time.Now().UTC()
	order.Status = update.Status
	order.StatusLabel = update.StatusLabel
	order.AdminNote = strings.TrimSpace(update.AdminNote)
	order.UpdatedAt = now

	if handledBy != "" {
		order.HandledBy = handledBy
		order.HandledAt = &now
	}

	for index := range r.orders {
		if r.orders[index].ID == id {
			r.orders[index] = order
			break
		}
	}
	r.ordersByID[id] = order

	if err := r.writeSnapshotLocked(); err != nil {
		return domain.Order{}, err
	}
	if err := r.rewriteLogLocked(); err != nil {
		return domain.Order{}, err
	}

	r.writesSinceSnapshot = 0
	return order, nil
}

func (r *FileOrderRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var snapshotErr error
	if len(r.orders) > 0 {
		snapshotErr = r.writeSnapshotLocked()
	}

	var closeErr error
	if r.file != nil {
		closeErr = r.file.Close()
	}

	if snapshotErr != nil {
		return snapshotErr
	}

	return closeErr
}

func (r *FileOrderRepository) bootstrap() error {
	orders, loadedFromLog, err := r.loadExistingOrders()
	if err != nil {
		return err
	}

	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].CreatedAt.Before(orders[j].CreatedAt)
	})

	r.orders = orders
	r.ordersByID = make(map[string]domain.Order, len(orders))
	for index, order := range orders {
		if order.UpdatedAt.IsZero() {
			order.UpdatedAt = order.CreatedAt
			r.orders[index] = order
		}
		r.ordersByID[order.ID] = order
	}

	file, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	r.file = file

	if !loadedFromLog && len(r.orders) > 0 {
		if err := r.rewriteLogLocked(); err != nil {
			return err
		}
	}

	return r.writeSnapshotLocked()
}

func (r *FileOrderRepository) loadExistingOrders() ([]domain.Order, bool, error) {
	logOrders, err := r.readLogFile()
	if err != nil {
		return nil, false, err
	}
	if len(logOrders) > 0 {
		return logOrders, true, nil
	}

	snapshotOrders, err := r.readSnapshotFile()
	if err != nil {
		return nil, false, err
	}

	return snapshotOrders, false, nil
}

func (r *FileOrderRepository) readLogFile() ([]domain.Order, error) {
	info, err := os.Stat(r.logPath)
	if err == nil && info.Size() == 0 {
		return []domain.Order{}, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Order{}, nil
	}
	if err != nil {
		return nil, err
	}

	file, err := os.Open(r.logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	orders := make([]domain.Order, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var order domain.Order
		if err := json.Unmarshal(line, &order); err != nil {
			return nil, err
		}

		if order.UpdatedAt.IsZero() {
			order.UpdatedAt = order.CreatedAt
		}
		orders = append(orders, order)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *FileOrderRepository) readSnapshotFile() ([]domain.Order, error) {
	content, err := os.ReadFile(r.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Order{}, nil
	}
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []domain.Order{}, nil
	}

	var payload storedOrders
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}

	for index := range payload.Orders {
		if payload.Orders[index].UpdatedAt.IsZero() {
			payload.Orders[index].UpdatedAt = payload.Orders[index].CreatedAt
		}
	}

	if payload.Orders == nil {
		return []domain.Order{}, nil
	}

	return payload.Orders, nil
}

func (r *FileOrderRepository) appendOrderLocked(order domain.Order) error {
	encoded, err := json.Marshal(order)
	if err != nil {
		return err
	}

	_, err = r.file.Write(append(encoded, '\n'))
	return err
}

func (r *FileOrderRepository) rewriteLogLocked() error {
	file, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	for _, order := range r.orders {
		encoded, err := json.Marshal(order)
		if err != nil {
			file.Close()
			return err
		}

		if _, err := file.Write(append(encoded, '\n')); err != nil {
			file.Close()
			return err
		}
	}

	if err := file.Close(); err != nil {
		return err
	}

	if r.file != nil {
		if err := r.file.Close(); err != nil {
			return err
		}
	}

	r.file, err = os.OpenFile(r.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	return err
}

func (r *FileOrderRepository) writeSnapshotLocked() error {
	payload := storedOrders{Orders: r.orders}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	tempPath := r.snapshotPath + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, r.snapshotPath)
}
