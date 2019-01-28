package notifier

import (
	"bytes"
	"context"
	"html/template"
	"time"

	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/sklog"
)

const (
	subjectIssueUpdate = "The {{.ChildName}} into {{.ParentName}} AutoRoller has uploaded issue {{.IssueID}}"

	bodyModeChange    = "{{.User}} changed the mode to \"{{.Mode}}\" with message: {{.Message}}"
	subjectModeChange = "The {{.ChildName}} into {{.ParentName}} AutoRoller mode was changed"

	bodyStrategyChange    = "{{.User}} changed the next-roll-revision strategy to \"{{.Strategy}}\" with message: {{.Message}}"
	subjectStrategyChange = "The {{.ChildName}} into {{.ParentName}} AutoRoller next-roll-revision strategy was changed"

	subjectThrottled     = "The {{.ChildName}} into {{.ParentName}} AutoRoller is throttled"
	bodySafetyThrottled  = "The roller is throttled because it attempted to upload too many CLs in too short a time.  The roller will unthrottle at {{.ThrottledUntil}}."
	bodySuccessThrottled = "The roller is throttled because it is configured not to land too many rolls within a time period. The roller will unthrottle at {{.ThrottledUntil}}."

	subjectNewFailure = "The {{.ChildName}} into {{.ParentName}} roll has failed (issue {{.IssueID}})"
	bodyNewFailure    = "The most recent roll attempt failed, while the previous attempt succeeded: {{.IssueURL}}"

	subjectNewSuccess = "The {{.ChildName}} into {{.ParentName}} roll is successful again (issue {{.IssueID}})"
	bodyNewSuccess    = "The most recent roll attempt succeeded, while the previous attempt failed: {{.IssueURL}}"

	subjectLastNFailed = "The last {{.N}} {{.ChildName}} into {{.ParentName}} rolls have failed"
	bodyLastNFailed    = "The roll is failing consistently. Time to investigate. The most recent roll attempt is here: {{.IssueURL}}"

	footer = "\n\nThe AutoRoll server is located here: {{.ServerURL}}"
)

var (
	subjectTmplIssueUpdate = template.Must(template.New("subjectIssueUpdate").Parse(subjectIssueUpdate))

	subjectTmplModeChange = template.Must(template.New("subjectModeChange").Parse(subjectModeChange))
	bodyTmplModeChange    = template.Must(template.New("bodyModeChange").Parse(bodyModeChange))

	subjectTmplStrategyChange = template.Must(template.New("subjectStrategyChange").Parse(subjectStrategyChange))
	bodyTmplStrategyChange    = template.Must(template.New("bodyStrategyChange").Parse(bodyStrategyChange))

	subjectTmplThrottled     = template.Must(template.New("subjectThrottled").Parse(subjectThrottled))
	bodyTmplSafetyThrottled  = template.Must(template.New("bodySafetyThrottled").Parse(bodySafetyThrottled))
	bodyTmplSuccessThrottled = template.Must(template.New("bodySuccessThrottled").Parse(bodySuccessThrottled))

	subjectTmplNewFailure = template.Must(template.New("subjectNewFailure").Parse(subjectNewFailure))
	bodyTmplNewFailure    = template.Must(template.New("bodyNewFailure").Parse(bodyNewFailure))

	subjectTmplNewSuccess = template.Must(template.New("subjectNewSuccess").Parse(subjectNewSuccess))
	bodyTmplNewSuccess    = template.Must(template.New("bodyNewSuccess").Parse(bodyNewSuccess))

	subjectTmplLastNFailed = template.Must(template.New("subjectLastNFailed").Parse(subjectLastNFailed))
	bodyTmplLastNFailed    = template.Must(template.New("bodyLastNFailed").Parse(bodyLastNFailed))

	footerTmpl = template.Must(template.New("footer").Parse(footer))
)

// tmplVars is a struct which contains information used to fill
// text templates in the Subject and Body fields of messages.
type tmplVars struct {
	ChildName      string
	IssueID        string
	IssueURL       string
	Mode           string
	Message        string
	N              int
	ParentName     string
	ServerURL      string
	Strategy       string
	ThrottledUntil string
	User           string
}

// AutoRollNotifier is a struct used for sending notifications from an
// AutoRoller. It is a convenience wrapper around notifier.Router.
type AutoRollNotifier struct {
	childName    string
	configReader chatbot.ConfigReader
	emailer      *email.GMail
	n            *notifier.Router
	parentName   string
	serverURL    string
}

