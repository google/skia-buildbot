package notifier

import (
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/email"
)

const (
	EMAIL_FROM_ADDRESS = "autoroller@skia.org"
)

// Message represents a message to be sent through one or more Notifiers.
type Message struct {
	// Required. Body of the message.
	Body string
	// Optional issue ID.
	Issue string
	// Required. Type of message.
	Type MessageType

	// Override the default threading behavior for this message.
	OverrideThreadName string
}

// Notifier is an interface used for sending notifications from an AutoRoller.
type Notifier interface {
	// Send the given message.
	Send(string, *Message) error
}

// emailNotifier is a Notifier implementation which sends email to interested
// parties.
type emailNotifier struct {
	from  string
	gmail *email.GMail
	to    []string
}

// See documentation for Notifier interface.
func (n *emailNotifier) Send(subject string, msg *Message) error {
	return n.gmail.SendWithMarkup(n.from, n.to, subject, msg.Body, "")
}

// EmailNotifier returns a Notifier which sends email to interested parties.
func EmailNotifier(emails []string, emailer *email.GMail) (Notifier, error) {
	return &emailNotifier{
		from:  EMAIL_FROM_ADDRESS,
		gmail: emailer,
		to:    emails,
	}, nil
}

// chatNotifier is a Notifier implementation which sends chat messages.
type chatNotifier struct {
	room string
}

// See documentation for Notifier interface.
func (n *chatNotifier) Send(thread string, msg *Message) error {
	// TODO(borenet): How to thread?
	return chatbot.Send(msg.Body, n.room)
}

// ChatNotifier returns a Notifier which sends email to interested parties.
func ChatNotifer(room string) (Notifier, error) {
	return &chatNotifier{
		room: room,
	}, nil
}
