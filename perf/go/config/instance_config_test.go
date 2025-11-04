package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstanceConfig_ParseNewQueryConfigFields(t *testing.T) {
	jsonBody := `
{
	"query_config": {
		"default_param_selections": {
			"stat": ["value"]
		},
		"default_url_values": {
			"stat": "value"
		},
		"conditional_defaults": [
			{
				"trigger": {
					"param": "metric",
					"values": ["cpu"]
				},
				"apply": [
					{
						"param": "stat",
						"values": ["avg"],
						"select_only_first": false
					}
				]
			}
		],
		"default_trigger_priority": {
			"metric": ["subtest", "bench_type"]
		}
	}
}
`
	var cfg InstanceConfig
	err := json.Unmarshal([]byte(jsonBody), &cfg)
	require.NoError(t, err)

	require.Equal(t, []string{"value"}, cfg.QueryConfig.DefaultParamSelections["stat"])
	require.Equal(t, "value", cfg.QueryConfig.DefaultUrlValues["stat"])

	// Check conditional defaults
	require.Len(t, cfg.QueryConfig.ConditionalDefaults, 1)
	rule := cfg.QueryConfig.ConditionalDefaults[0]
	require.Equal(t, "metric", rule.Trigger.Param)
	require.Equal(t, []string{"cpu"}, rule.Trigger.Values)
	require.Len(t, rule.Apply, 1)
	apply := rule.Apply[0]
	require.Equal(t, "stat", apply.Param)
	require.Equal(t, []string{"avg"}, apply.Values)
	require.False(t, apply.SelectFirstAvailable)

	// Check default trigger priority
	require.Equal(t, []string{"subtest", "bench_type"}, cfg.QueryConfig.DefaultTriggerPriority["metric"])
}
