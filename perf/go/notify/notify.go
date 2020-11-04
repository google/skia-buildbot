// notify is a package for sending notification.
package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/stepfit"
)

var timeNow = time.Now

const (
	fromAddress = "alertserver@skia.org"
	email       = `<b>Alert</b><br><br>
<p>
	A Perf Regression ({{.Cluster.StepFit.Status}}) has been found at:
</p>
<p style="padding: 1em;">
	<a href="{{.URL}}/g/t/{{.Commit.GitHash}}">{{.URL}}/g/t/{{.Commit.GitHash}}</a>
</p>
<p>
  For:
</p>
<p style="padding: 1em;">
  <a href="{{.Commit.URL}}">{{.Commit.URL}}</a>
</p>
<p>
	With {{.Cluster.Num}} matching traces.
</p>
<p>
   And direction {{.Cluster.StepFit.Status}}.
</p>
`
)

var (
	emailTemplate = template.Must(template.New("email").Parse(email))

	emailAddressSplitter = regexp.MustCompile("[, ]+")
)

// Email sending interface. Note that email.GMail implements this interface.
type Email interface {
	Send(from string, to []string, subject string, body string, threadingReference string) (string, error)
}

// NoEmail implements Email but only logs the information without sending email.
type NoEmail struct{}

// Send implements the Email interface.
func (n NoEmail) Send(from string, to []string, subject string, body string, threadingReference string) (string, error) {
	sklog.Infof("Not sending email: From: %q To: %q Subject: %q Body: %q ThreadingReference: %q", from, to, subject, body, threadingReference)
	return "", nil
}

// Notifier sends notifications.
type Notifier struct {
	// email is the thing that sends email.
	email Email

	// url is the URL of this instance of Perf.
	url string
}

// New returns a new Notifier.
func New(email Email, url string) *Notifier {
	return &Notifier{
		email: email,
		url:   url,
	}
}

// context is used in expanding the emailTemplate.
type context struct {
	URL     string
	Commit  perfgit.Commit
	Alert   *alerts.Alert
	Cluster *clustering2.ClusterSummary
}

func (n *Notifier) formatEmail(c perfgit.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary) (string, error) {
	templateContext := &context{
		URL:     n.url,
		Commit:  c,
		Alert:   alert,
		Cluster: cl,
	}

	var b bytes.Buffer
	if err := emailTemplate.Execute(&b, templateContext); err != nil {
		return "", fmt.Errorf("Failed to format email body: %s", err)
	}
	return b.String(), nil
}

func splitEmails(s string) []string {
	ret := []string{}
	for _, e := range emailAddressSplitter.Split(s, -1) {
		if e != "" {
			ret = append(ret, e)
		}
	}
	return ret
}

// Send a notification for the given cluster found at the given commit. Where to send it is defined in the alerts.Config.
func (n *Notifier) Send(c perfgit.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary) error {
	if alert.Alert == "" {
		return fmt.Errorf("No notification sent. No email address set for alert #%s", alert.IDAsString)
	}
	body, err := n.formatEmail(c, alert, cl)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("%s - Regression found for %s", alert.DisplayName, c.Display(timeNow()))
	if _, err := n.email.Send(fromAddress, splitEmails(alert.Alert), subject, body, ""); err != nil {
		return fmt.Errorf("Failed to send email: %s", err)
	}

	return nil
}

// ExampleSend sends an example for dummy data for the given alerts.Config.
func (n *Notifier) ExampleSend(alert *alerts.Alert) error {
	c := perfgit.Commit{
		Subject:   "Re-enable opList dependency tracking",
		URL:       "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
		GitHash:   "d261e1075a93677442fdf7fe72aba7e583863664",
		Timestamp: 1498176000,
	}
	cl := &clustering2.ClusterSummary{
		Num: 10,
		StepFit: &stepfit.StepFit{
			Status: stepfit.HIGH,
		},
	}
	return n.Send(c, alert, cl)
}
