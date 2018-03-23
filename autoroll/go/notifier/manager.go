package notifier

import (
	"golang.org/x/sync/errgroup"
)

// filteredThreadedNotifier groups a Notifier with a Filter and a Threader.
type filteredThreadedNotifier struct {
	notifier Notifier
	filter   Filter
	threader Threader
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
		group.Go(func() error {
			if n.filter.ShouldSend(msg.Type) {
				threadName := msg.OverrideThreadName
				if threadName == "" {
					threadName = n.threader.ThreadName(msg)
				}
				return n.notifier.Send(threadName, msg)
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

// Add a new Notifier.
func (m *Manager) Add(n Notifier, f Filter, t Threader) {
	m.notifiers = append(m.notifiers, &filteredThreadedNotifier{
		notifier: n,
		filter:   f,
		threader: t,
	})
}
