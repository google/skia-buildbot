package alertmanager

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

var (
	oneAlert = `{
  "receiver": "general",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "PerfAlert",
        "category": "general",
        "instance": "localhost:10110",
        "job": "perf",
        "monitor": "codelab-monitor",
        "severity": "warning"
      },
      "annotations": {
        "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
        "summary": "One or more untriaged clusters."
      },
      "startsAt": "2017-01-05T15:28:22.805-05:00",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://jcgregorio.cnc.corp.google.com:9090/graph?g0.expr=perf_clustering_untriaged+%3E+0\u0026g0.tab=0"
    }
  ],
  "groupLabels": {
    "alertname": "PerfAlert"
  },
  "commonLabels": {
    "alertname": "PerfAlert",
    "category": "general",
    "instance": "localhost:10110",
    "job": "perf",
    "monitor": "codelab-monitor",
    "severity": "warning"
  },
  "commonAnnotations": {
    "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
    "summary": "One or more untriaged clusters."
  },
  "externalURL": "http://jcgregorio.cnc.corp.google.com:10117"
}`

	twoAlerts = `{
  "receiver": "general",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "PerfAlert",
        "category": "general",
        "instance": "localhost:10110",
        "job": "perf",
        "monitor": "codelab-monitor",
        "severity": "warning"
      },
      "annotations": {
        "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
        "abbr": "skia-perf-a"
      },
      "startsAt": "2017-01-05T15:28:22.805-05:00",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://jcgregorio.cnc.corp.google.com:9090/graph?g0.expr=perf_clustering_untriaged+%3E+0\u0026g0.tab=0"
    },
    {
      "status": "firing",
      "labels": {
        "alertname": "PerfAlert",
        "category": "general",
        "instance": "localhost:10110",
        "job": "perf",
        "monitor": "codelab-monitor",
        "severity": "warning"
      },
      "annotations": {
        "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
        "abbr": "skia-perf-b"
      },
      "startsAt": "2017-01-05T15:28:22.805-05:00",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://jcgregorio.cnc.corp.google.com:9090/graph?g0.expr=perf_clustering_untriaged+%3E+0\u0026g0.tab=0"
    }
  ],
  "groupLabels": {
    "alertname": "PerfAlert"
  },
  "commonLabels": {
    "alertname": "PerfAlert",
    "category": "general",
    "instance": "localhost:10110",
    "job": "perf",
    "monitor": "codelab-monitor",
    "severity": "warning"
  },
  "commonAnnotations": {
    "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
    "summary": "One or more untriaged clusters."
  },
  "externalURL": "http://jcgregorio.cnc.corp.google.com:10117"
}`
)

func TestEmailBody(t *testing.T) {
	testutils.SmallTest(t)
	expectedBody := `<b>Alerts</b>: PerfAlert <br><br>

<table border="0" cellspacing="5" cellpadding="5">
  <tr>
    <th>Name</th>
    <th>Severity</th>
    <th>Status</th></th>
    <th>Description</th>
  </tr>
  
    <tr>
      <td>PerfAlert</td>
      <td>Warning</td>
      <td>Firing</td>
      <td>At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.</td>
    </tr>
  
</table>
`
	expectedSubject := `Alert: PerfAlert started at 3:28pm EST (5 Jan 2017)`

	body, subject, err := Email(bytes.NewBufferString(oneAlert))
	assert.NoError(t, err)
	assert.Equal(t, expectedBody, body)
	assert.Equal(t, expectedSubject, subject)
}

func TestChatlBody(t *testing.T) {
	testutils.SmallTest(t)

	expectedBody := `*PerfAlert*
  *Firing* (Warning) At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.
  `

	body, err := Chat(bytes.NewBufferString(oneAlert))
	assert.NoError(t, err)
	assert.Equal(t, expectedBody, body)

	expectedBody2 := `*PerfAlert* [2]

  *Firing* At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.

   skia-perf-a skia-perf-b 
`

	body, err = Chat(bytes.NewBufferString(twoAlerts))
	assert.NoError(t, err)
	assert.Equal(t, expectedBody2, body)
}
