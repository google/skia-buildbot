package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.skia.org/infra/go/httputils"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	assert.Equal(t, "", client.hostName)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	m, err := GetOrRegister("requests", nil)
	assert.NoError(t, err)
	m.Inc(1)
	assert.Equal(t, int64(1), m.value)

	assert.Equal(t, 1, len(client.metrics))
	assert.Equal(t, m, client.metrics[",meas=requests,"])

	c := httputils.NewTimeoutClient()
	err = Init("perf", ts.URL, c, "skia-perf")
	assert.NoError(t, err)

	assert.Equal(t, m, client.metrics[",app=perf,host=skia-perf,meas=requests,"])
}
