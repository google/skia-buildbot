package notify

import (
	"context"
	"fmt"
	"regexp"

	"go.skia.org/infra/go/email"
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

// EmailTransport implements Transport using email.Client.
type EmailTransport struct {
	client email.Client
}

// NewEmailTransport returns a new EmailService instance.
func NewEmailTransport() (EmailTransport, error) {
	client, err := email.NewClient(context.TODO())
	if err != nil {
		return EmailTransport{}, skerr.Wrapf(err, "creating email client")
	}
	return EmailTransport{
		client: client,
	}, nil
}

// SendNewRegression implements Transport.
func (e EmailTransport) SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (string, error) {
	if alert.Alert == "" {
		return "", fmt.Errorf("No notification sent. No email address set for alert #%s", alert.IDAsString)
	}

	threadingReference, err := email.SendWithMarkup(ctx, e.client, splitEmails(alert.Alert), subject, "", body, "")
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

	_, err := email.SendWithMarkup(ctx, e.client, splitEmails(alert.Alert), subject, "", body, threadingReference)
	if err != nil {
		return skerr.Wrapf(err, "sending notification by email")
	}
	return nil
}

// UpdateRegressionNotification implements Transport.
func (e EmailTransport) UpdateRegressionNotification(ctx context.Context, alert *alerts.Alert, body, notificationId string) error {
	return nil
}