// Return an AutoRollNotifier instance.
func New(ctx context.Context, childName, parentName, serverURL string, emailer *email.GMail, chatBotConfigReader chatbot.ConfigReader, configs []*notifier.Config) (*AutoRollNotifier, error) {
	n := &AutoRollNotifier{
		childName:  childName,
		emailer:    emailer,
		n:          notifier.NewRouter(emailer, chatBotConfigReader),
		parentName: parentName,
		serverURL:  serverURL,
	}
	if err := n.ReloadConfigs(ctx, configs); err != nil {
		return nil, err
	}
	return n, nil
}

func (a *AutoRollNotifier) ReloadConfigs(ctx context.Context, configs []*notifier.Config) error {
	// Create a new router and add the specified configs to it.
	n := notifier.NewRouter(a.emailer, a.configReader)
	if err := n.AddFromConfigs(ctx, configs); err != nil {
		return err
	}
	a.n = n
	return nil
}

// Return the underlying notifier.Router.
func (a *AutoRollNotifier) Router() *notifier.Router {
	return a.n
}

// Send a message.
func (a *AutoRollNotifier) send(ctx context.Context, vars *tmplVars, subjectTmpl, bodyTmpl *template.Template, severity notifier.Severity) {
	vars.ChildName = a.childName
	vars.ParentName = a.parentName
	vars.ServerURL = a.serverURL
	var subjectBytes bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute subject template: %s", err)
		return
	}
	var bodyBytes bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute body template: %s", err)
		return
	}
	if err := footerTmpl.Execute(&bodyBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute footer template: %s", err)
		return
	}
	if err := a.n.Send(ctx, &notifier.Message{
		Subject:  subjectBytes.String(),
		Body:     bodyBytes.String(),
		Severity: severity,
	}); err != nil {
		// We don't want to block the roller on failure to send
		// notifications. Log the error and move on.
		sklog.Error(err)
	}
}

// Send an issue update message.
func (a *AutoRollNotifier) SendIssueUpdate(ctx context.Context, id, url, msg string) {
	bodyTmpl, err := template.New("body").Parse(msg)
	if err != nil {
		sklog.Errorf("Failed to send notification; failed to parse template from message: %s", err)
		return
	}
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplIssueUpdate, bodyTmpl, notifier.SEVERITY_INFO)
}

// Send a mode change message.
func (a *AutoRollNotifier) SendModeChange(ctx context.Context, user, mode, message string) {
	a.send(ctx, &tmplVars{
		Message: message,
		Mode:    mode,
		User:    user,
	}, subjectTmplModeChange, bodyTmplModeChange, notifier.SEVERITY_WARNING)
}

// Send a strategy change message.
func (a *AutoRollNotifier) SendStrategyChange(ctx context.Context, user, strategy, message string) {
	a.send(ctx, &tmplVars{
		Message:  message,
		Strategy: strategy,
		User:     user,
	}, subjectTmplStrategyChange, bodyTmplStrategyChange, notifier.SEVERITY_WARNING)
}

// Send a notification that the roller is safety-throttled.
func (a *AutoRollNotifier) SendSafetyThrottled(ctx context.Context, until time.Time) {
	a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSafetyThrottled, notifier.SEVERITY_ERROR)
}

// Send a notification that the roller is success-throttled.
func (a *AutoRollNotifier) SendSuccessThrottled(ctx context.Context, until time.Time) {
	a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSuccessThrottled, notifier.SEVERITY_INFO)
}

// Send a notification that the most recent roll succeeded when the roll before
// it failed.
func (a *AutoRollNotifier) SendNewSuccess(ctx context.Context, id, url string) {
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplNewSuccess, bodyTmplNewSuccess, notifier.SEVERITY_WARNING)
}

// Send a notification that the most recent roll failed when the roll before
// it succeeded.
func (a *AutoRollNotifier) SendNewFailure(ctx context.Context, id, url string) {
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplNewFailure, bodyTmplNewFailure, notifier.SEVERITY_WARNING)
}

// Send a notification that the last N roll attempts have failed.
func (a *AutoRollNotifier) SendLastNFailed(ctx context.Context, n int, url string) {
	a.send(ctx, &tmplVars{
		IssueURL: url,
		N:        n,
	}, subjectTmplLastNFailed, bodyTmplLastNFailed, notifier.SEVERITY_ERROR)
}
