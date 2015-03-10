package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"

	"code.google.com/p/goauth2/oauth"
	gmail "code.google.com/p/google-api-go-client/gmail/v1"
	"go.skia.org/infra/go/auth"
)

var (
	emailTemplate string = `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}
Content-Type: text/html

<html>
{{.Body}}
</html>
`
	emailTemplateParsed *template.Template = nil
)

func init() {
	emailTemplateParsed = template.Must(template.New("email").Parse(emailTemplate))
}

// GMail is an object used for authenticating to the GMail API server.
type GMail struct {
	service *gmail.Service
}

// NewGMail returns a new GMail object which is authorized to send email.
func NewGMail(clientId, clientSecret, tokenCacheFile string) (*GMail, error) {
	config := oauth.Config{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Scope:        gmail.GmailComposeScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		TokenCache:   oauth.CacheFile(tokenCacheFile),
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		AccessType:   "offline",
	}
	client, err := auth.RunFlow(&config)
	if err != nil {
		return nil, err
	}
	service, err := gmail.New(client)
	if err != nil {
		return nil, err
	}
	return &GMail{
		service: service,
	}, nil
}

// Send an email.
func (a *GMail) Send(to []string, subject string, body string) error {
	user := "me"
	msgBytes := new(bytes.Buffer)
	if err := emailTemplateParsed.Execute(msgBytes, struct {
		From    string
		To      string
		Subject string
		Body    template.HTML
	}{
		From:    user,
		To:      strings.Join(to, ","),
		Subject: subject,
		Body:    template.HTML(body),
	}); err != nil {
		return fmt.Errorf("Failed to send email; could not execute template: %v", err)
	}
	msg := gmail.Message{}
	msg.SizeEstimate = int64(msgBytes.Len())
	msg.Snippet = subject
	msg.Raw = base64.URLEncoding.EncodeToString(msgBytes.Bytes())

	_, err := a.service.Users.Messages.Send(user, &msg).Do()
	return err
}
