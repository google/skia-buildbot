package notify

import (
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
)

// Email sending interface. Note that email.GMail implements this interface.
type Email interface {
	Send(senderDisplayName string, to []string, subject string, body string) error
}

type Notifier struct {
	email Email
}

func New(email Email) *Notifier {
	return &Notifier{
		email: email,
	}
}

func Send(cid *cid.CommitDetail, alert *alerts.Config, cl *clustering2.ClusterSummary) error {
	// Look at alert.Alert and determine if we are sending an email or a webhook.
	// Format a message appropriate for the medium, possibly including:
	//   - link to commit on triage page.
	//   - link to commit in repo.
	//   - commit description.
	// Send formatted message.
	return nil
}

func ExampleSend(alert *alerts.Config) error {
	// Fill in a sample cid and cluster and call Send to test out the alerts configuration.
	return nil
}
