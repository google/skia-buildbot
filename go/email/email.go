package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"

	"go.skia.org/infra/go/auth"
	gmail "google.golang.org/api/gmail/v1"
)

var (
	viewActionMarkupTemplate string = `
<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="{{.Link}}"/>
    <meta itemprop="name" content="{{.Name}}"/>
  </div>
  <meta itemprop="description" content="{{.Description}}"/>
</div>
`
	emailTemplate string = `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}
Content-Type: text/html; charset=UTF-8

<html>
<body>
{{.Markup}}
{{.Body}}
</body>
</html>
`
	viewActionMarkupTemplateParsed *template.Template = nil
	emailTemplateParsed            *template.Template = nil
)

func init() {
	viewActionMarkupTemplateParsed = template.Must(template.New("view_action").Parse(viewActionMarkupTemplate))
	emailTemplateParsed = template.Must(template.New("email").Parse(emailTemplate))
}

// GMail is an object used for authenticating to the GMail API server.
type GMail struct {
	service *gmail.Service
}

// Message represents a single email message.
type Message struct {
	To      []string
	Subject string
	Body    string
}

// GetViewActionMarkup returns a string that contains the required markup.
func GetViewActionMarkup(link, name, description string) (string, error) {
	markupBytes := new(bytes.Buffer)
	if err := viewActionMarkupTemplateParsed.Execute(markupBytes, struct {
		Link        string
		Name        string
		Description string
	}{
		Link:        link,
		Name:        name,
		Description: description,
	}); err != nil {
		return "", fmt.Errorf("Could not execute template: %v", err)
	}
	return markupBytes.String(), nil
}

// NewGMail returns a new GMail object which is authorized to send email.
func NewGMail(clientId, clientSecret, tokenCacheFile string) (*GMail, error) {
	client, err := auth.NewClientFromIdAndSecret(clientId, clientSecret, tokenCacheFile, gmail.GmailComposeScope)
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
	return a.SendWithMarkup(to, subject, body, "")
}

// Send an email with gmail markup.
// Documentation about markups supported in gmail are here: https://developers.google.com/gmail/markup/
// A go-to action example is here: https://developers.google.com/gmail/markup/reference/go-to-action
func (a *GMail) SendWithMarkup(to []string, subject string, body string, markup string) error {
	user := "me"
	msgBytes := new(bytes.Buffer)
	if err := emailTemplateParsed.Execute(msgBytes, struct {
		From    string
		To      string
		Subject string
		Body    template.HTML
		Markup  template.HTML
	}{
		From:    user,
		To:      strings.Join(to, ","),
		Subject: subject,
		Body:    template.HTML(body),
		Markup:  template.HTML(markup),
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

// SendMessage sends the given Message.
func (a *GMail) SendMessage(msg *Message) error {
	return a.Send(msg.To, msg.Subject, msg.Body)
}
