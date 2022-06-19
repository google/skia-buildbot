package email

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	ttemplate "text/template"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	gmail "google.golang.org/api/gmail/v1"
)

const (
	viewActionMarkupTemplate = `
<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="{{.Link}}"/>
    <meta itemprop="name" content="{{.Name}}"/>
  </div>
  <meta itemprop="description" content="{{.Description}}"/>
</div>
`
	emailTemplate = `From: {{.From}}
To: {{.To}}
Subject: {{.Subject}}
Content-Type: text/html; charset=UTF-8
References: {{.ThreadingReference}}
In-Reply-To: {{.ThreadingReference}}

<html>
<body>
{{.Markup}}
{{.Body}}
</body>
</html>
`
)

var (
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

	// From is the email address of the authenticated account.
	from string
}

// Message represents a single email message.
type Message struct {
	SenderDisplayName  string
	To                 []string
	Subject            string
	Body               string
	ThreadingReference string
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
		return "", skerr.Wrapf(err, "Could not execute template %s", name)
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
	ret := &GMail{
		service: service,
		from:    "me",
	}
	if err := ret.populateFromAddress(); err != nil {
		sklog.Errorf("Failed to determine sending accounts email address: %s", err)
	}
	return ret, nil
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

// populateFromAddress fills in a.from with the email address for the
// authenticated account.
func (a *GMail) populateFromAddress() error {
	profile, err := a.service.Users.GetProfile("me").Do()
	if err != nil {
		return skerr.Wrapf(err, "Failed to get profile.")
	}
	a.from = profile.EmailAddress
	return nil
}

// Send an email. Returns the messageId of the sent email.
func (a *GMail) Send(senderDisplayName string, to []string, subject, body, threadingReference string) (string, error) {
	return a.SendWithMarkup(senderDisplayName, to, subject, body, "", threadingReference)
}

// FormatAsRFC2822 returns a *bytes.Buffer that contains the email message
// formatted in RFC 2822 format.
func FormatAsRFC2822(fromDisplayName string, from string, to []string, subject, body, markup, threadingReference string) (*bytes.Buffer, error) {
	fromWithName := fmt.Sprintf("%s <%s>", fromDisplayName, from)
	var msgBytes bytes.Buffer
	if err := emailTemplateParsed.Execute(&msgBytes, struct {
		From               template.HTML
		To                 string
		Subject            string
		ThreadingReference string
		Body               template.HTML
		Markup             template.HTML
	}{
		From:               template.HTML(fromWithName),
		To:                 strings.Join(to, ","),
		Subject:            subject,
		ThreadingReference: threadingReference,
		Body:               template.HTML(body),
		Markup:             template.HTML(markup),
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed to format email.")
	}
	return &msgBytes, nil
}

// SendWithMarkup sends an email with gmail markup. Returns the messageId of the sent email.
// Documentation about markups supported in gmail are here: https://developers.google.com/gmail/markup/
// A go-to action example is here: https://developers.google.com/gmail/markup/reference/go-to-action
func (a *GMail) SendWithMarkup(fromDisplayName string, to []string, subject, body, markup, threadingReference string) (string, error) {
	msgBytes, err := FormatAsRFC2822(fromDisplayName, a.from, to, subject, body, markup, threadingReference)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	sklog.Infof("Message to send: %q", msgBytes.String())
	return a.SendRFC2822Message(subject, msgBytes.Bytes())
}

// SendRFC2822Message sends the RFC2822 formatted email message in body with the
// given subject.
func (a *GMail) SendRFC2822Message(subject string, body []byte) (string, error) {
	msg := gmail.Message{}
	msg.SizeEstimate = int64(len(body))
	msg.Snippet = subject
	msg.Raw = base64.URLEncoding.EncodeToString(body)

	m, err := a.service.Users.Messages.Send(a.from, &msg).Do()
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to send email: %s", subject)
	}
	return m.Id, nil
}

var fromRegex = regexp.MustCompile(`(?m)^From: (.*)$`)
var toRegex = regexp.MustCompile(`(?m)^To: (.*)$`)
var subjectRegex = regexp.MustCompile(`(?m)^Subject:(.*)$`)
var doubleNewLine = regexp.MustCompile(`\n\n`)

const defaultSubject = "(no subject)"

// ParseRFC2822Message returns the email address in the From:, To: and Subject:
// lines, and also returns the body of the message, which is presumed to be an
// HTML formatted email.
func ParseRFC2822Message(body []byte) (string, []string, string, string, error) {
	// From: senderDisplayName <sender email>
	// Subject: subject
	// To: A Display Name <a@example.com>, B <b@example.org>
	match := fromRegex.FindSubmatch(body)
	if match == nil || len(match) < 2 {
		return "", nil, "", "", skerr.Fmt("Failed to find a From: line in message.")
	}
	from := string(match[1])

	match = subjectRegex.FindSubmatch(body)
	subject := defaultSubject
	if len(match) >= 2 {
		subject = string(bytes.TrimSpace(match[1]))
	}

	match = toRegex.FindSubmatch(body)
	if match == nil || len(match) < 2 {
		return "", nil, "", "", skerr.Fmt("Failed to find a To: line in message.")
	}
	to := []string{}
	for _, addr := range bytes.Split(match[1], []byte(",")) {
		toAsString := string(bytes.TrimSpace(addr))
		if toAsString != "" {
			to = append(to, toAsString)
		}
	}
	if len(to) < 1 {
		return "", nil, "", "", skerr.Fmt("Failed to find any To: addresses.")
	}

	parts := doubleNewLine.Split(string(body), 2)
	if len(parts) != 2 {
		return "", nil, "", "", skerr.Fmt("Failed to find the body of the message.")
	}
	messageBody := parts[1]

	return from, to, subject, messageBody, nil
}

// GetThreadingReference returns the reference string that can be used to thread emails.
func (a *GMail) GetThreadingReference(messageID string) (string, error) {
	// Get the reference from the response headers of messages.get call.
	m, err := a.service.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get message data for id %s", messageID)
	}
	reference := ""
	for _, h := range m.Payload.Headers {
		if h.Name == "Message-Id" {
			reference = h.Value
			break
		}
	}
	if reference == "" {
		return "", skerr.Wrapf(err, "Could not find \"Message-Id\" header for Message-Id %s", messageID)
	}
	return reference, nil
}

// SendMessage sends the given Message. Returns the messageId of the sent email.
func (a *GMail) SendMessage(msg *Message) (string, error) {
	return a.Send(msg.SenderDisplayName, msg.To, msg.Subject, msg.Body, msg.ThreadingReference)
}
