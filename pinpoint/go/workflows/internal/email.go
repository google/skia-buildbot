package internal

import (
	"context"

	"go.temporal.io/sdk/activity"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/skerr"
)

// SendEmailActivity sends an email via LUCI Notify.
func SendEmailActivity(ctx context.Context, to []string, subject, body string) error {
	logger := activity.GetLogger(ctx)
	client, err := email.NewClient(ctx)
	if err != nil {
		logger.Error("Unable to create email client:", err)
		return skerr.Wrap(err)
	}
	_, err = email.SendWithMarkup(ctx, client, to, subject, body, "", "")
	if err != nil {
		logger.Error("Send email failed:", err)
		return skerr.Wrap(err)
	}
	return skerr.Wrap(err)
}
