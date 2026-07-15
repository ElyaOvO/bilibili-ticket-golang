package notify_service

import "bilibili-ticket-golang/cmd/gui/i18n"

func notifyChannelTypes() []NotifyChannelTypeMeta {
	return []NotifyChannelTypeMeta{
		{Type: "gotify", Label: "Gotify", Fields: []NotifyChannelFieldMeta{
			{Key: "endpoint", Label: i18n.T("notify.field.endpoint", nil), Type: "url", Placeholder: "https://gotify.example.com", Required: true},
			{Key: "token", Label: "Token / API Key", Type: "password", Placeholder: i18n.T("notify.field.token_placeholder_gotify", nil), Required: true},
		}},
		{Type: "pushplus", Label: "PushPlus", Fields: []NotifyChannelFieldMeta{
			{Key: "token", Label: "Token / API Key", Type: "password", Placeholder: i18n.T("notify.field.token_placeholder_pushplus", nil), Required: true},
		}},
		{Type: "Bark", Label: "Bark", Fields: []NotifyChannelFieldMeta{
			{Key: "endpoint", Label: i18n.T("notify.field.endpoint", nil), Type: "url", Placeholder: "https://api.day.app", Required: true, Default: "https://api.day.app"},
			{Key: "token", Label: "Token / API Key", Type: "password", Placeholder: i18n.T("notify.field.token_placeholder_bark", nil), Required: true},
		}},
		{Type: "ntfy", Label: "ntfy", Fields: []NotifyChannelFieldMeta{
			{Key: "endpoint", Label: i18n.T("notify.field.endpoint", nil), Type: "url", Placeholder: "https://ntfy.sh", Required: true, Default: "https://ntfy.sh"},
			{Key: "topic", Label: "Topic", Type: "text", Placeholder: i18n.T("notify.field.topic_placeholder_ntfy", nil), Required: true},
			{Key: "auth_method", Label: i18n.T("notify.field.auth_method", nil), Type: "select", Options: []SelectOption{
				{Label: i18n.T("notify.field.auth_none", nil), Value: ""},
				{Label: "Access Token (Bearer)", Value: "token"},
				{Label: i18n.T("notify.field.auth_basic", nil), Value: "basic"},
			}},
			{Key: "token", Label: "Access Token", Type: "password", Placeholder: i18n.T("notify.field.token_placeholder_ntfy", nil), DependsOn: &FieldCondition{Key: "auth_method", Value: "token"}},
			{Key: "username", Label: i18n.T("notify.field.username", nil), Type: "text", Placeholder: i18n.T("notify.field.username_placeholder_ntfy", nil), DependsOn: &FieldCondition{Key: "auth_method", Value: "basic"}},
			{Key: "password", Label: i18n.T("notify.field.password", nil), Type: "password", Placeholder: i18n.T("notify.field.password_placeholder_ntfy", nil), DependsOn: &FieldCondition{Key: "auth_method", Value: "basic"}},
		}},
	}
}
