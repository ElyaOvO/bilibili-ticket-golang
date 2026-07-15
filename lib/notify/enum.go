package notify

// NotificationType classifies notification backends.
type NotificationType int

const (
	None NotificationType = iota
	Gotify
	PushPlus
	Bark
	Ntfy
)

// ConvertNotificationType converts a string name to a NotificationType.
func ConvertNotificationType(name string) NotificationType {
	switch name {
	case "gotify":
		return Gotify
	case "pushplus":
		return PushPlus
	case "bark", "Bark":
		return Bark
	case "ntfy":
		return Ntfy
	default:
		return None
	}
}
