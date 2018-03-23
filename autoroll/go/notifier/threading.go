package notifier

import "fmt"

const (
	// Types of threading.

	// Send all notifications to the same thread. Minimum noise.
	THREADING_SINGLE_THREAD = iota

	// Use separate threads for mode changes and each new roll attempt.
	THREADING_SEPARATE_THREADS

	// If not specified, use this thread name in single-thread mode.
	DEFAULT_SINGLE_THREAD_NAME = "Update from the AutoRoller"

	// Thread name for all warnings.
	THREAD_NAME_WARNING = "AutoRoll warning"

	// Thread name for all mode changes.
	THREAD_NAME_MODE_CHANGE = "The AutoRoller mode was changed"

	// Thread name for updates on a particular roll.
	THREAD_NAME_ROLL_UPDATE = "The AutoRoller uploaded change %s"
)

// Threader is an interface which derives a thread name from a message.
type Threader interface {
	ThreadName(*Message) string
}

// singleThreader puts ALL messages in the same thread.
type singleThreader struct {
	threadName string
}

// See documentation for Threader interface.
func (t *singleThreader) ThreadName(msg *Message) string {
	return t.threadName
}

// SingleThreader returns a Threader which puts ALL messages in the same thread,
// optionally using the supplied thread name.
func SingleThreader(threadName string) Threader {
	if threadName == "" {
		threadName = DEFAULT_SINGLE_THREAD_NAME
	}
	return &singleThreader{
		threadName: threadName,
	}
}

// multiThreader uses a separate thread for each roll attempt, mode changes,
// and warnings.
type multiThreader struct{}

// See documentation for Threader interface.
func (t *multiThreader) ThreadName(msg *Message) string {
	switch msg.Type {
	case MESSAGE_TYPE_WARNING:
		return THREAD_NAME_WARNING
	case MESSAGE_TYPE_MODE_CHANGE:
		return THREAD_NAME_MODE_CHANGE
	case MESSAGE_TYPE_ROLL_UPDATE:
		return fmt.Sprintf(THREAD_NAME_ROLL_UPDATE, msg.Issue)
	default:
		return DEFAULT_SINGLE_THREAD_NAME
	}
}

// MultiThreader returns a Threader which uses a separate thread for each roll
// attempt, mode changes, and warnings.
func MultiThreader() Threader {
	return &multiThreader{}
}
