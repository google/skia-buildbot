package notifier

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/util"
)

const (
	EMAIL_FROM_ADDRESS = "noreply@skia.org"
)

// Notifier is an interface used for sending notifications from an AutoRoller.
type Notifier interface {
	// Send the given message to the given thread. This should be safe to
	// run in a goroutine.
	Send(ctx context.Context, thread string, msg *Message) error
}

// Configuration for a Notifier.
type Config struct {
	// Required fields.

	// Configuration for filtering out messages.
	Filter string `json:"filter"`

	// Exactly one of these should be specified.
	Email  *EmailNotifierConfig  `json:"email,omitempty"`
	Chat   *ChatNotifierConfig   `json:"chat,omitempty"`
	PubSub *PubSubNotifierConfig `json:"pubsub,omitempty"`

	// Optional fields.

	// If present, all messages inherit this subject line.
	Subject string `json:"subject,omitempty"`
}

// Validate the Config.
func (c *Config) Validate() error {
	if c.Filter == "" {
		return errors.New("Filter is required.")
	}
	if _, err := ParseFilter(c.Filter); err != nil {
		return err
	}
	n := []util.Validator{}
	if c.Email != nil {
		n = append(n, c.Email)
	}
	if c.Chat != nil {
		n = append(n, c.Chat)
	}
	if c.PubSub != nil {
		n = append(n, c.PubSub)
	}
	if len(n) != 1 {
		return fmt.Errorf("Exactly one notification config must be supplied, but got %d", len(n))
	}
	return n[0].Validate()
}

// Create a Notifier from the Config.
func (c *Config) Create(ctx context.Context, emailer *email.GMail) (Notifier, Filter, string, error) {
	if err := c.Validate(); err != nil {
		return nil, FILTER_SILENT, "", err
	}
	filter, err := ParseFilter(c.Filter)
	if err != nil {
		return nil, FILTER_SILENT, "", err
	}
	var n Notifier
	if c.Email != nil {
		n, err = EmailNotifier(c.Email.Emails, emailer, "")
	} else if c.Chat != nil {
		n, err = ChatNotifier(c.Chat.RoomID)
	} else if c.PubSub != nil {
		n, err = PubSubNotifier(ctx, c.PubSub.Topic)
	} else {
		return nil, FILTER_SILENT, "", fmt.Errorf("No config specified!")
	}
	if err != nil {
		return nil, FILTER_SILENT, "", err
	}
	return n, filter, c.Subject, nil
}

// Configuration for EmailNotifier.
type EmailNotifierConfig struct {
	// List of email addresses to notify. Required.
	Emails []string `json:"emails"`
}

// Validate the EmailNotifierConfig.
func (c *EmailNotifierConfig) Validate() error {
	if c.Emails == nil || len(c.Emails) == 0 {
		return fmt.Errorf("Emails is required.")
	}
	return nil
}

// emailNotifier is a Notifier implementation which sends email to interested
// parties.
type emailNotifier struct {
	from   string
	gmail  *email.GMail
	markup string
	to     []string
}

// See documentation for Notifier interface.
func (n *emailNotifier) Send(_ context.Context, subject string, msg *Message) error {
	if n.gmail == nil {
		return nil
	}
	return n.gmail.SendWithMarkup(n.from, n.to, subject, msg.Body, n.markup)
}

// EmailNotifier returns a Notifier which sends email to interested parties.
// Sends the same ViewAction markup with each message.
func EmailNotifier(emails []string, emailer *email.GMail, markup string) (Notifier, error) {
	return &emailNotifier{
		from:   EMAIL_FROM_ADDRESS,
		gmail:  emailer,
		markup: markup,
		to:     emails,
	}, nil
}

// Configuration for ChatNotifier.
type ChatNotifierConfig struct {
	RoomID string `json:"room"`
}

// Validate the ChatNotifierConfig.
func (c *ChatNotifierConfig) Validate() error {
	if c.RoomID == "" {
		return fmt.Errorf("RoomID is required.")
	}
	return nil
}

// chatNotifier is a Notifier implementation which sends chat messages.
type chatNotifier struct {
	roomId string
}

// See documentation for Notifier interface.
func (n *chatNotifier) Send(_ context.Context, thread string, msg *Message) error {
	return chatbot.Send(msg.Body, n.roomId, thread)
}

// ChatNotifier returns a Notifier which sends email to interested parties.
func ChatNotifier(roomId string) (Notifier, error) {
	return &chatNotifier{
		roomId: roomId,
	}, nil
}

// Configuration for a PubSubNotifier.
type PubSubNotifierConfig struct {
	Topic string `json:"topic"`
}

// Validate the PubSubNotifierConfig.
func (c *PubSubNotifierConfig) Validate() error {
	if c.Topic == "" {
		return errors.New("Topic is required.")
	}
	return nil
}

// pubSubNotifier is a Notifier implementation which sends pub/sub messages.
type pubSubNotifier struct {
	topic *pubsub.Topic
}

// See documentation for Notifier interface.
func (n *pubSubNotifier) Send(ctx context.Context, subject string, msg *Message) error {
	res := n.topic.Publish(ctx, &pubsub.Message{
		Attributes: map[string]string{
			"severity": msg.Severity.String(),
			"subject":  subject,
		},
		Data: []byte(msg.Body),
	})
	_, err := res.Get(ctx)
	return err
}

// PubSubNotifier returns a Notifier which sends messages via PubSub.
func PubSubNotifier(ctx context.Context, topic string) (Notifier, error) {
	client, err := pubsub.NewClient(ctx, common.PROJECT_ID)
	if err != nil {
		return nil, err
	}

	// Create the topic if it doesn't exist.
	t := client.Topic(topic)
	if exists, err := t.Exists(ctx); err != nil {
		return nil, err
	} else if !exists {
		t, err = client.CreateTopic(ctx, topic)
		if err != nil {
			return nil, err
		}
	}
	return &pubSubNotifier{
		topic: t,
	}, nil
}
