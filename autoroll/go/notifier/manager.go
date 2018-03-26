package notifier

import (
	"golang.org/x/sync/errgroup"
)

// Message represents a message to be sent through one or more Notifiers.
type Message struct {
	// Required. Subject line of the message. This is ignored in favor of
	// the default in single-thread mode.
	Subject string
	// Required. Body of the message.
	Body string
	// If the Notifier is set to single-thread mode, override that for this
	// Message and use the provided Subject line instead.
	OverrideSingleThread bool
	// Severity of the message. May cause the message not to be sent,
	// depending on filter settings.
	Severity Severity
}

// filteredThreadedNotifier groups a Notifier with a Filter and an optional
// static subject line for all messages to this Notifier.
type filteredThreadedNotifier struct {
	notifier            Notifier
	filter              Filter
	singleThreadSubject string
}

// Manager is a struct used for sending notification through zero or more
// Notifiers.
type Manager struct {
	notifiers []*filteredThreadedNotifier
}

// Send a notification.
func (m *Manager) Send(msg *Message) error {
	var group errgroup.Group
	for _, n := range m.notifiers {
		n := n
		group.Go(func() error {
			if n.filter.ShouldSend(msg.Severity) {
				subject := msg.Subject
				if n.singleThreadSubject != "" && !msg.OverrideSingleThread {
					subject = n.singleThreadSubject
				}
				return n.notifier.Send(subject, msg)
			}
			return nil
		})
	}
	return group.Wait()
}

// Return a Manager instance.
func NewManager() (*Manager, error) {
	return &Manager{
		notifiers: []*filteredThreadedNotifier{},
	}, nil
}

// Add a new Notifier, which filters according to the given Filter. If
// singleThreadSubject is provided, that will be used as the subject for all
// Messages, ignoring their Subject field, unless OverrideSingleThread is true.
func (m *Manager) Add(n Notifier, f Filter, singleThreadSubject string) {
	m.notifiers = append(m.notifiers, &filteredThreadedNotifier{
		notifier:            n,
		filter:              f,
		singleThreadSubject: singleThreadSubject,
	})
}
