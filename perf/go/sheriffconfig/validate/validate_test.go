package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
	"google.golang.org/protobuf/proto"
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
							Match: []string{
								"master=ChromiumPerf&bot=~.*-perf",
							},
							Exclude: []string{
								"master=ChromiumPerf",
							},
						},
					},
				},
				Instance: "chrome-internal",
			},
			{
				Name:         "Sub B",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"master=ChromiumPerf&bot=mac-perf&test=Speedometer2",
							},
						},
					},
				},
				Instance: "chrome-internal",
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

func TestValidateConfig_MissingInstance(t *testing.T) {
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
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Subscription 'Sub Test' is missing instance.")
}

func TestValidateConfig_NoAnomalyConfigs(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				Instance:     "chrome-internal",
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
				Instance: "chrome-internal",
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
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Match Pattern at index 0: Pattern must have at least 1 key declared.")
}

func TestValidateConfig_PatternWithExplicitEmptyFields(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"bot=&benchmark=",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Match Pattern at index 0: Explicit value for key must be non-empty. Key:")
}

func TestValidateConfig_PatternWithInvalidRegex(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"master=ChromiumPerf&bot=~*)(mac-perf&test=Speedometer2",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Match Pattern at index 0: Invalid Regex for 'bot' key: *)(mac-perf.")
}

func TestValidateConfig_InvalidExcludePattern(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test1@google.com",
				BugComponent: "A>B",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"master=ChromiumPerf&bot=~.*-perf&test=Speedometer2",
							},
							Exclude: []string{
								"master=ChromiumPerf&bot=bot2",
							},
						},
					},
				},
			},
		},
	}

	err := ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error for Subscription at index 0: Error for Anomaly Config at index 0: Error for Exclude Pattern at index 0: Pattern must only have 1 key declared.")
}

func TestValidateConfig_NoDuplicateNames(t *testing.T) {
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"master=ChromiumPerf&bot=mac-perf&test=Speedometer2",
							},
						},
					},
				},
			},
			{
				Name:         "Sub Test",
				ContactEmail: "test2@google.com",
				BugComponent: "A>B>c",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{
								"master=ChromiumPerf&bot=mac-perf&test=Speedometer2",
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

func TestDeserializeProto_InvalidPrototext(t *testing.T) {
	// Invalid sheriff config:
	// 	subscriptions {
	// 		invalidfield: "a"
	//	}
	content := "subscriptions {\n\tinvalidfield: \"a\"\n}"
	_, err := DeserializeProto(content)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to unmarshal prototext")
}

func TestDeserializeProto_ValidPrototext(t *testing.T) {
	// Valid sheriff config:
	//  subscriptions {
	//      name: "a"
	//  }
	content := "subscriptions {\n\tname: \"a\"\n}"
	config, err := DeserializeProto(content)

	require.NoError(t, err)

	expectedconfig := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name: "a",
			},
		},
	}

	// Use proto.Equal for comparison
	if !proto.Equal(config, expectedconfig) {
		t.Errorf("Protos are not equal")
	}
}

func TestValidateConfig_DetectionRuleExclusivity(t *testing.T) {
	// Case 1: Valid config with detection_rule only (no step, no threshold)
	config := &pb.SheriffConfig{
		Subscriptions: []*pb.Subscription{
			{
				Name:         "Sub Test",
				ContactEmail: "test@google.com",
				BugComponent: "A>B",
				Instance:     "chrome-internal",
				AnomalyConfigs: []*pb.AnomalyConfig{
					{
						Rules: &pb.Rules{
							Match: []string{"bot=mac-perf"},
						},
						DetectionRule: &pb.AnomalyDetectionRule{
							Rule: &pb.AnomalyDetectionRule_SimpleRule{
								SimpleRule: &pb.AlgorithmCheck{
									Step:      pb.AnomalyConfig_PERCENT_STEP,
									Threshold: 0.05,
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

	// Case 2: Invalid config with detection_rule and non-default step
	config.Subscriptions[0].AnomalyConfigs[0].Step = pb.AnomalyConfig_PERCENT_STEP
	err = ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both 'step' and 'detection_rule'")

	// Reset step, set threshold
	config.Subscriptions[0].AnomalyConfigs[0].Step = pb.AnomalyConfig_ORIGINAL_STEP
	config.Subscriptions[0].AnomalyConfigs[0].Threshold = 1.2
	err = ValidateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both 'threshold' and 'detection_rule'")
}
