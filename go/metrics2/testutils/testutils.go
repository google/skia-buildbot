package testutils

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
)

var (
	mtx sync.Mutex
)

type test struct {
	newRegistry *prometheus.Registry
	oldRegistry prometheus.Registerer
	t           *testing.T
}

// NewTest initializes the test to run with metrics. The caller MUST call
// Cleanup() when finished.
func NewTest(t *testing.T) *test {
	mtx.Lock()
	// TODO(borenet): In some cases, we also really need to clear the
	// metrics client (where current values for metrics are cached) for
	// each test. We can't currently do that here without introducing a
	// circular dependency.
	registry := prometheus.NewRegistry()
	oldRegistry := prometheus.DefaultRegisterer
	prometheus.DefaultRegisterer = registry
	return &test{
		newRegistry: registry,
		oldRegistry: oldRegistry,
		t:           t,
	}
}

func (t *test) Cleanup() {
	prometheus.DefaultRegisterer = t.oldRegistry
	mtx.Unlock()
}

// GetRecordedMetric returns the value that prometheus is reporting.
// Using this allows for unit tests to check that metrics are properly
// being calculated and sent. This approach was found to be less awkward
// than using mocks and is decenly performant.
// See datahopper/bot_metrics/bots_test.go for an example use.
func (t *test) GetRecordedMetric(metricName string, tags map[string]string) string {
	output := t.ListMetrics()
	metric := metricName + StringifyTags(tags)
	for _, s := range strings.Split(output, "\n") {
		if strings.HasPrefix(s, metric) {
			return strings.Split(s, " ")[1]
		}
	}
	return "Could not find anything for " + metric
}

// ListMetrics lists all of the known metrics and their values.
func (t *test) ListMetrics() string {
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
	assert.NoError(t.t, err)
	return string(b)
}

func StringifyTags(tags map[string]string) string {
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
