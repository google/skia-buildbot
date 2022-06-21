// Package emailclient is a client for talking to emailservice.
package emailclient

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Client for sending emails to the emailservice.
type Client struct {
	emailServiceURL string
	client          *http.Client
}

// New returns a new Client.
func New() Client {
	return Client{
		emailServiceURL: "http://emailservice:8000/send",
		client:          httputils.DefaultClientConfig().With2xxOnly().Client(),
	}
}

// SendWithMarkup sends an email with gmail markup. Returns the messageId of the
// sent email. Documentation about markups supported in gmail are here:
// https://developers.google.com/gmail/markup/ A go-to action example is here:
// https://developers.google.com/gmail/markup/reference/go-to-action
//
// It is almost a drop-in replacement for email.Gmail.SendWithMarkup with the
// following changes:
//
// - The 'from' email address must be supplied.
func (c *Client) SendWithMarkup(fromDisplayName string, from string, to []string, subject, body, markup, threadingReference string) (string, error) {
	// Generate the Message-ID.
	id, err := uuid.NewRandom()
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to generate uuid")
	}
	messageID := fmt.Sprintf("<%x@skia.org>", id)
	msgBytes, err := email.FormatAsRFC2822(fromDisplayName, from, to, subject, body, markup, threadingReference, messageID)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to format.")
	}
	sklog.Infof("Message to send: %q", msgBytes.String())
	_, err = c.client.Post(c.emailServiceURL, "message/rfc822", msgBytes)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to send.")
	}
	return messageID, nil
}
