package notify

import (
	"context"
	"fmt"
	"regexp"

	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
)

var (
	emailAddressSplitter = regexp.MustCompile("[, ]+")
)

// splitEmails breaks up a comma separated list of email addresses.
func splitEmails(s string) []string {
	ret := []string{}
	for _, e := range emailAddressSplitter.Split(s, -1) {
		if e != "" {
			ret = append(ret, e)
		}
	}
	return ret
}

// EmailTransport implements Transport using emailclient.
type EmailTransport struct {
	client emailclient.Client
}

// NewEmailTransport returns a new EmailService instance.
func NewEmailTransport() EmailTransport {
	return EmailTransport{
		client: emailclient.New(),
	}
}

// SendNewRegression implements Transport.
func (e EmailTransport) SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (string, error) {
	if alert.Alert == "" {
		return "", fmt.Errorf("No notification sent. No email address set for alert #%s", alert.IDAsString)
	}

	threadingReference, err := e.client.SendWithMarkup("", fromAddress, splitEmails(alert.Alert), subject, "", body, "")
	if err != nil {
		return "", skerr.Wrapf(err, "sending notification by email")
	}
	return threadingReference, nil

}

// SendRegressionMissing implements Transport.
func (e EmailTransport) SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) error {
	if alert.Alert == "" {
		return skerr.Fmt("No notification sent. No email address set for alert #%s", alert.IDAsString)
	}

	_, err := e.client.SendWithMarkup("", fromAddress, splitEmails(alert.Alert), subject, "", body, threadingReference)
	if err != nil {
		return skerr.Wrapf(err, "sending notification by email")
	}
	return nil
}

// UpdateRegressionNotification implements Transport.
func (e EmailTransport) UpdateRegressionNotification(ctx context.Context, alert *alerts.Alert, body, notificationId string) error {
	return nil
}
