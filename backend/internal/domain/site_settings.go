// Автор: Kuruma
package domain

type SiteSettings struct {
	ContactTitle       string `json:"contactTitle"`
	DiscordURL         string `json:"discordUrl"`
	DiscordText        string `json:"discordText"`
	DiscordButtonLabel string `json:"discordButtonLabel"`
	VKURL              string `json:"vkUrl"`
	VKText             string `json:"vkText"`
	VKButtonLabel      string `json:"vkButtonLabel"`
	SupportEmail       string `json:"supportEmail"`
	SupportText        string `json:"supportText"`
}

type SiteSettingsInput struct {
	ContactTitle       string `json:"contactTitle"`
	DiscordURL         string `json:"discordUrl"`
	DiscordText        string `json:"discordText"`
	DiscordButtonLabel string `json:"discordButtonLabel"`
	VKURL              string `json:"vkUrl"`
	VKText             string `json:"vkText"`
	VKButtonLabel      string `json:"vkButtonLabel"`
	SupportEmail       string `json:"supportEmail"`
	SupportText        string `json:"supportText"`
}
