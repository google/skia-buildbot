package email

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	ttemplate "text/template"

	"go.skia.org/infra/go/skerr"
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
{{if .ThreadingReference}}References: {{.ThreadingReference}}
{{end}}{{if .ThreadingReference}}In-Reply-To: {{.ThreadingReference}}
{{end}}{{if .MessageID -}}Message-ID: {{.MessageID}}
{{end}}
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

// FormatAsRFC2822 returns a *bytes.Buffer that contains the email message
// formatted in RFC 2822 format.
func FormatAsRFC2822(fromDisplayName string, from string, to []string, subject, body, markup, threadingReference string, messageID string) (*bytes.Buffer, error) {
	fromWithName := fmt.Sprintf("%s <%s>", fromDisplayName, from)
	var msgBytes bytes.Buffer
	if err := emailTemplateParsed.Execute(&msgBytes, struct {
		From               template.HTML
		To                 string
		Subject            string
		ThreadingReference string
		Body               template.HTML
		Markup             template.HTML
		MessageID          string
	}{
		From:               template.HTML(fromWithName),
		To:                 strings.Join(to, ","),
		Subject:            subject,
		ThreadingReference: threadingReference,
		Body:               template.HTML(body),
		Markup:             template.HTML(markup),
		MessageID:          messageID,
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed to format email.")
	}
	return &msgBytes, nil
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
	if len(match) < 2 {
		return "", nil, "", "", skerr.Fmt("Failed to find a From: line in message.")
	}
	from := string(match[1])

	match = subjectRegex.FindSubmatch(body)
	subject := defaultSubject
	if len(match) >= 2 {
		subject = string(bytes.TrimSpace(match[1]))
	}

	match = toRegex.FindSubmatch(body)
	if len(match) < 2 {
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
