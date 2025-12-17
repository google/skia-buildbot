package email

import (
	"context"
	"crypto/tls"

	"go.chromium.org/luci/mailer/api/mailer"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

const luciNotifyServiceURL = "luci-notify.appspot.com"

// Client sends email via LUCI Notify.
type Client interface {
	mailer.MailerClient
}

// NewClient returns a Client instance which sends email via LUCI Notify.
func NewClient(ctx context.Context) (Client, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	conn, err := grpc.NewClient(
		luciNotifyServiceURL,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: ts}),
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return mailer.NewMailerClient(conn), nil
}

// SendWithMarkup is a convenience function for call sites previously using
// emailclient.SendWithMarkup.
func SendWithMarkup(ctx context.Context, c Client, to []string, subject, body, markup, threadingReference string) (string, error) {
	req := &mailer.SendMailRequest{
		To:       to,
		Subject:  subject,
		HtmlBody: markup + "\n" + body,
	}
	if threadingReference != "" {
		req.InReplyTo = threadingReference
		req.References = []string{threadingReference}
	}
	resp, err := c.SendMail(ctx, req)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return resp.MessageId, nil
}
