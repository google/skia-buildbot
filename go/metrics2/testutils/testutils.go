package testutils

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/util"
)

// GetRecordedMetric returns the value that prometheus is reporting.
// Using this allows for unit tests to check that metrics are properly
// being calculated and sent. This approach was found to be less awkward
// than using mocks and is decently performant.
// See datahopper/bot_metrics/bots_test.go for an example use.
func GetRecordedMetric(t sktest.TestingT, metricName string, tags map[string]string) string {
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
	require.NoError(t, err)
	// b at this point looks like:
	// # HELP go_gc_duration_seconds A summary of the GC invocation durations.
	// # TYPE go_gc_duration_seconds summary
	// go_gc_duration_seconds{quantile="0"} 0
	// go_gc_duration_seconds{quantile="0.5"} 0
	// go_gc_duration_seconds{quantile="1"} 0
	// go_gc_duration_seconds_sum 0
	// # ...
	metric := metricName + stringifyTags(tags)
	for _, s := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(s, metric) {
			split := strings.Split(s, " ")
			return split[len(split)-1]
		}
	}
	return "Could not find anything for " + metric
}

// stringifyTags takes the given tags and returns them as would match the prometheus query
// format (e.g. `{key1="value1",key2="value2"}`) or "" if the map is empty.
func stringifyTags(tags map[string]string) string {
	if len(tags) == 0 {
		// Metrics w/o tags are stored on a line that look like:
		// go_goroutines 10
		// So we don't want to append {} otherwise, nothing will match.
		return ""
	}
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
