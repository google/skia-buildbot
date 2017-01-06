// alertmanager parses the JSON alerts sent from the Prometheus AlertManager
// and produces formatted emails from the data.
package alertmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
)

const (
	alert_email = `<b>Alerts</b>: {{range .GroupLabels}}{{.}}{{end}} <br><br>

<table border="0" cellspacing="5" cellpadding="5">
  <tr>
    <th>Name</th>
    <th>Severity</th>
    <th>Status</th></th>
    <th>Description</th>
  </tr>
  {{range .Alerts}}
    <tr>
      <td>{{.Labels.alertname}}</td>
      <td>{{.Labels.severity}}</td>
      <td>{{.Status}}</td>
      <td>{{.Annotations.description}}</td>
    </tr>
  {{end}}
</table>
`
)

var (
	emailTemplate = template.Must(template.New("alert_email").Parse(alert_email))
	loc           *time.Location
)

// AlertManagerRequest is used to parse the incoming JSON.
type AlertManagerRequest struct {
	Receiver string  `json:"receiver"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`

	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

// Alert is used in AlertManagerRequest.
type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	// TODO format is wrong, actually sent as:     "startsAt": "2017-01-05T15:21:52.805-05:00"
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"generatorURL"`
}

// Email returns the body and subject of an email to send for the given alerts.
func Email(r io.Reader) (string, string, error) {
	request := AlertManagerRequest{}
	if err := json.NewDecoder(r).Decode(&request); err != nil {
		return "", "", fmt.Errorf("Failed to decode incoming AlertManagerRequest: %s", err)
	}

	ts := time.Now()
	alertnames := []string{}
	for _, alert := range request.Alerts {
		alertnames = append(alertnames, alert.Labels["alertname"])
		if alert.StartsAt.Before(ts) {
			ts = alert.StartsAt
		}
	}
	if loc == nil {
		var err error
		loc, err = time.LoadLocation("America/New_York")
		if err != nil {
			sklog.Errorf("Failed to load time location: %s", err)
		}
	}
	if loc != nil {
		ts = ts.In(loc)
	}

	subject := fmt.Sprintf("Alert: %s started at %s", strings.Join(alertnames, " "), ts.Format("3:04pm MST (2 Jan 2006)"))
	var b bytes.Buffer
	if err := emailTemplate.Execute(&b, request); err != nil {
		return "", "", fmt.Errorf("Failed to template alert: %s", err)
	}
	return b.String(), subject, nil
}
