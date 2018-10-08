package testutils

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

// GetRecordedMetric returns the value that prometheus is reporting.
// Using this allows for unit tests to check that metrics are properly
// being calculated and sent. This approach was found to be less awkward
// than using mocks and is decenly performant.
// See datahopper/bot_metrics/bots_test.go for an example use.
func GetRecordedMetric(t testutils.TestingT, metricName string, tags map[string]string) string {
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
	metric := metricName + stringifyTags(tags)
	for _, s := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(s, metric) {
			return strings.Split(s, " ")[1]
		}
	}
	return "Could not find anything for " + metric
}

func stringifyTags(tags map[string]string) string {
	// Prometheus always puts tags/labels in order by key value
	// https://github.com/prometheus/client_golang/blob/94ff84a9a6ebb5e6eb9172897c221a64df3443bc/prometheus/desc.go#L106
	// We do the same for tests
	keys := []string{}
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	labelStrings := []string{}
	for _, k := range keys {
		labelStrings = append(labelStrings, fmt.Sprintf(`%s="%s"`, k, tags[k]))
	}
	return "{" + strings.Join(labelStrings, ",") + "}"
}
