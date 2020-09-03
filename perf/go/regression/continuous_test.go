package regression

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
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
		paramsProvider: func() paramtools.ParamSet {
			return paramtools.ParamSet{
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
