package notifytypes

// Type is the type of notifiers that can be built.
type Type string

const (
	// HTMLEmail means send HTML formatted emails.
	HTMLEmail Type = "html_email"

	// MarkdownIssueTracker means send Markdown formatted notifications to the
	// issue tracker.
	MarkdownIssueTracker Type = "markdown_issuetracker"

	// ChromeperfAlerting means send the regression data to chromeperf
	// alerting system
	ChromeperfAlerting Type = "chromeperf"

	// None means do not send any notification.
	None Type = "none"
)

// AllNotifierTypes is the list of all valid NotifyTypes.
var AllNotifierTypes []Type = []Type{HTMLEmail, MarkdownIssueTracker, None}
