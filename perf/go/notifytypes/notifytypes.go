package notifytypes

// Type is the type of notifiers that can be built.
type Type string

const (
	// HTMLEmail means send HTML formatted emails.
	HTMLEmail Type = "html_email"

	// None means do not send any notification.
	None Type = "none"
)

// AllNotifierTypes is the list of all valid NotifyType's.
var AllNotifierTypes []Type = []Type{HTMLEmail, None}
