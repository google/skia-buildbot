package regression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
)

func TestBuildConfigsAndParamSet(t *testing.T) {
	unittest.SmallTest(t)

	c := Continuous{
		eventDriven: false,
		provider: func() ([]*alerts.Config, error) {
			// Only fill in ID since we are just testing if ch channel returns
			// what we set here.
			return []*alerts.Config{
				{
					ID: 1,
				},
				{
					ID: 3,
				},
			}, nil
		},
		paramsProvider: func() paramtools.ParamSet {
			return paramtools.ParamSet{
				"config": []string{"8888", "565"},
			}
		},
	}

	// Build channel.
	ch := c.buildConfigAndParamsetChannel()

	// Read value.
	cnp := <-ch

	// Confirm it conforms to expectations.
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
	assert.Len(t, cnp.configs, 2)
	ids := []int64{}
	for _, cfg := range cnp.configs {
		ids = append(ids, cfg.ID)
	}
	assert.Subset(t, []int64{1, 3}, ids)

	// Confirm we continue to get items from the channel.
	cnp = <-ch
	assert.Equal(t, c.paramsProvider(), cnp.paramset)
}
