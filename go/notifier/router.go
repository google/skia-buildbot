package notifier

import (
	"fmt"

	"go.skia.org/infra/go/email"
	"golang.org/x/sync/errgroup"
)

// Message represents a message to be sent through one or more Notifiers.
type Message struct {
	// Subject line of the message (required). This is ignored in favor of
	// the default in single-thread mode.
	Subject string
	// Body of the message (required).
	Body string
	// Severity of the message. May cause the message not to be sent,
	// depending on filter settings.
	Severity Severity
}

// Validate the Message.
func (m *Message) Validate() error {
	if m.Subject == "" {
		return fmt.Errorf("Message.Subject is required.")
	}
	if m.Body == "" {
		return fmt.Errorf("Message.Body is required.")
	}
	return nil
}

// filteredThreadedNotifier groups a Notifier with a Filter and an optional
// static subject line for all messages to this Notifier.
type filteredThreadedNotifier struct {
	notifier            Notifier
	filter              Filter
	singleThreadSubject string
}

// Router is a struct used for sending notification through zero or more
// Notifiers.
type Router struct {
	emailer   *email.GMail
	notifiers []*filteredThreadedNotifier
}

// Send a notification.
func (r *Router) Send(msg *Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	var group errgroup.Group
	for _, n := range r.notifiers {
		n := n
		group.Go(func() error {
			if n.filter.ShouldSend(msg.Severity) {
				subject := msg.Subject
				if n.singleThreadSubject != "" {
					subject = n.singleThreadSubject
				}
				return n.notifier.Send(subject, msg)
			}
			return nil
		})
	}
	return group.Wait()
}

// Return a Router instance.
func NewRouter(emailer *email.GMail) *Router {
	return &Router{
		emailer:   emailer,
		notifiers: []*filteredThreadedNotifier{},
	}
}

// Add a new Notifier, which filters according to the given Filter. If
// singleThreadSubject is provided, that will be used as the subject for all
// Messages, ignoring their Subject field.
func (r *Router) Add(n Notifier, f Filter, singleThreadSubject string) {
	r.notifiers = append(r.notifiers, &filteredThreadedNotifier{
		notifier:            n,
		filter:              f,
		singleThreadSubject: singleThreadSubject,
	})
}

// Add a new Notifier based on the given Config.
func (r *Router) AddFromConfig(c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	n, f, s, err := c.Create(r.emailer)
	if err != nil {
		return err
	}
	r.Add(n, f, s)
	return nil
}

// Add all of the Notifiers specified by the given Configs.
func (r *Router) AddFromConfigs(cfgs []*Config) error {
	for _, c := range cfgs {
		if err := r.AddFromConfig(c); err != nil {
			return err
		}
	}
	return nil
}
