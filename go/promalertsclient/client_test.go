package promalertsclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/prometheus/alertmanager/dispatch"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
)

func TestSunnyDayNoFilter(t *testing.T) {
	testutils.SmallTest(t)
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "myalerts.skia.org")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(ALERTS_RESPONSE)),
	}
	mc.On("Get", "http://myalerts.skia.org/api/v1/alerts").Return(&mockResponse, nil)

	alerts, err := client.GetAlerts(nil)
	assert.NoError(t, err)
	assert.Len(t, alerts, 15, "There are exactly alerts")
}

func TestSunnyDayFilterGoneBots(t *testing.T) {
	testutils.SmallTest(t)
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "myalerts.skia.org")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(ALERTS_RESPONSE)),
	}
	mc.On("Get", "http://myalerts.skia.org/api/v1/alerts").Return(&mockResponse, nil)

	alerts, err := client.GetAlerts(func(a dispatch.APIAlert) bool {
		alertName := string(a.Labels["alertname"])
		return alertName == "BotMissing" || alertName == "BotQuarantined"
	})
	assert.NoError(t, err)
	assert.Len(t, alerts, 7, "There are exactly 7 dead and quarantined bots")

	assert.Equal(t, "skia-e-win-032", string(alerts[0].Labels["bot"]))
	assert.Equal(t, "skia-rpi-061", string(alerts[1].Labels["bot"]))
	assert.Equal(t, "skia-e-win-032", string(alerts[2].Labels["bot"]))
	assert.Equal(t, "skia-e-linux-001", string(alerts[3].Labels["bot"]))
	assert.Equal(t, "skia-e-win-055", string(alerts[4].Labels["bot"]))
	assert.Equal(t, "skia-rpi-054", string(alerts[5].Labels["bot"]))
	assert.Equal(t, "build87-m5", string(alerts[6].Labels["bot"]))

}

type mockhttpclient struct {
	mock.Mock
}

