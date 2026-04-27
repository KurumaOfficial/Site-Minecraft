// Автор: Kuruma
package domain

import "time"

// AuditLogEntry — запись журнала действий администраторов.
// Пишется при каждой мутации в админ-панели (создание/изменение/удаление
// заявок, каталога, промокодов, настроек оплаты и интеграций, логин/логаут).
//
// Поля:
//   - ID: уникальный идентификатор записи (ULID-like).
//   - At: момент действия (UTC).
//   - ActorID: Discord ID администратора (или "local-bypass" для локальной
//     разработки), позволяет соотнести действие с конкретным пользователем,
//     даже если он сменил никнейм.
//   - ActorName: человекочитаемое имя на момент действия.
//   - Action: машинный код действия в формате "subject.verb"
//     (например, "order.update", "catalog.save", "integrations.test").
//   - Subject: краткий человекочитаемый идентификатор объекта,
//     над которым совершено действие (slug товара, ID заявки, провайдер).
//   - Summary: одной строкой что именно произошло.
//   - Before/After: JSON-снимки до и после (для diff'а в UI).
//   - IP/UserAgent: для расследования инцидентов.
type AuditLogEntry struct {
	ID        string         `json:"id"`
	At        time.Time      `json:"at"`
	ActorID   string         `json:"actorId"`
	ActorName string         `json:"actorName"`
	Action    string         `json:"action"`
	Subject   string         `json:"subject,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
	IP        string         `json:"ip,omitempty"`
	UserAgent string         `json:"userAgent,omitempty"`
}

// AuditLogFilter ограничивает выдачу журнала.
type AuditLogFilter struct {
	Limit   int    // 0 ⇒ дефолт (200), макс. 1000
	Action  string // префикс: "order." вернёт все order.*
	ActorID string // конкретный администратор
}
