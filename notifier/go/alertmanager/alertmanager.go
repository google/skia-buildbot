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

	alert_chat = `*{{range .GroupLabels}}{{.}}{{end}}*{{ $length := len .Alerts}}{{with index .Alerts 0}}{{ if eq $length 1 }}
  *{{.Status}}* ({{.Labels.severity}}) {{.Annotations.description}}
{{ else }} [{{$length}}]

  *{{.Status}}* {{.Annotations.description}}

{{end}} {{end}} {{ if ne $length 1 }} {{range .Alerts}}{{.Annotations.abbr}} {{end}}
{{end}}`
)

var (
	emailTemplate = template.Must(template.New("alert_email").Parse(alert_email))
	chatTemplate  = template.Must(template.New("alert_chat").Parse(alert_chat))
	loc           *time.Location
)

func init() {
	var err error
	loc, err = time.LoadLocation("America/New_York")
	if err != nil {
		sklog.Errorf("Failed to load time location: %s", err)
	}
}

// AlertManagerRequest is used to parse the incoming JSON.
type AlertManagerRequest struct {
	Receiver string   `json:"receiver"`
	Status   string   `json:"status"`
	Alerts   []*Alert `json:"alerts"`

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

// caps fixes capitalization in some fields.
func caps(a *AlertManagerRequest) *AlertManagerRequest {
	a.Status = strings.Title(a.Status)
	for _, alert := range a.Alerts {
		alert.Status = strings.Title(alert.Status)
		if severity, ok := alert.Labels["severity"]; ok {
			alert.Labels["severity"] = strings.Title(severity)
		}
	}
	return a
}

func extractRequest(r io.Reader) (*AlertManagerRequest, []string, time.Time, error) {
	request := &AlertManagerRequest{}
	if err := json.NewDecoder(r).Decode(request); err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("Failed to decode incoming AlertManagerRequest: %s", err)
	}
	sklog.Infof("AlertManagerRequest: %v", *request)
	for _, a := range request.Alerts {
		sklog.Infof("Alert: %v", *a)
	}

	startTime := time.Now()
	alertnames := []string{}
	for _, alert := range request.Alerts {
		alertnames = append(alertnames, alert.Labels["alertname"])
		if alert.StartsAt.Before(startTime) {
			startTime = alert.StartsAt
		}
	}
	if loc != nil {
		startTime = startTime.In(loc)
	}
	return caps(request), alertnames, startTime, nil
}

// Email returns the body and subject of an email to send for the given alerts.
func Email(r io.Reader) (string, string, error) {
	request, alertnames, startTime, err := extractRequest(r)
	if err != nil {
		return "", "", fmt.Errorf("Failed to extract request from JSON: %s", err)
	}

	subject := fmt.Sprintf("Alert: %s started at %s", strings.Join(alertnames, " "), startTime.Format("3:04pm MST (2 Jan 2006)"))
	var b bytes.Buffer
	if err := emailTemplate.Execute(&b, request); err != nil {
		return "", "", fmt.Errorf("Failed to template alert: %s", err)
	}
	return b.String(), subject, nil
}

// Chat returns the body of a chat message to send for the given alerts.
func Chat(r io.Reader) (string, error) {
	request, _, _, err := extractRequest(r)
	if err != nil {
		return "", fmt.Errorf("Failed to extract request from JSON: %s", err)
	}

	var b bytes.Buffer
	if err := chatTemplate.Execute(&b, request); err != nil {
		return "", fmt.Errorf("Failed to template alert: %s", err)
	}
	return b.String(), nil
}
