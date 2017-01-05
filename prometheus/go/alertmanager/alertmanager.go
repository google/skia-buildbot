package alertmanager

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

const (
	alert_email = `Alert(s): {{range .GroupLabels}}{{.}}{{end}}
{{range .Alerts}}
  Alert: {{.Labels.alertname}}
  Severity: {{.Labels.severity}}
  Status: {{.Status}}
	{{.Annotations.description}}
{{end}}`
)

var (
	emailTemplate = template.Must(template.New("alert_email").Parse(alert_email))
)

type AlertManagerRequest struct {
	Receiver string  `json:"receiver"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`

	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

// Alert holds one alert for notification templates.
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

func EmailBody(a AlertManagerRequest) (string, error) {
	var b bytes.Buffer
	if err := emailTemplate.Execute(&b, a); err != nil {
		return "", fmt.Errorf("Failed to template alert: %s", err)
	}
	return b.String(), nil
}
