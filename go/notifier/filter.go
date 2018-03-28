package notifier

const (
	// Message filters.
	FILTER_SILENT Filter = iota
	FILTER_ERROR
	FILTER_WARNING
	FILTER_INFO
	FILTER_DEBUG
)

const (
	// Severity of messages
	SEVERITY_ERROR Severity = iota
	SEVERITY_WARNING
	SEVERITY_INFO
	SEVERITY_DEBUG
)

type Filter int
type Severity int

func (f Filter) ShouldSend(t Severity) bool {
	return int(t) < int(f)
}
