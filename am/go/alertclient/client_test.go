package alertclient

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSunnyDayGetAlerts(t *testing.T) {
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "alert-manager:9000")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(INCIDENTS_RESPONSE)),
	}
	mc.On("Get", "http://alert-manager:9000/_/incidents").Return(&mockResponse, nil)

	alerts, err := client.GetAlerts()
	assert.NoError(t, err)
	assert.Len(t, alerts, 4)

	// spot check some values
	assert.Equal(t, "build32-a9", alerts[0].Params["bot"])
	assert.Equal(t, "BotMissing", alerts[0].Params["alertname"])
	assert.Equal(t, "build37-m5", alerts[1].Params["bot"])
	assert.Equal(t, "BotMissing", alerts[1].Params["alertname"])
	assert.Equal(t, "skia-push:20000", alerts[2].Params["instance"])
	assert.Equal(t, "DirtyPackages", alerts[2].Params["alertname"])
	assert.Equal(t, "MissingData", alerts[3].Params["alertname"])
}

func TestSunnyDayGetSileces(t *testing.T) {
	mc := &mockhttpclient{}
	defer mc.AssertExpectations(t)
	client := New(mc, "alert-manager:9000")

	mockResponse := http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewBufferString(SILENCES_RESPONSE)),
	}
	mc.On("Get", "http://alert-manager:9000/_/silences").Return(&mockResponse, nil)

	silences, err := client.GetSilences()
	assert.NoError(t, err)
	assert.Len(t, silences, 3)

	fmt.Printf("%#v", silences)

	assert.Equal(t, []string{"skia-rpi-047"}, silences[0].ParamSet["bot"])
	assert.Equal(t, []string{"BotUnemployed"}, silences[0].ParamSet["alertname"])
	assert.Equal(t, []string{"build16-a9", "build19-a9", "build22-a9", "build23-a9"}, silences[1].ParamSet["bot"])
	assert.Equal(t, []string{"skia-datahopper2:20000"}, silences[2].ParamSet["instance"])
	assert.Equal(t, []string{"BotUnemployed"}, silences[2].ParamSet["alertname"])
}

type mockhttpclient struct {
	mock.Mock
}

