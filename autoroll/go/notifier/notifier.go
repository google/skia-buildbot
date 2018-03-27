package notifier

import (
	"bytes"
	"html/template"

	"go.skia.org/infra/go/notifier"
)

const (
	bodyModeChange = "{{.User}} changed the mode to \"{{.Mode}}\" with message: {{.Message}}"
	bodyThrottled  = "The roller is throttled because it attempted to upload too many CLs in too short a time."

	subjectIssueUpdate = "The {{.ChildName}} into {{.ParentName}} AutoRoller has uploaded issue {{.IssueID}}"
	subjectModeChange  = "The {{.ChildName}} into {{.ParentName}} AutoRoller mode was changed"
	subjectThrottled   = "The {{.ChildName}} into {{.ParentName}} AutoRoller is throttled!"
)

var (
	bodyTmplModeChange = template.Must(template.New("bodyModeChange").Parse(bodyModeChange))
	bodyTmplThrottled  = template.Must(template.New("bodyThrottled").Parse(bodyThrottled))

	subjectTmplIssueUpdate = template.Must(template.New("subjectIssueUpdate").Parse(subjectIssueUpdate))
	subjectTmplModeChange  = template.Must(template.New("subjectModeChange").Parse(subjectModeChange))
	subjectTmplThrottled   = template.Must(template.New("subjectThrottled").Parse(subjectThrottled))
)

// tmplVars is a struct which contains information used to fill
// text templates in the Subject and Body fields of messages.
type tmplVars struct {
	ChildName  string
	IssueID    string
	IssueURL   string
	Mode       string
	Message    string
	ParentName string
	User       string
}

// AutoRollNotifier is a struct used for sending notifications from an
// AutoRoller. It is a convenience wrapper around notifier.Manager.
type AutoRollNotifier struct {
	childName  string
	n          *notifier.Manager
	parentName string
}

// Return an AutoRollNotifier instance.
func New(childName, parentName string) *AutoRollNotifier {
	return &AutoRollNotifier{
		childName:  childName,
		n:          notifier.NewManager(),
		parentName: parentName,
	}
}

// Add a new Notifier. See docs for notifier.Manager.Add for more details.
func (a *AutoRollNotifier) Add(n notifier.Notifier, f notifier.Filter, singleThreadSubject string) {
	a.n.Add(n, f, singleThreadSubject)
}

// Send a message.
func (a *AutoRollNotifier) send(vars *tmplVars, subjectTmpl, bodyTmpl *template.Template, severity notifier.Severity) error {
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
	return a.n.Send(&notifier.Message{
		Subject:  string(subjectBytes.Bytes()),
		Body:     string(bodyBytes.Bytes()),
		Severity: severity,
	})
}

// Send an issue update message.
func (a *AutoRollNotifier) SendIssueUpdate(id, url, msg string) error {
	bodyTmpl, err := template.New("body").Parse(msg)
	if err != nil {
		return err
	}
	return a.send(&tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplIssueUpdate, bodyTmpl, notifier.SEVERITY_INFO)
}

// Send a mode change message.
func (a *AutoRollNotifier) SendModeChange(user, mode, message string) error {
	return a.send(&tmplVars{
		Message: message,
		Mode:    mode,
		User:    user,
	}, subjectTmplModeChange, bodyTmplModeChange, notifier.SEVERITY_WARNING)
}

// Send a notification that the roller is throttled.
func (a *AutoRollNotifier) SendThrottled() error {
	return a.send(&tmplVars{}, subjectTmplThrottled, bodyTmplThrottled, notifier.SEVERITY_ERROR)
}
