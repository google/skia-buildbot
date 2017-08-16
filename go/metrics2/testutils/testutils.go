package testutils

import (
	"io/ioutil"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
)

// GetRecordedMetric returns the value that prometheus is reporting.
// Using this allows for unit tests to check that metrics are properly
// being calculated and sent. This approach was found to be less awkward
// than using mocks and is decenly performant.
// See datahopper/bot_metrics/bots_test.go for an example use.
func GetRecordedMetric(t *testing.T, metric string) string {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rw := httptest.NewRecorder()
	promhttp.HandlerFor(prometheus.DefaultRegisterer.(*prometheus.Registry), promhttp.HandlerOpts{
		ErrorLog:           nil,
		ErrorHandling:      promhttp.PanicOnError,
		DisableCompression: true,
	}).ServeHTTP(rw, req)
	resp := rw.Result()
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	for _, s := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(s, metric) {
			return strings.Split(s, " ")[1]
		}
	}
	return ""
}