func (m *mockhttpclient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

var INCIDENTS_RESPONSE = `[
  {
    "key": "hh",
    "id": "11c5d1865a972745d92edb25de38e458",
    "active": true,
    "start": 1540782384,
    "last_seen": 1540823889,
    "params": {
      "__state__": "active",
      "abbr": "build32-a9",
      "alertname": "BotMissing",
      "bot": "build32-a9",
      "category": "infra",
      "description": "Swarming bot build32-a9 is missing. https://chromium-swarm.appspot.com/bot?id=build32-a9 https://goto.google.com/skolo-maintenance",
      "id": "11c5d865a72745d9edb25de38e458",
      "instance": "skia-datahopper2:20000",
      "job": "datahopper",
      "link_to_source": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen%7Bbot%21~%22%28ct-gce-.%2A%29%7C%28build4.%2Bdevice.%2B%29%22%7D+%2F+1000+%2F+1000+%2F+1000+%2F+60+%3E+15&g0.tab=1",
      "location": "google.com:skia-buildbots",
      "pool": "Skia",
      "severity": "critical",
      "swarming": "chromium-swarm.appspot.com"
    },
    "notes": null
  },
  {
    "key": "jj",
    "id": "506d79cc57a66d5e030d36cf05af0fbe",
    "active": true,
    "start": 1540782384,
    "last_seen": 1540823888,
    "params": {
      "__state__": "active",
      "abbr": "build37-m5",
      "alertname": "BotMissing",
      "bot": "build37-m5",
      "category": "infra",
      "description": "Swarming bot build37-m5 is missing. https://chrome-swarming.appspot.com/bot?id=build37-m5 https://goto.google.com/skolo-maintenance",
      "id": "506d79cc57a66030d36cf05af0fbe",
      "instance": "skia-datahopper2:20000",
      "job": "datahopper",
      "link_to_source": "https://prom.skia.org/graph?g0.expr=swarming_bots_last_seen%7Bbot%21~%22%28ct-gce-.%2A%29%7C%28build4.%2Bdevice.%2B%29%22%7D+%2F+1000+%2F+1000+%2F+1000+%2F+60+%3E+15&g0.tab=1",
      "location": "google.com:skia-buildbots",
      "pool": "CT",
      "severity": "critical",
      "swarming": "chrome-swarming.appspot.com"
    },
    "notes": null
  },
  {
    "key": "kk",
    "id": "542ecc822aed513c25db53eb834461fa",
    "active": true,
    "start": 1540797218,
    "last_seen": 1540823888,
    "params": {
      "__state__": "active",
      "alertname": "DirtyPackages",
      "assigned_to": "stephana@google.com",
      "category": "infra",
      "description": "One or more dirty packages have been running for more than 24 hours. https://push.skia.org",
      "id": "542ecc822aed513c25db53eb834461fa",
      "instance": "skia-push:20000",
      "job": "push",
      "link_to_source": "https://prom.skia.org/graph?g0.expr=min_over_time%28dirty_packages%5B25h%5D%29+%3E%3D+1&g0.tab=1",
      "location": "google.com:skia-buildbots",
      "severity": "warning"
    },
    "notes": null
  },
  {
    "key": "ll",
    "id": "6eb5b4904823e593b36306e267546ad7",
    "active": true,
    "start": 1540782384,
    "last_seen": 1540823888,
    "params": {
      "__state__": "active",
      "abbr": "max(swarming_bots_last_seen{bot=~\"ct-gce-.*\"})/1024/1024/1024/60*max(ct_gce_bots_up)",
      "alertname": "MissingData",
      "category": "infra",
      "description": "There is no data for the following alert: max(swarming_bots_last_seen{bot=~\"ct-gce-.*\"})/1024/1024/1024/60*max(ct_gce_bots_up)",
      "id": "6eb5b4923e593b36306e2676ad7",
      "link_to_source": "https://prom.skia.org/graph?g0.expr=absent%28max%28swarming_bots_last_seen%7Bbot%3D~%22ct-gce-.%2A%22%7D%29+%2F+1024+%2F+1024+%2F+1024+%2F+60+%2A+max%28ct_gce_bots_up%29%29&g0.tab=1",
      "location": "google.com:skia-buildbots",
      "severity": "critical"
    },
    "notes": null
  }
]`

var SILENCES_RESPONSE = `[
  {
    "key": "ss",
    "active": true,
    "user": "kjlubick@google.com",
    "param_set": {
      "__state__": [
        "active"
      ],
      "abbr": [
        "skia-rpi-047"
      ],
      "alertname": [
        "BotUnemployed"
      ],
      "bot": [
        "skia-rpi-047"
      ],
      "category": [
        "infra"
      ],
      "instance": [
        "skia-datahopper2:20000"
      ],
      "job": [
        "datahopper"
      ],
      "link_to_source": [
        "https://prom.skia.org/graph?g0.expr=swarming_bots_last_task%7Bpool%3D~%22Skia.%2A%22%7D+%2F+1000+%2F+1000+%2F+1000+%2F+60+%2F+60+%3E%3D+72&g0.tab=1"
      ],
      "location": [
        "google.com:skia-buildbots"
      ],
      "pool": [
        "Skia"
      ],
      "severity": [
        "critical"
      ]
    },
    "created": 1538393977,
    "updated": 1538393977,
    "duration": "300d",
    "notes": [
      {
        "text": "Android bot removed",
        "author": "",
        "ts": 1538393989
      }
    ]
  },
  {
    "key": "aa",
    "active": true,
    "user": "jcgregorio@google.com",
    "param_set": {
      "__state__": [
        "active"
      ],
      "bot": [
        "build16-a9",
        "build19-a9",
        "build22-a9",
        "build23-a9"
      ]
    },
    "created": 1540811511,
    "updated": 1540811511,
    "duration": "2d",
    "notes": [
      {
        "text": "https://bugs.chromium.org/p/chromium/issues/detail?id=892099",
        "author": "",
        "ts": 1540568794
      },
      {
        "text": "Reactivated by \"borenet@google.com\".",
        "author": "borenet@google.com",
        "ts": 1540811511
      }
    ]
  },
  {
    "key": "bb",
    "active": true,
    "user": "kjlubick@google.com",
    "param_set": {
      "__state__": [
        "active"
      ],
      "abbr": [
        "skia-i-rpi-023",
        "skia-i-rpi-024"
      ],
      "alertname": [
        "BotUnemployed"
      ],
      "bot": [
        "skia-i-rpi-023",
        "skia-i-rpi-024"
      ],
      "category": [
        "infra"
      ],
      "instance": [
        "skia-datahopper2:20000"
      ],
      "job": [
        "datahopper"
      ],
      "link_to_source": [
        "https://prom.skia.org/graph?g0.expr=swarming_bots_last_task%7Bpool%3D~%22Skia.%2A%22%7D+%2F+1000+%2F+1000+%2F+1000+%2F+60+%2F+60+%3E%3D+72&g0.tab=1"
      ],
      "location": [
        "google.com:skia-buildbots"
      ],
      "pool": [
        "SkiaInternal"
      ],
      "severity": [
        "critical"
      ]
    },
    "created": 1534957125,
    "updated": 1534957125,
    "duration": "12w",
    "notes": [
      {
        "text": "",
        "author": "",
        "ts": 1534957134
      }
    ]
  }
]`
