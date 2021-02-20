package continuous

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMatchingConfigsFromTraceIDs_TraceIDSliceIsEmpty_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)

	config := alerts.NewConfig()
	config.Query = "foo=bar"
	traceIDs := []string{}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Empty(t, matchingConfigs)
}

func TestMatchingConfigsFromTraceIDs_OneConfigThatMatchesZeroTraces_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)

	config := alerts.NewConfig()
	config.Query = "arch=some-unknown-arch"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Empty(t, matchingConfigs)
}

func TestMatchingConfigsFromTraceIDs_OneConfigThatMatchesOneTrace_ReturnsTheOneConfig(t *testing.T) {
	unittest.SmallTest(t)

	config := alerts.NewConfig()
	config.Query = "arch=x86"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Len(t, matchingConfigs, 1)
}

func TestMatchingConfigsFromTraceIDs_TwoConfigsThatMatchesOneTrace_ReturnsBothConfigs(t *testing.T) {
	unittest.SmallTest(t)

	config1 := alerts.NewConfig()
	config1.Query = "arch=x86"
	config2 := alerts.NewConfig()
	config2.Query = "arch=arm"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config1, config2})
	require.Len(t, matchingConfigs, 2)
}

func TestMatchingConfigsFromTraceIDs_GroupByMatchesTrace_ReturnsConfigWithRestrictedQuery(t *testing.T) {
	unittest.SmallTest(t)

	config1 := alerts.NewConfig()
	config1.Query = "arch=x86"
	config1.GroupBy = "config"
	traceIDs := []string{
		",arch=x86,config=8888,",
		",arch=arm,config=8888,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config1})
	require.Len(t, matchingConfigs, 1)
	require.Equal(t, "arch=x86&config=8888", matchingConfigs[0].Query)
	_, err := url.ParseQuery(matchingConfigs[0].Query)
	require.NoError(t, err)
}

func TestMatchingConfigsFromTraceIDs_MultipleGroupByPartsMatchTrace_ReturnsConfigWithRestrictedQueryUsingAllMatchingGroupByKeys(t *testing.T) {
	unittest.SmallTest(t)

	config := alerts.NewConfig()
	config.Query = "arch=x86"
	config.GroupBy = "config,device"
	traceIDs := []string{
		",arch=x86,config=8888,device=Pixel4,",
		",arch=arm,config=8888,device=Pixel4,",
	}
	matchingConfigs := matchingConfigsFromTraceIDs(traceIDs, []*alerts.Alert{config})
	require.Len(t, matchingConfigs, 1)
	require.Equal(t, "arch=x86&config=8888&device=Pixel4", matchingConfigs[0].Query)
	_, err := url.ParseQuery(matchingConfigs[0].Query)
	require.NoError(t, err)
}