func (m *mockhttpclient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

var ALERTS_RESPONSE = `{
  "status": "success",
  "data": [
    {
      "labels": {
        "alertname": "AndroidMasterPerfUntriagedClusters",
        "category": "general",
        "instance": "skia-android-master-perf:20000",
        "job": "skiaperfd",
        "severity": "warning",
        "specialroute": "android-master",
        "subdomain": "android-master-perf"
      },
      "annotations": {
        "description": "At least one untriaged perf cluster has been found. Please visit https://android-master-perf.skia.org/t/ to triage.",
        "summary": "One or more untriaged clusters."
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=perf_clustering_untriaged%7Binstance%3D~%22skia-android-master-perf%3A20000%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "GoldExpiredIgnores",
        "category": "general",
        "instance": "skia-gold-prod:20001",
        "job": "gold",
        "severity": "warning"
      },
      "annotations": {
        "description": "At least one expired ignore rule has been found. Please visit https://gold.skia.org/ignores to delete or extend."
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_num_expired_ignore_rules%7Binstance%3D%22skia-gold-prod%3A20001%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotQuarantined",
        "bot": "skia-e-win-032",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-e-win-032",
        "description": "Swarming bot skia-e-win-032 is quarantined. https://chromium-swarm.appspot.com/bot?id=skia-e-win-032 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=avg_over_time%28swarming_bots_quarantined%5B10m%5D%29+%3E%3D+1&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotMissing",
        "bot": "skia-rpi-061",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-rpi-061",
        "description": "Swarming bot skia-rpi-061 is missing. https://chromium-swarm.appspot.com/bot?id=skia-rpi-061 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T14:47:37.819Z",
      "endsAt": "2017-05-23T15:05:37.819Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotMissing",
        "bot": "skia-e-win-032",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-e-win-032",
        "description": "Swarming bot skia-e-win-032 is missing. https://chromium-swarm.appspot.com/bot?id=skia-e-win-032 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotMissing",
        "bot": "skia-e-linux-001",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-e-linux-001",
        "description": "Swarming bot skia-e-linux-001 is missing. https://chromium-swarm.appspot.com/bot?id=skia-e-linux-001 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T13:01:37.819Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "NamedFiddles",
        "category": "infra",
        "instance": "skia-fiddle:20000",
        "job": "fiddle",
        "severity": "warning"
      },
      "annotations": {
        "description": "See https://fiddle.skia.org/f/ and https://skia.googlesource.com/buildbot/%2B/master/fiddle/PROD.md#named_fail"
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=named_failures+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "GoldUntriaged",
        "category": "general",
        "corpus": "gm",
        "instance": "skia-gold-prod:20001",
        "job": "gold",
        "severity": "warning",
        "type": "untriaged"
      },
      "annotations": {
        "description": "At least one untriaged GM has been found. Please visit https://gold.skia.org/ to triage."
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "GoldUntriaged",
        "category": "general",
        "corpus": "image",
        "instance": "skia-gold-prod:20001",
        "job": "gold",
        "severity": "warning",
        "type": "untriaged"
      },
      "annotations": {
        "description": "At least one untriaged GM has been found. Please visit https://gold.skia.org/ to triage."
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "GoldUntriaged",
        "category": "general",
        "corpus": "svg",
        "instance": "skia-gold-prod:20001",
        "job": "gold",
        "severity": "warning",
        "type": "untriaged"
      },
      "annotations": {
        "description": "At least one untriaged GM has been found. Please visit https://gold.skia.org/ to triage."
      },
      "startsAt": "2017-05-23T14:16:37.819Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotMissing",
        "bot": "skia-e-win-055",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-e-win-055",
        "description": "Swarming bot skia-e-win-055 is missing. https://chromium-swarm.appspot.com/bot?id=skia-e-win-055 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "TaskSchedulerDBFreePages",
        "category": "infra",
        "database": "task_scheduler_db",
        "instance": "skia-task-scheduler-internal:20000",
        "job": "task_scheduler",
        "metric": "FreePageCount",
        "severity": "critical"
      },
      "annotations": {
        "description": "There are a large number of free pages in the Task Scheduler DB on skia-task-scheduler-internal:20000. https://skia.googlesource.com/buildbot/%2B/master/task_scheduler/PROD.md#db_too_many_free_pages",
        "summary": "Task Scheduler DB excess free pages (skia-task-scheduler-internal:20000)"
      },
      "startsAt": "2017-05-23T12:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=bolt_db%7Bdatabase%3D%22task_scheduler_db%22%2Cmetric%3D%22FreePageCount%22%7D+%3E+150&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotQuarantined",
        "bot": "skia-rpi-054",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "Skia",
        "severity": "critical",
        "swarming": "chromium-swarm.appspot.com"
      },
      "annotations": {
        "abbr": "skia-rpi-054",
        "description": "Swarming bot skia-rpi-054 is quarantined. https://chromium-swarm.appspot.com/bot?id=skia-rpi-054 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T14:34:37.819Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=avg_over_time%28swarming_bots_quarantined%5B10m%5D%29+%3E%3D+1&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "PerfUntriagedClusters",
        "category": "general",
        "instance": "skia-perf:20000",
        "job": "skiaperfd",
        "severity": "warning"
      },
      "annotations": {
        "description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
        "summary": "One or more untriaged clusters."
      },
      "startsAt": "2017-05-23T12:40:37.819Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=perf_clustering_untriaged%7Binstance%3D%22skia-perf%3A20000%22%7D+%3E+0&g0.tab=0"
    },
    {
      "labels": {
        "alertname": "BotQuarantined",
        "bot": "build87-m5",
        "category": "infra",
        "instance": "skia-datahopper2:20000",
        "job": "datahopper",
        "pool": "CT",
        "severity": "critical",
        "swarming": "chrome-swarming.appspot.com"
      },
      "annotations": {
        "abbr": "build87-m5",
        "description": "Swarming bot build87-m5 is quarantined. https://chromium-swarm.appspot.com/bot?id=build87-m5 https://goto.google.com/skolo-maintenance"
      },
      "startsAt": "2017-05-23T11:35:37.804Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "https://prom.skia.org/graph?g0.expr=avg_over_time%28swarming_bots_quarantined%5B10m%5D%29+%3E%3D+1&g0.tab=0"
    }
  ]
}`
