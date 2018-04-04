package notifier

import (
	"bytes"
	"context"
	"html/template"
	"time"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/notifier"
)

const (
	subjectIssueUpdate = "The {{.ChildName}} into {{.ParentName}} AutoRoller has uploaded issue {{.IssueID}}"

	bodyModeChange    = "{{.User}} changed the mode to \"{{.Mode}}\" with message: {{.Message}}"
	subjectModeChange = "The {{.ChildName}} into {{.ParentName}} AutoRoller mode was changed"

	subjectThrottled     = "The {{.ChildName}} into {{.ParentName}} AutoRoller is throttled"
	bodySafetyThrottled  = "The roller is throttled because it attempted to upload too many CLs in too short a time.  The roller will unthrottle at {{.ThrottledUntil}}."
	bodySuccessThrottled = "The roller is throttled because it is configured not to land too many rolls within a time period. The roller will unthrottle at {{.ThrottledUntil}}."
)

var (
	subjectTmplIssueUpdate = template.Must(template.New("subjectIssueUpdate").Parse(subjectIssueUpdate))

	subjectTmplModeChange = template.Must(template.New("subjectModeChange").Parse(subjectModeChange))
	bodyTmplModeChange    = template.Must(template.New("bodyModeChange").Parse(bodyModeChange))

	subjectTmplThrottled     = template.Must(template.New("subjectThrottled").Parse(subjectThrottled))
	bodyTmplSafetyThrottled  = template.Must(template.New("bodySafetyThrottled").Parse(bodySafetyThrottled))
	bodyTmplSuccessThrottled = template.Must(template.New("bodySuccessThrottled").Parse(bodySuccessThrottled))
)

// tmplVars is a struct which contains information used to fill
// text templates in the Subject and Body fields of messages.
type tmplVars struct {
	ChildName      string
	IssueID        string
	IssueURL       string
	Mode           string
	Message        string
	ParentName     string
	ThrottledUntil string
	User           string
}

// AutoRollNotifier is a struct used for sending notifications from an
// AutoRoller. It is a convenience wrapper around notifier.Router.
type AutoRollNotifier struct {
	childName  string
	emailer    *email.GMail
	n          *notifier.Router
	parentName string
}

// Return an AutoRollNotifier instance.
func New(childName, parentName string, emailer *email.GMail) *AutoRollNotifier {
	return &AutoRollNotifier{
		childName:  childName,
		n:          notifier.NewRouter(emailer),
		parentName: parentName,
	}
}

// Return the underlying notifier.Router.
func (a *AutoRollNotifier) Router() *notifier.Router {
	return a.n
}

// Send a message.
func (a *AutoRollNotifier) send(ctx context.Context, vars *tmplVars, subjectTmpl, bodyTmpl *template.Template, severity notifier.Severity) error {
	vars.ChildName = a.childName
	vars.ParentName = a.parentName
	var subjectBytes bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBytes, vars); err != nil {
		return err
	}
	var bodyBytes bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBytes, vars); err != nil {
		return err
	}
	return a.n.Send(ctx, &notifier.Message{
		Subject:  subjectBytes.String(),
		Body:     bodyBytes.String(),
		Severity: severity,
	})
}

// Send an issue update message.
func (a *AutoRollNotifier) SendIssueUpdate(ctx context.Context, id, url, msg string) error {
	bodyTmpl, err := template.New("body").Parse(msg)
	if err != nil {
		return err
	}
	return a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplIssueUpdate, bodyTmpl, notifier.SEVERITY_INFO)
}

// Send a mode change message.
func (a *AutoRollNotifier) SendModeChange(ctx context.Context, user, mode, message string) error {
	return a.send(ctx, &tmplVars{
		Message: message,
		Mode:    mode,
		User:    user,
	}, subjectTmplModeChange, bodyTmplModeChange, notifier.SEVERITY_WARNING)
}

// Send a notification that the roller is safety-throttled.
func (a *AutoRollNotifier) SendSafetyThrottled(ctx context.Context, until time.Time) error {
	return a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSafetyThrottled, notifier.SEVERITY_ERROR)
}

// Send a notification that the roller is success-throttled.
func (a *AutoRollNotifier) SendSuccessThrottled(ctx context.Context, until time.Time) error {
	return a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSuccessThrottled, notifier.SEVERITY_WARNING)
}
