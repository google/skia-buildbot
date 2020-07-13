package notifier

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/util"
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
	// Type of message. This is used with the optional IncludeMsgTypes to
	// send only specific types of messages.
	Type string
}

// Validate the Message.
func (m *Message) Validate() error {
	if m.Subject == "" {
		return fmt.Errorf("Message.Subject is required.")
	}
	if m.Body == "" {
		return fmt.Errorf("Message.Body is required.")
	}
	if m.Type == "" {
		return fmt.Errorf("Message.Type is required.")
	}
	return nil
}

// filteredThreadedNotifier groups a Notifier with a Filter and an optional
// static subject line for all messages to this Notifier.
type filteredThreadedNotifier struct {
	includeMsgTypes     []string
	notifier            Notifier
	filter              Filter
	singleThreadSubject string
}

// Router is a struct used for sending notification through zero or more
// Notifiers.
type Router struct {
	client       *http.Client
	configReader chatbot.ConfigReader
	emailer      *email.GMail
	notifiers    []*filteredThreadedNotifier
}

// Send a notification.
func (r *Router) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	var group errgroup.Group
	for _, n := range r.notifiers {
		n := n
		group.Go(func() error {
			if n.includeMsgTypes != nil {
				if !util.In(msg.Type, n.includeMsgTypes) {
					return nil
				}
			} else if !n.filter.ShouldSend(msg.Severity) {
				return nil
			}
			subject := msg.Subject
			if n.singleThreadSubject != "" {
				subject = n.singleThreadSubject
			}
			return n.notifier.Send(ctx, subject, msg)
		})
	}
	return group.Wait()
}

// Return a Router instance.
func NewRouter(client *http.Client, emailer *email.GMail, chatBotConfigReader chatbot.ConfigReader) *Router {
	return &Router{
		client:       client,
		configReader: chatBotConfigReader,
		emailer:      emailer,
		notifiers:    []*filteredThreadedNotifier{},
	}
}

// Add a new Notifier, which filters according to the given Filter. If
// singleThreadSubject is provided, that will be used as the subject for all
// Messages, ignoring their Subject field.
func (r *Router) Add(n Notifier, f Filter, includeMsgTypes []string, singleThreadSubject string) {
	r.notifiers = append(r.notifiers, &filteredThreadedNotifier{
		includeMsgTypes:     includeMsgTypes,
		notifier:            n,
		filter:              f,
		singleThreadSubject: singleThreadSubject,
	})
}

// Add a new Notifier based on the given Config.
func (r *Router) AddFromConfig(ctx context.Context, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	n, f, wl, s, err := c.Create(ctx, r.client, r.emailer, r.configReader)
	if err != nil {
		return err
	}
	r.Add(n, f, wl, s)
	return nil
}

// Add all of the Notifiers specified by the given Configs.
func (r *Router) AddFromConfigs(ctx context.Context, cfgs []*Config) error {
	for _, c := range cfgs {
		if err := r.AddFromConfig(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
