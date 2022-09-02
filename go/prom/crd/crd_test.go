package crd

import (
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

const original = `apiVersion: monitoring.googleapis.com/v1
kind: Rules
metadata:
  name: perf
  namespace: default
spec:
  groups:
  - name: example
    interval: 30s
    rules:
    - alert: AndroidIngestFailures
      expr: rate(process_failures[1h]) > 0.01
      labels:
        category: infra
      annotations:
        description: Error rate for processing buildids is too high. See ...
    - alert: AndroidIngestLiveness
      expr: liveness_last_successful_add_s > 300
      for: 15m
      labels:
        category: infra
      annotations:
        description: Liveness for processing buildids is too high. See https://github.com/google/skia-buildbot/blob/main/android_ingest/PROD.md#liveness
`

func TestStructs_RoundTripYAMLDocThroughStructs_YAMLDocIsUnchanged(t *testing.T) {

	var deserialized Rules
	err := yaml.Unmarshal([]byte(original), &deserialized)
	require.NoError(t, err)

	reserialized, err := yaml.Marshal(deserialized)
	require.NoError(t, err)

	require.Equal(t, original, string(reserialized))
}

func TestRules_AddAbsentRules_AlertWithDoubleComparisonIsSkipped(t *testing.T) {

	rules := Rules{
		Spec: Spec{
			Groups: []Group{
				{
					Name:     "example",
					Interval: "15s",
					Rules: []Rule{
						{
							Alert: "ThisWillNotGetAnAbsentAlert",
							Expr:  "rate(process_failures[1h]) > 0.01 && rate(process_failures[1h]) < 10.0",
						},
						{
							Alert: "AndroidIngestLiveness",
							Expr:  "liveness_last_successful_add_s > 300",
						},
					},
				},
			},
		},
	}

	rules.AddAbsentRules("skia-public")

	expected := Rules{
		Spec: Spec{
			Groups: []Group{
				{
					Name:     "example",
					Interval: "15s",
					Rules: []Rule{
						{
							Alert: "ThisWillNotGetAnAbsentAlert",
							Expr:  "rate(process_failures[1h]) > 0.01 && rate(process_failures[1h]) < 10.0",
						},
						{
							Alert: "AndroidIngestLiveness",
							Expr:  "liveness_last_successful_add_s > 300",
						},
					},
				},
				// A new group should be added.
				{
					Name:     "absent-example",
					Interval: "15s",
					Rules: []Rule{
						// But the new group only contains one Alert, the one for AndroidIngestLiveness.
						{
							Alert: "Absent",
							Expr:  "absent(liveness_last_successful_add_s)",
							For:   absentAlertForDuration,
							Labels: map[string]string{
								"category": "infra",
								"severify": "critical",
							},
							Annotations: map[string]string{
								"abbr":        "AndroidIngestLiveness",
								"equation":    "liveness_last_successful_add_s",
								"description": "There is no data for the Alert: \"AndroidIngestLiveness\"",
							},
						},
					},
				},
			},
		},
	}

	require.Equal(t, expected, rules)
}

func TestRules_AddAbsentRules_AlertsOnlyAppearInIncludedClusters(t *testing.T) {

	rules := Rules{
		Spec: Spec{
			Groups: []Group{
				{
					Name:     "example",
					Interval: "15s",
					Rules: []Rule{
						{
							Alert: "ThisWillNotGetAnAbsentAlertOutSideOfSkiaPublic",
							Expr:  "go_goroutines",
							Annotations: map[string]string{
								onlyInClustersAnnotationKey: "skia-public",
							},
						},
						{
							Alert: "AndroidIngestLiveness",
							Expr:  "liveness_last_successful_add_s > 300",
						},
					},
				},
			},
		},
	}

	rules.AddAbsentRules("skia-corp")

	expected := Rules{
		Spec: Spec{
			Groups: []Group{
				{
					Name:     "example",
					Interval: "15s",
					Rules: []Rule{
						{
							Alert: "ThisWillNotGetAnAbsentAlertOutSideOfSkiaPublic",
							Expr:  "go_goroutines",
							Annotations: map[string]string{
								onlyInClustersAnnotationKey: "skia-public",
							},
						},
						{
							Alert: "AndroidIngestLiveness",
							Expr:  "liveness_last_successful_add_s > 300",
						},
					},
				},
				// A new group should be added.
				{
					Name:     "absent-example",
					Interval: "15s",
					Rules: []Rule{
						// But the new group only contains one Alert, the one for AndroidIngestLiveness.
						{
							Alert: "Absent",
							Expr:  "absent(liveness_last_successful_add_s)",
							For:   absentAlertForDuration,
							Labels: map[string]string{
								"category": "infra",
								"severify": "critical",
							},
							Annotations: map[string]string{
								"abbr":        "AndroidIngestLiveness",
								"equation":    "liveness_last_successful_add_s",
								"description": "There is no data for the Alert: \"AndroidIngestLiveness\"",
							},
						},
					},
				},
			},
		},
	}

	require.Equal(t, expected, rules)
}

func TestRuleSkip_OnlyInClusterAnnotationPresent_ReturnsTrueForMatchingClusterNames(t *testing.T) {
	rule := Rule{
		Annotations: map[string]string{
			onlyInClustersAnnotationKey: "skia-public, skia-corp",
		},
	}

	require.True(t, rule.Include("skia-public"))
	require.True(t, rule.Include("skia-corp"))
	require.False(t, rule.Include("this-is-not-a-matching-cluster-name"))
}

func TestRuleSkip_OnlyInClusterAnnotationAbsent_ReturnsTrue(t *testing.T) {
	rule := Rule{}

	require.True(t, rule.Include("skia-public"))
}
