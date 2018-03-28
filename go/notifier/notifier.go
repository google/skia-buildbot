package notifier

import (
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/email"
)

const (
	EMAIL_FROM_ADDRESS = "noreply@skia.org"
)

// Notifier is an interface used for sending notifications from an AutoRoller.
type Notifier interface {
	// Send the given message to the given thread. This should be safe to
	// run in a goroutine.
	Send(thread string, msg *Message) error
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
func (n *emailNotifier) Send(subject string, msg *Message) error {
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

// chatNotifier is a Notifier implementation which sends chat messages.
type chatNotifier struct {
	roomId string
}

// See documentation for Notifier interface.
func (n *chatNotifier) Send(thread string, msg *Message) error {
	return chatbot.Send(msg.Body, n.roomId, thread)
}

// ChatNotifier returns a Notifier which sends email to interested parties.
func ChatNotifier(roomId string) (Notifier, error) {
	return &chatNotifier{
		roomId: roomId,
	}, nil
}
