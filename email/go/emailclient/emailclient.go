// Package emailclient is a client for talking to emailservice.
package emailclient

import (
	"net/http"
	"net/mail"

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
	to, err := dedupAddresses(to)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to dedup \"to\" addresses: %s", to)
	}
	msgBytes, err := email.FormatAsRFC2822(fromDisplayName, from, to, subject, body, markup, threadingReference, "")
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to format.")
	}
	sklog.Infof("Message to send: %q", msgBytes.String())
	resp, err := c.client.Post(c.emailServiceURL, "message/rfc822", msgBytes)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to send.")
	}
	return resp.Header.Get("X-Message-Id"), nil
}

// dedupAddresses dedupes RFC 5322 addresses. Without this sendgrid could fail
// to send the message with: "Each email address in the personalization block
// should be unique between to, cc, and bcc. We found the first duplicate
// instance of [xyz] in the personalizations".
// Note that deduping might or might not preserve the "Name" portion of a
// parsed address based on the order in which the "to" addresses are processed.
// Eg: ["name@example.org", "Name<name@example.org"] will dedup to ["name@example.org"]
// but ["Name<name@example.org>", "name@example.org"] will dedup to ["Name<name@example.org>"]
func dedupAddresses(to []string) ([]string, error) {
	deduped := []string{}
	seen := map[string]bool{}
	for _, unparsedAddr := range to {
		parsedAddr, err := mail.ParseAddress(unparsedAddr)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse %s", unparsedAddr)
		}
		if _, ok := seen[parsedAddr.Address]; !ok {
			seen[parsedAddr.Address] = true
			deduped = append(deduped, unparsedAddr)
		}
	}
	return deduped, nil
}
