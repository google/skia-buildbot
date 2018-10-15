package promalertsclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestSunnyDayNoFilter(t *testing.T) {
	testutils.SmallTest(t)
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "myalerts.skia.org")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(ALERTS_GROUPS_RESPONSE)),
	}
	mc.On("Get", "http://myalerts.skia.org/api/v1/alerts/groups").Return(&mockResponse, nil)

	alerts, err := client.GetAlerts(nil)
	assert.NoError(t, err)
	assert.Len(t, alerts, 16)
}

func TestSunnyDayFilterGoneBots(t *testing.T) {
	testutils.SmallTest(t)
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "myalerts.skia.org")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(ALERTS_GROUPS_RESPONSE)),
	}
	mc.On("Get", "http://myalerts.skia.org/api/v1/alerts/groups").Return(&mockResponse, nil)

	alerts, err := client.GetAlerts(func(a Alert) bool {
		alertName := string(a.Labels["alertname"])
		return alertName == "BotMissing" || alertName == "BotQuarantined"
	})
	assert.NoError(t, err)
	assert.Len(t, alerts, 4)

	assert.Equal(t, "skia-rpi-130", string(alerts[0].Labels["bot"]))
	assert.True(t, alerts[0].Silenced, "rpi-130's alert is silenced")
	assert.Equal(t, "skia-e-win-055", string(alerts[1].Labels["bot"]))
	assert.True(t, alerts[1].Silenced, "win-055's alert is silenced")
	assert.Equal(t, "build87-m5", string(alerts[2].Labels["bot"]))
	assert.True(t, alerts[2].Silenced, "build87-m5's alert is silenced")
	assert.Equal(t, "build39-m5", string(alerts[3].Labels["bot"]))
	assert.False(t, alerts[3].Silenced, "build39-m5's alert is *not* silenced")

}

type mockhttpclient struct {
	mock.Mock
}

