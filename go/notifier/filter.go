package notifier

import "fmt"

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

func ParseFilter(f string) (Filter, error) {
	switch f {
	case "silent":
		return FILTER_SILENT, nil
	case "error":
		return FILTER_ERROR, nil
	case "warning":
		return FILTER_WARNING, nil
	case "info":
		return FILTER_INFO, nil
	case "debug", "":
		return FILTER_DEBUG, nil
	default:
		return FILTER_SILENT, fmt.Errorf("Unknown filter %q", f)
	}
}
