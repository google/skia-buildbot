package email

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TODO(borenet): Replace the current implementation with this once we're able
// to update our dependency on go.chromium.org/luci.
// const luciNotifyServiceURL = "luci-notify.appspot.com"

// // Client sends email via LUCI Notify.
// type Client interface {
// 	mailer.MailerClient
// }

// // NewClient returns a Client instance which sends email via LUCI Notify.
// func NewClient(ctx context.Context) (Client, error) {
// 	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
// 	if err != nil {
// 		return nil, skerr.Wrap(err)
// 	}
// 	conn, err := grpc.NewClient(
// 		luciNotifyServiceURL,
// 		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
// 		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: ts}),
// 	)
// 	if err != nil {
// 		return nil, skerr.Wrap(err)
// 	}
// 	return mailer.NewMailerClient(conn), nil
// }

type SendMailRequest struct {
	RequestId  string   `json:"request_id,omitempty"`
	Sender     string   `json:"sender,omitempty"`
	ReplyTo    string   `json:"reply_to,omitempty"`
	To         []string `json:"to,omitempty"`
	Cc         []string `json:"cc,omitempty"`
	Bcc        []string `json:"bcc,omitempty"`
	Subject    string   `json:"subject,omitempty"`
	TextBody   string   `json:"text_body,omitempty"`
	HtmlBody   string   `json:"html_body,omitempty"`
	InReplyTo  string   `json:"in_reply_to,omitempty"`
	References []string `json:"references,omitempty"`
}

type SendMailResponse struct {
	MessageId string `json:"message_id,omitempty"`
}

type Client interface {
	SendMail(ctx context.Context, in *SendMailRequest) (*SendMailResponse, error)
}

type clientImpl struct {
	c *http.Client
}

func NewClient(ctx context.Context) (*clientImpl, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &clientImpl{
		c: &http.Client{
			Transport: &oauth2.Transport{
				Source: ts,
				Base:   http.DefaultTransport,
			},
			Timeout: time.Minute,
		},
	}, nil
}

func (c *clientImpl) SendMail(ctx context.Context, in *SendMailRequest) (*SendMailResponse, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := c.c.Post("https://notify.api.luci.app/prpc/luci.mailer.v1.Mailer/SendMail", "application/json; charset=utf-8", bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed reading response body")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, skerr.Fmt("Got unexpected status %s: %s", resp.Status, string(body))
	}
	var out SendMailResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, skerr.Wrapf(err, "failed decoding response body")
		}
	}
	return &out, nil
}
