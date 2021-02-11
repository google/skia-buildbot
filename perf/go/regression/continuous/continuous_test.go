package continuous

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
)

func TestBuildConfigsAndParamSet(t *testing.T) {
	unittest.SmallTest(t)

	c := Continuous{
		provider: func() ([]*alerts.Alert, error) {
			// Only fill in ID since we are just testing if ch channel returns
			// what we set here.
			return []*alerts.Alert{
				{
					IDAsString: "1",
				},
				{
					IDAsString: "3",
				},
			}, nil
		},
		paramsProvider: func() paramtools.ReadOnlyParamSet {
			return paramtools.ReadOnlyParamSet{
				"config": []string{"8888", "565"},
			}
		},
		pollingDelay: time.Nanosecond,
		instanceConfig: &config.InstanceConfig{
			DataStoreConfig: config.DataStoreConfig{},
			GitRepoConfig:   config.GitRepoConfig{},
			IngestionConfig: config.IngestionConfig{},
		},
		flags: &config.FrontendFlags{},
	}

	// Build channel.
	ch := c.buildConfigAndParamsetChannel()

	// Read value.
	cnp := <-ch

	// Confirm it conforms to expectations.
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
	assert.Len(t, cnp.configs, 2)
	ids := []string{}
	for _, cfg := range cnp.configs {
		ids = append(ids, cfg.IDAsString)
	}
	assert.Subset(t, []string{"1", "3"}, ids)

	// Confirm we continue to get items from the channel.
	cnp = <-ch
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
}

func TestAllRequestsFromBaseRequest_WithValidGroupBy_Success(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := regression.NewRegressionDetectionRequest() // Doesn't take GroupBy into consideration.
	alert := alerts.NewConfig()
	alert.GroupBy = "config"
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps)
	assert.Len(t, allRequests, 2)
	assert.Contains(t, []string{"arch=x86&config=8888", "arch=x86&config=565"}, allRequests[0].Query)
}

func TestAllRequestsFromBaseRequest_WithInvalidValidGroupBy_Success(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := regression.NewRegressionDetectionRequest() // Doesn't take GroupBy into consideration.
	alert := alerts.NewConfig()
	alert.GroupBy = "SomeUnknownKey"
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps)
	assert.Len(t, allRequests, 0)
}

func TestAllRequestsFromBaseRequest_WithoutGroupBy_Success(t *testing.T) {
	unittest.SmallTest(t)

	baseRequest := regression.NewRegressionDetectionRequest() // Doesn't take GroupBy into consideration.
	alert := alerts.NewConfig()
	alert.GroupBy = ""
	alert.Query = "arch=x86"
	baseRequest.Alert = alert
	ps := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
		"arch":   []string{"x86", "arm"},
	}
	allRequests := allRequestsFromBaseRequest(baseRequest, ps)
	// With no GroupBy a slice with just the baseRequest is returned.
	assert.Len(t, allRequests, 1)
	assert.Equal(t, baseRequest, allRequests[0])
}
