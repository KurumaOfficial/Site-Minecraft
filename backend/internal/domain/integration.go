package domain

import "time"

// IntegrationEndpoint описывает один внешний webhook, который вызывается
// сервером при выдаче определенной категории заказов (привилегии, кейсы,
// донат-валюта). URL и секрет хранятся только на бэкенде.
type IntegrationEndpoint struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	Token   string `json:"token,omitempty"`
}

// IntegrationsConfig объединяет webhook'и для разных категорий товаров и
// настройки исходящих интеграций с Minecraft-плагином.
type IntegrationsConfig struct {
	PrivilegesEndpoint IntegrationEndpoint `json:"privilegesEndpoint"`
	CasesEndpoint      IntegrationEndpoint `json:"casesEndpoint"`
	CurrencyEndpoint   IntegrationEndpoint `json:"currencyEndpoint"`
	WebhookSecret      string              `json:"webhookSecret,omitempty"`
	UpdatedAt          time.Time           `json:"updatedAt"`
}

// IntegrationsInput — DTO для приёма настроек из админ-панели.
type IntegrationsInput struct {
	PrivilegesEndpoint IntegrationEndpoint `json:"privilegesEndpoint"`
	CasesEndpoint      IntegrationEndpoint `json:"casesEndpoint"`
	CurrencyEndpoint   IntegrationEndpoint `json:"currencyEndpoint"`
	WebhookSecret      string              `json:"webhookSecret"`
}

// IntegrationTestRequest — запрос на тестовую отправку webhook из админки.
type IntegrationTestRequest struct {
	Category string `json:"category"`
}

// IntegrationTestResult — результат тестового запроса.
type IntegrationTestResult struct {
	OK         bool   `json:"ok"`
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// PaymentProviderID — идентификатор платёжного провайдера. "manual" =
// текущий режим выдачи через заявку, остальные значения — заготовки.
type PaymentProviderID string

const (
	PaymentProviderManual   PaymentProviderID = "manual"
	PaymentProviderYooKassa PaymentProviderID = "yookassa"
)

// PaymentSettings — настройки платёжной системы. Заготовка: пока активным
// является только провайдер "manual", остальные сохраняются и подключаются
// после реальной интеграции.
type PaymentSettings struct {
	Provider     PaymentProviderID `json:"provider"`
	TestMode     bool              `json:"testMode"`
	ShopID       string            `json:"shopId,omitempty"`
	SecretKey    string            `json:"secretKey,omitempty"`
	PublicKey    string            `json:"publicKey,omitempty"`
	ReturnURL    string            `json:"returnUrl,omitempty"`
	WebhookURL   string            `json:"webhookUrl,omitempty"`
	Description  string            `json:"description,omitempty"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// PaymentSettingsInput — DTO для приёма настроек оплаты из админки.
type PaymentSettingsInput struct {
	Provider    PaymentProviderID `json:"provider"`
	TestMode    bool              `json:"testMode"`
	ShopID      string            `json:"shopId"`
	SecretKey   string            `json:"secretKey"`
	PublicKey   string            `json:"publicKey"`
	ReturnURL   string            `json:"returnUrl"`
	WebhookURL  string            `json:"webhookUrl"`
	Description string            `json:"description"`
}

// PaymentInitResponse — ответ на инициацию платежа.
//   - Manual = true: ручная очередь (заявка попадает к админу)
//   - RedirectURL не пустой: фронт делает window.location = redirectUrl
//   - PaymentID: идентификатор платежа в провайдере (для аудита)
type PaymentInitResponse struct {
	Provider    PaymentProviderID `json:"provider"`
	OrderID     string            `json:"orderId"`
	PaymentID   string            `json:"paymentId,omitempty"`
	RedirectURL string            `json:"redirectUrl,omitempty"`
	Manual      bool              `json:"manual"`
	Message     string            `json:"message"`
}

// PaymentVerification — результат повторной проверки платежа в API
// провайдера. Backend использует его для подтверждения подлинности
// webhook'а от платежной системы.
type PaymentVerification struct {
	Provider  PaymentProviderID `json:"provider"`
	PaymentID string            `json:"paymentId"`
	OrderID   string            `json:"orderId"`
	Status    string            `json:"status"`
	Paid      bool              `json:"paid"`
	FetchedAt time.Time         `json:"fetchedAt"`
}
