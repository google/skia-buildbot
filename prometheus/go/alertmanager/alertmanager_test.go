package alertmanager

import (
	"bytes"
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestEmailBody(t *testing.T) {
	testutils.SmallTest(t)
	r := `{
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
      <td>warning</td>
      <td>firing</td>
      <td>At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.</td>
    </tr>
  
</table>
`
	expectedSubject := `Alert: PerfAlert started at 3:28pm EST (5 Jan 2017)`

	body, subject, err := Email(bytes.NewBufferString(r))
	assert.NoError(t, err)
	assert.Equal(t, expectedBody, body)
	assert.Equal(t, expectedSubject, subject)
}