func (m *mockhttpclient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

var ALERTS_GROUPS_RESPONSE = `{
  "status": "success",
  "data": [
    {
      "labels": {
        "alertname": "AndroidMasterPerfUntriagedClusters"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "android-master",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=perf_clustering_untriaged%7Binstance%3D~%22skia-android-master-perf%3A20000%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "BotMissing"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
            {
              "labels": {
                "alertname": "BotMissing",
                "bot": "skia-rpi-130",
                "category": "infra",
                "instance": "skia-datahopper2:20000",
                "job": "datahopper",
                "pool": "Skia",
                "severity": "critical",
                "swarming": "chromium-swarm.appspot.com"
              },
              "annotations": {
                "abbr": "skia-rpi-130",
                "description": "Swarming bot skia-rpi-130 is missing. https://chromium-swarm.appspot.com/bot?id=skia-rpi-130 https://goto.google.com/skolo-maintenance"
              },
              "startsAt": "2017-05-25T15:23:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0",
              "inhibited": false,
              "silenced": "d5dfce4a-c7b6-4c10-b656-cef11f2c89fb"
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen+%2F+1024+%2F+1024+%2F+1024+%2F+60+%3E+15&g0.tab=0",
              "inhibited": false,
              "silenced": "83ff3ee5-30d3-40ab-9a0f-8d3e98218691"
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "BotQuarantined"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=avg_over_time%28swarming_bots_quarantined%5B10m%5D%29+%3E%3D+1&g0.tab=0",
              "inhibited": false,
              "silenced": "6c3c4f57-4369-4de4-902e-906f66a9623f"
            },
            {
              "labels": {
                "alertname": "BotQuarantined",
                "bot": "build39-m5",
                "category": "infra",
                "instance": "skia-datahopper2:20000",
                "job": "datahopper",
                "pool": "CT",
                "severity": "critical",
                "swarming": "chrome-swarming.appspot.com"
              },
              "annotations": {
                "abbr": "build39-m5",
                "description": "Swarming bot build39-m5 is quarantined. https://chromium-swarm.appspot.com/bot?id=build39-m5 https://goto.google.com/skolo-maintenance"
              },
              "startsAt": "2017-05-24T17:57:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=avg_over_time%28swarming_bots_quarantined%5B10m%5D%29+%3E%3D+1&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "CQWatcherCLsCount"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
            {
              "labels": {
                "alertname": "CQWatcherCLsCount",
                "category": "infra",
                "instance": "skia-cq-watcher:20000",
                "job": "cq_watcher",
                "severity": "warning"
              },
              "annotations": {
                "description": "There are 10 CLs or more in Skia's CL. https://skia.googlesource.com/buildbot/%2B/master/cq_watcher/PROD.md#too_many_cls",
                "summary": "Too many CLs in CQ."
              },
              "startsAt": "2017-05-26T16:16:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=cq_watcher_in_flight_waiting_in_cq%7Binstance%3D%22skia-cq-watcher%3A20000%22%2Cjob%3D%22cq_watcher%22%7D+%3E%3D+10&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "DirtyPackages"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
            {
              "labels": {
                "alertname": "DirtyPackages",
                "category": "infra",
                "instance": "skia-push:20000",
                "job": "push",
                "severity": "warning"
              },
              "annotations": {
                "description": "One or more dirty packages have been running for more than 24 hours. https://push.skia.org"
              },
              "startsAt": "2017-05-25T20:30:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=min_over_time%28dirty_packages%5B25h%5D%29+%3E%3D+1&g0.tab=0",
              "inhibited": false,
              "silenced": "c14d499d-aa1a-4c06-9a2c-a6d1cf3d2d52"
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "GoldExpiredIgnores"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "general",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_num_expired_ignore_rules%7Binstance%3D%22skia-gold-prod%3A20001%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "GoldUntriaged"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "general",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
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
              "startsAt": "2017-05-26T13:09:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=gold_status_by_corpus%7Binstance%3D%22skia-gold-prod%3A20001%22%2Ctype%3D%22untriaged%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "NamedFiddles"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "startsAt": "2017-05-26T17:30:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=named_failures+%3E+0&g0.tab=0",
              "inhibited": false,
              "silenced": "a5af782c-6ef1-4283-899a-307f340923a5"
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "PerfUntriagedClusters"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "general",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "startsAt": "2017-05-24T21:14:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=perf_clustering_untriaged%7Binstance%3D%22skia-perf%3A20000%22%7D+%3E+0&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "SkiaAutoRoll"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "general",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
            {
              "labels": {
                "alertname": "SkiaAutoRoll",
                "category": "general",
                "child_path": "src/third_party/skia",
                "instance": "skia-autoroll:20000",
                "job": "autoroll",
                "severity": "warning"
              },
              "annotations": {
                "description": "The last DEPS roll attempt for Skia failed. https://skia.googlesource.com/buildbot/%2B/master/autoroll/PROD.md#autoroll_failed."
              },
              "startsAt": "2017-05-25T14:35:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=autoroll_last_roll_result%7Bchild_path%3D%22src%2Fthird_party%2Fskia%22%7D+%3D%3D+0&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "SkiaAutoRoll24H"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "general",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
            {
              "labels": {
                "alertname": "SkiaAutoRoll24H",
                "category": "general",
                "child_path": "src/third_party/skia",
                "instance": "skia-autoroll:20000",
                "job": "autoroll",
                "name": "last-autoroll-landed",
                "severity": "warning",
                "type": "liveness"
              },
              "annotations": {
                "description": "The last-landed AutoRoll for Skia was over 24h ago. https://skia.googlesource.com/buildbot/%2B/master/autoroll/PROD.md#no_rolls_24h."
              },
              "startsAt": "2017-05-26T00:28:37.819Z",
              "endsAt": "0001-01-01T00:00:00Z",
              "generatorURL": "https://prom.skia.org/graph?g0.expr=liveness_last_autoroll_landed_s%7Bchild_path%3D%22src%2Fthird_party%2Fskia%22%7D+%2F+60+%2F+60+%3E+24&g0.tab=0",
              "inhibited": false
            }
          ]
        }
      ]
    },
    {
      "labels": {
        "alertname": "TaskSchedulerDBFreePages"
      },
      "blocks": [
        {
          "routeOpts": {
            "receiver": "skiabot",
            "groupBy": [
              "alertname"
            ],
            "groupWait": 300000000000,
            "groupInterval": 120000000000,
            "repeatInterval": 43200000000000
          },
          "alerts": [
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
              "generatorURL": "https://prom.skia.org/graph?g0.expr=bolt_db%7Bdatabase%3D%22task_scheduler_db%22%2Cmetric%3D%22FreePageCount%22%7D+%3E+150&g0.tab=0",
              "inhibited": false,
              "silenced": "2fef345c-2cbf-4348-9be2-0be289122958"
            }
          ]
        }
      ]
    }
  ]
}`
