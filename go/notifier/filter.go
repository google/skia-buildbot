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

func (f Filter) String() string {
	switch f {
	case FILTER_SILENT:
		return "silent"
	case FILTER_ERROR:
		return "error"
	case FILTER_WARNING:
		return "warning"
	case FILTER_INFO:
		return "info"
	case FILTER_DEBUG:
		return "debug"
	default:
		return "UNKNOWN!"
	}
}

func (f Filter) ShouldSend(t Severity) bool {
	return int(t) < int(f)
}

type Severity int

func (s Severity) String() string {
	switch s {
	case SEVERITY_ERROR:
		return "error"
	case SEVERITY_WARNING:
		return "warning"
	case SEVERITY_INFO:
		return "info"
	case SEVERITY_DEBUG:
		return "debug"
	default:
		return "UNKNOWN!"
	}
}

func ParseFilter(f string) (Filter, error) {
	switch f {
	case FILTER_SILENT.String():
		return FILTER_SILENT, nil
	case FILTER_ERROR.String():
		return FILTER_ERROR, nil
	case FILTER_WARNING.String():
		return FILTER_WARNING, nil
	case FILTER_INFO.String():
		return FILTER_INFO, nil
	case FILTER_DEBUG.String(), "":
		return FILTER_DEBUG, nil
	default:
		return FILTER_SILENT, fmt.Errorf("Unknown filter %q", f)
	}
}
