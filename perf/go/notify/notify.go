// notify is a package for sending notification.
package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
)

const (
	FROM_ADDRESS = "alertserver@skia.org"
	EMAIL        = `<b>Alert</b><br><br>
<p>
	A Perf Regression has been found at:
</p>
<p style="padding: 1em;">
	<a href="https://{{.SubDomain}}.skia.org/g/t/{{.Commit.Hash}}">https://{{.SubDomain}}.skia.org/g/t/{{.Commit.Hash}}</a>
</p>
<p>
  For:
</p>
<p style="padding: 1em;">
  <a href="{{.Commit.URL}}">{{.Commit.URL}}</a>
</p>
<p>
	With {{.Cluster.Num}} matching traces.
</p>`
)

var (
	emailTemplate = template.Must(template.New("email").Parse(EMAIL))

	emailAddressSplitter = regexp.MustCompile("[, ]+")
)

// Email sending interface. Note that email.GMail implements this interface.
type Email interface {
	Send(from string, to []string, subject string, body string) error
}

// NoEmail implements Email but only logs the information without sending email.
type NoEmail struct{}

func (n NoEmail) Send(from string, to []string, subject string, body string) error {
	sklog.Infof("Not sending email: From: %q To: %q Subject: %q Body: %q", from, to, subject, body)
	return nil
}

// Notifier sends notifications.
type Notifier struct {
	email     Email
	subdomain string
}

// New returns a new Notifier.
func New(email Email, subdomain string) *Notifier {
	return &Notifier{
		email:     email,
		subdomain: subdomain,
	}
}

type context struct {
	SubDomain string
	Commit    *cid.CommitDetail
	Alert     *alerts.Alert
	Cluster   *clustering2.ClusterSummary
}

func (n *Notifier) formatEmail(c *cid.CommitDetail, alert *alerts.Alert, cl *clustering2.ClusterSummary) (string, error) {
	templateContext := &context{
		SubDomain: n.subdomain,
		Commit:    c,
		Alert:     alert,
		Cluster:   cl,
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
func (n *Notifier) Send(c *cid.CommitDetail, alert *alerts.Alert, cl *clustering2.ClusterSummary) error {
	if alert.Alert == "" {
		return fmt.Errorf("No notification sent. No email address set for alert #%d", alert.ID)
	}
	body, err := n.formatEmail(c, alert, cl)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("%s - Regression found for %q", alert.DisplayName, c.Message)
	if err := n.email.Send(FROM_ADDRESS, splitEmails(alert.Alert), subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %s", err)
	}

	return nil
}

// ExampleSend sends an example for dummy data for the given alerts.Config.
func (n *Notifier) ExampleSend(alert *alerts.Alert) error {
	c := &cid.CommitDetail{
		Message: "Re-enable opList dependency tracking",
		URL:     "https://skia.googlesource.com/skia/+/d261e1075a93677442fdf7fe72aba7e583863664",
		Hash:    "d261e1075a93677442fdf7fe72aba7e583863664",
	}
	cl := &clustering2.ClusterSummary{
		Num: 10,
	}
	return n.Send(c, alert, cl)
}
