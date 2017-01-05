package alertmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailBody(t *testing.T) {
	a := AlertManagerRequest{
		Receiver: "general",
		Status:   "firing",
		Alerts: []Alert{
			Alert{
				Status: "firing",
				Labels: map[string]string{
					"monitor":   "codelab-monitor",
					"severity":  "warning",
					"alertname": "PerfAlert",
					"category":  "general",
					"instance":  "localhost:10110",
					"job":       "perf",
				},
				Annotations: map[string]string{
					"description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
					"summary":     "One or more untriaged clusters.",
				},
				GeneratorURL: "http://prom.skia.org/graph?g0.expr=perf_clustering_untriaged+%3E+0&g0.tab=0"},
		},
		GroupLabels: map[string]string{
			"alertname": "PerfAlert",
		},
		CommonLabels:      map[string]string{"monitor": "codelab-monitor", "severity": "warning", "alertname": "PerfAlert", "category": "general", "instance": "localhost:10110", "job": "perf"},
		CommonAnnotations: map[string]string{"description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.", "summary": "One or more untriaged clusters."},
		ExternalURL:       "https://prom.skia.org",
	}

	body, err := EmailBody(a)
	assert.NoError(t, err)
	assert.Equal(t, "Alert(s) Firing: PerfAlert\n\n\n  Status: firing\n  Severity: warning\n\tAt least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.\n\n", body)
}
