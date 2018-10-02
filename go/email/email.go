package email

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"strings"
	ttemplate "text/template"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
	viewActionMarkupTemplateParsed *template.Template  = nil
	emailTemplateParsed            *ttemplate.Template = nil
)

func init() {
	viewActionMarkupTemplateParsed = template.Must(template.New("view_action").Parse(viewActionMarkupTemplate))
	emailTemplateParsed = ttemplate.Must(ttemplate.New("email").Parse(emailTemplate))
}

// GMail is an object used for authenticating to the GMail API server.
type GMail struct {
	service *gmail.Service
}

// Message represents a single email message.
type Message struct {
	SenderDisplayName string
	To                []string
	Subject           string
	Body              string
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
	ts, err := auth.NewTokenSourceFromIdAndSecret(clientId, clientSecret, tokenCacheFile, gmail.GmailComposeScope)
	if err != nil {
		return nil, err
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	service, err := gmail.New(client)
	if err != nil {
		return nil, err
	}
	return &GMail{
		service: service,
	}, nil
}

// ClientSecrets is the structure of a client_secrets.json file that contains info on an installed client.
type ClientSecrets struct {
	Installed ClientConfig `json:"installed"`
}

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// NewFromFiles creates a new GMail object authorized from the given files.
//
// Creates a copy of the token cache file in /tmp since mounted secrets are read-only.
func NewFromFiles(emailTokenCacheFile, emailClientSecretsFile string) (*GMail, error) {
	var clientSecrets ClientSecrets
	err := util.WithReadFile(emailClientSecretsFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&clientSecrets)
	})
	if err != nil {
		sklog.Fatalf("Failed to read client secrets from %q: %s", emailClientSecretsFile, err)
	}
	// Create a copy of the token cache file since mounted secrets are read-only.
	fout, err := ioutil.TempFile("", "")
	if err != nil {
		sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
	}
	err = util.WithReadFile(emailTokenCacheFile, func(fin io.Reader) error {
		_, err := io.Copy(fout, fin)
		if err != nil {
			err = fout.Close()
		}
		return err
	})
	if err != nil {
		sklog.Fatalf("Failed to write token cache file from %q to %q: %s", emailTokenCacheFile, fout.Name(), err)
	}
	emailTokenCacheFile = fout.Name()

	return NewGMail(clientSecrets.Installed.ClientID, clientSecrets.Installed.ClientSecret, emailTokenCacheFile)
}

// Send an email.
func (a *GMail) Send(senderDisplayName string, to []string, subject string, body string) error {
	return a.SendWithMarkup(senderDisplayName, to, subject, body, "")
}

// Send an email with gmail markup.
// Documentation about markups supported in gmail are here: https://developers.google.com/gmail/markup/
// A go-to action example is here: https://developers.google.com/gmail/markup/reference/go-to-action
func (a *GMail) SendWithMarkup(senderDisplayName string, to []string, subject string, body string, markup string) error {
	sender := "me"
	// Get email address to use in the from section.
	profile, err := a.service.Users.GetProfile(sender).Do()
	if err != nil {
		return fmt.Errorf("Failed to get profile for %s: %v", sender, err)
	}
	fromWithName := fmt.Sprintf("%s <%s>", senderDisplayName, profile.EmailAddress)

	msgBytes := new(bytes.Buffer)
	if err := emailTemplateParsed.Execute(msgBytes, struct {
		From    template.HTML
		To      string
		Subject string
		Body    template.HTML
		Markup  template.HTML
	}{
		From:    template.HTML(fromWithName),
		To:      strings.Join(to, ","),
		Subject: subject,
		Body:    template.HTML(body),
		Markup:  template.HTML(markup),
	}); err != nil {
		return fmt.Errorf("Failed to send email; could not execute template: %v", err)
	}
	sklog.Infof("Message to send: %q", msgBytes.String())
	msg := gmail.Message{}
	msg.SizeEstimate = int64(msgBytes.Len())
	msg.Snippet = subject
	msg.Raw = base64.URLEncoding.EncodeToString(msgBytes.Bytes())

	_, err = a.service.Users.Messages.Send(sender, &msg).Do()
	return err
}

// SendMessage sends the given Message.
func (a *GMail) SendMessage(msg *Message) error {
	return a.Send(msg.SenderDisplayName, msg.To, msg.Subject, msg.Body)
}
