package notifier

import (
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/email"
)

const (
	EMAIL_FROM_ADDRESS = "autoroller@skia.org"
)

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
	if n.gmail == nil {
		return nil
	}
	/*markup, err := email.GetViewActionMarkup(r.serverURL, "Go to AutoRoller", "Direct link to the AutoRoll server.")
	if err != nil {
		return err
	}*/
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
func ChatNotifier(room string) (Notifier, error) {
	return &chatNotifier{
		room: room,
	}, nil
}
