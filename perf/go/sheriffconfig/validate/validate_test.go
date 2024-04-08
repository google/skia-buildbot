package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
)

func TestValidateConfig_ValidConfig(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub A",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "~.*-perf",
								},
							},
							Exclude: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
								},
							},
						},
					},
				},
			},
			{
				Name:         "Sub B",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "mac-perf",
									Test: "Speedometer2",
								},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.NoError(t, err)
}

func TestValidateConfig_NoSubscriptions(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Config must have at least one Subscription.")
}

func TestValidateConfig_NoName(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Missing name.")
}

func TestValidateConfig_NoContactEmail(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{Name: "Sub Test"},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Subscription 'Sub Test' is missing contact_email.")
}

func TestValidateConfig_NoBugComponent(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Subscription 'Sub Test' is missing bug_component.")
}

func TestValidateConfig_NoAnomalyConfigs(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Subscription 'Sub Test' must have at least one Anomaly Config.")
}

func TestValidateConfig_NoMatchPatterns(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Anomaly config must have at least one match pattern.")
}

func TestValidateConfig_PatternWithAllEmptyFields(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Match Pattern at index 0: Pattern must have at least 1 explicit field declared.")
}

func TestValidateConfig_PatternWithInvalidRegex(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "~*)(mac-perf",
									Test: "Speedometer2",
								},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Match Pattern at index 0: Invalid Regex for 'bot' field: *)(mac-perf.")
}

func TestValidateConfig_InvalidExcludePattern(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "~.*-perf",
									Test: "Speedometer2",
								},
							},
							Exclude: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "bot2",
								},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Exclude Pattern at index 0: Pattern must only have 1 explicit field declared.")
}

func TestValidateConfig_NoDuplicateNames(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "mac-perf",
									Test: "Speedometer2",
								},
							},
						},
					},
				},
			},
			{
				Name:         "Sub Test",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []*pb.Pattern{
								{
									Main: "ChromiumPerf",
									Bot:  "mac-perf",
									Test: "Speedometer2",
								},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Found duplicated subscription name: Sub Test. Names must be unique.")
}
