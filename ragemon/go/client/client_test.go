package client

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.skia.org/infra/go/httputils"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	// init() should have run by now.
	assert.Equal(t, "", client.hostName)

	// Set up an HTTP server that emulates the ragemon server.
	requests := 0
	requestBody := ""
	bodyError := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			bodyError = true
		}
		requestBody = string(b)

	}))
	defer ts.Close()

	// Create a metric before Init() is called, which should work.
	m, err := GetOrRegister("requests", nil)
	assert.NoError(t, err)
	m.Inc(1)
	assert.Equal(t, int64(1), m.value)
	m.Clear()
	assert.Equal(t, int64(0), m.value)
	m.Inc(1)
	assert.Equal(t, int64(1), m.value)

	assert.Equal(t, 1, len(client.metrics))
	assert.Equal(t, m, client.metrics[",meas=requests,"])

	// Now create Init() and confirm that all the metric keys get updated correctly.
	c := httputils.NewTimeoutClient()
	err = Init("perf", ts.URL, c, "skia-perf")
	time.Sleep(time.Second)
	assert.Equal(t, 1, requests)
	assert.NoError(t, err)

	assert.Equal(t, m, client.metrics[",app=perf,host=skia-perf,meas=requests,"])
	assert.Equal(t, int64(1), m.value)

	assert.Equal(t, ",app=perf,host=skia-perf,meas=requests, 1", requestBody)
	assert.False(t, bodyError)

	// Now confirm updates are sent correctly.
	m.Inc(2)
	oneStep()

	assert.Equal(t, ",app=perf,host=skia-perf,meas=requests, 3", requestBody)
	assert.False(t, bodyError)

	// Now that Init() has been called new metrics should have a full key from the beginning.
	m2 := MustGetOrRegister("404s", nil)
	m2.Dec(1)
	assert.Equal(t, int64(-1), m2.Value())
	assert.Equal(t, 2, len(client.metrics))
	assert.Equal(t, m2, client.metrics[",app=perf,host=skia-perf,meas=404s,"])

	// Call Init() again but w/o passing in the hostname.
	err = Init("perf", ts.URL, c, "")
	time.Sleep(time.Second)
	assert.NoError(t, err)

	// Test error conditions.
	_, err = GetOrRegister("not/valid", map[string]string{"foo": "bar"})
	assert.Error(t, err)

	_, err = GetOrRegister("good", map[string]string{"foo": "bar?bad"})
	assert.Error(t, err)
}
