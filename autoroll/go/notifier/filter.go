package notifier

const (
	// Message filters.

	// Don't send any messages.
	FILTER_SILENT = iota

	// Only send warning messages.
	FILTER_WARNINGS_ONLY

	// Send messages for mode changes and warnings.
	FILTER_WARNINGS_AND_MODE_CHANGES

	// Send messages for everything.
	FILTER_EVERYTHING

	// Types of messages
	MESSAGE_TYPE_WARNING = iota
	MESSAGE_TYPE_MODE_CHANGE
	MESSAGE_TYPE_ROLL_UPDATE
)

type Filter int
type MessageType int

func (f Filter) ShouldSend(t MessageType) bool {
	return int(t) < int(f)
}
