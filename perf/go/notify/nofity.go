package notify

import (
	"bytes"
	"fmt"
	"html/template"

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
<p style="padding: 2em;">
	<a href="https://{{.SubDomain}}.skia.org/g/c/{{.Commit.Hash}}"> https://{{.SubDomain}}.skia.org/g/c/{{.Commit.Hash}}</a>
</p>
<p>
  For:
</p>
<p style="padding: 2em;">
  <a href="{{.Commit.URL}}">{{.Commit.URL}}</a>
</p>
<p>
	With {{.Cluster.Num}} matching traces.
</p>`
)

var (
	emailTemplate = template.Must(template.New("email").Parse(EMAIL))
)

// Email sending interface. Note that email.GMail implements this interface.
type Email interface {
	Send(from string, to []string, subject string, body string) error
}

type Notifier struct {
	email     Email
	subdomain string
}

func New(email Email, subdomain string) *Notifier {
	return &Notifier{
		email:     email,
		subdomain: subdomain,
	}
}

type context struct {
	SubDomain string
	Commit    *cid.CommitDetail
	Alert     *alerts.Config
	Cluster   *clustering2.ClusterSummary
}

func (n *Notifier) formatEmail(c *cid.CommitDetail, alert *alerts.Config, cl *clustering2.ClusterSummary) (string, error) {
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

func (n *Notifier) Send(c *cid.CommitDetail, alert *alerts.Config, cl *clustering2.ClusterSummary) error {
	body, err := n.formatEmail(c, alert, cl)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("Regression found for %q", c.Message)
	if err := n.email.Send(FROM_ADDRESS, []string{alert.Alert}, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %s", err)
	}

	return nil
}

func (n *Notifier) ExampleSend(alert *alerts.Config) error {
	// Fill in a sample cid and cluster and call Send to test out the alerts configuration.

	c := &cid.CommitDetail{
		Message: "Re-enable opList dependency tracking",
		URL:     "https://skia.googlesource.com/skia/+/d261e1075a93677442fdf7fe72aba7e583863664",
	}
	cl := &clustering2.ClusterSummary{
		Num: 10,
	}
	return n.Send(c, alert, cl)
}
