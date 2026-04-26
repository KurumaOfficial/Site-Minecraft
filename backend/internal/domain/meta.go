package domain

type ProjectMeta struct {
	ProjectName           string `json:"projectName"`
	ServerName            string `json:"serverName"`
	AppName               string `json:"appName"`
	Tagline               string `json:"tagline"`
	SiteURL               string `json:"siteUrl"`
	AdminRedirectURL      string `json:"adminRedirectUrl"`
	SupportEmail          string `json:"supportEmail"`
	DiscordURL            string `json:"discordUrl"`
	VKURL                 string `json:"vkUrl"`
	ContactTitle          string `json:"contactTitle"`
	DiscordText           string `json:"discordText"`
	DiscordButtonLabel    string `json:"discordButtonLabel"`
	VKText                string `json:"vkText"`
	VKButtonLabel         string `json:"vkButtonLabel"`
	SupportText           string `json:"supportText"`
	OfferURL              string `json:"offerUrl"`
	PrivacyURL            string `json:"privacyUrl"`
	CatalogCount          int    `json:"catalogCount"`
	PrivilegeCount        int    `json:"privilegeCount"`
	CaseCount             int    `json:"caseCount"`
	StorageMode           string `json:"storageMode"`
	DatabaseProvider      string `json:"databaseProvider"`
	DatabaseEnabled       bool   `json:"databaseEnabled"`
	SupabaseReady         bool   `json:"supabaseReady"`
	SupabaseConfigured    bool   `json:"supabaseConfigured"`
	SupabaseURL           string `json:"supabaseUrl,omitempty"`
	SupabasePublicKey     string `json:"supabasePublicKey,omitempty"`
	AdminPanelEnabled     bool   `json:"adminPanelEnabled"`
	DiscordAuthEnabled    bool   `json:"discordAuthEnabled"`
	BackendStatus         string `json:"backendStatus"`
	BackendStatusLabel    string `json:"backendStatusLabel"`
	PaymentMode           string `json:"paymentMode"`
	PaymentModeLabel      string `json:"paymentModeLabel"`
	OrderLookupEnabled    bool   `json:"orderLookupEnabled"`
	ManualFulfillmentMode bool   `json:"manualFulfillmentMode"`
}
