package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/util"

	specpb "go.skia.org/infra/cabe/go/proto"
)

func TestDisjointDimensions(t *testing.T) {
	for i, test := range []struct {
		name          string
		dimensionSets []map[string][]string
		ignoredKeys   util.StringSet
		expected      []string
	}{
		{
			name: "empty",
		},
		{
			name: "one element can't be disjoint with itself",
			dimensionSets: []map[string][]string{
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}},
			},
		},
		{
			name: "two elements with same dimensions",
			dimensionSets: []map[string][]string{
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}},
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}},
			},
		},
		{
			name: "two elements with one different dimension key",
			dimensionSets: []map[string][]string{
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}},
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}, "dim3": []string{"baz"}},
			},
			expected: []string{`key: "dim3", values: ["baz"]`},
		},
		{
			name: "two elements with one different dimension key, ignored",
			dimensionSets: []map[string][]string{
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}},
				{"dim1": []string{"foo"}, "dim2": []string{"bar"}, "dim3": []string{"baz"}},
			},
			ignoredKeys: util.NewStringSet([]string{"dim3"}),
			expected:    nil,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			got := disjointDimensions(test.dimensionSets, test.ignoredKeys)
			diff := cmp.Diff(got, test.expected, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("expected %+v got %+v\ndiff:%v", test.expected, got, diff)
			}
		})
	}
}

func TestDisjointTags(t *testing.T) {
	for i, test := range []struct {
		name        string
		tagSets     [][]string
		ignoredKeys util.StringSet
		expected    []string
	}{
		{
			name: "empty",
		},
		{
			name: "one element can't be disjoint with itself",
			tagSets: [][]string{
				{"tag1:foo", "tag2:bar"},
			},
		},
		{
			name: "two elements with same tags",
			tagSets: [][]string{
				{"tag1:foo", "tag2:bar"},
				{"tag1:foo", "tag2:bar"},
			},
		},
		{
			name: "two elements with one different tag key",
			tagSets: [][]string{
				{"tag1:foo", "tag2:bar"},
				{"tag1:foo", "tag2:bar", "tag3:baz"},
			},
			expected: []string{"tag3"},
		},
		{
			name: "two elements with one different tag key, ignored",
			tagSets: [][]string{
				{"tag1:foo", "tag2:bar"},
				{"tag1:foo", "tag2:bar", "tag3:baz"},
			},
			ignoredKeys: util.NewStringSet([]string{"tag3"}),
			expected:    nil,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			got := disjointTags(test.tagSets, test.ignoredKeys)
			diff := cmp.Diff(got, test.expected, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("expected %+v got %+v\ndiff:%v", test.expected, got, diff)
			}
		})
	}
}

func TestSwarmingTaskInfos_disjointRequestDimensions(t *testing.T) {
	for i, test := range []struct {
		name        string
		input       armTasks
		ignoredKeys util.StringSet
		expected    []string
	}{
		{
			name: "empty",
		},
		{
			name: "one element can't be disjoint with itself",
			input: []*armTask{
				{
					taskInfo: swarmingTaskInfo{
						request: &swarming.SwarmingRpcsTaskRequest{
							Properties: &swarming.SwarmingRpcsTaskProperties{
								Dimensions: []*swarming.SwarmingRpcsStringPair{
									{
										Key:   "gpu",
										Value: "10de",
									},
								},
							},
						},
						result: &swarming.SwarmingRpcsTaskResult{
							BotDimensions: []*swarming.SwarmingRpcsStringListPair{
								{
									Key: "gpu",
									Value: []string{
										"10de",
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			got := armTasks(test.input).disjointRequestDimensions(test.ignoredKeys)
			diff := cmp.Diff(got, test.expected, cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("expected %+v got %+v\ndiff:%v", test.expected, got, diff)
			}
		})
	}
}

func TestCheckRunTask(t *testing.T) {
	for i, test := range []struct {
		name             string
		taskInfo         *swarmingTaskInfo
		expectedFindings []string
		options          []CheckerOptions
	}{
		{
			name:             "empty",
			expectedFindings: []string{"CheckRunTask: taskInfo was nil"},
		}, {
			name: "run task missing expected request tags",
			taskInfo: &swarmingTaskInfo{
				taskID: "task0",
				request: &swarming.SwarmingRpcsTaskRequest{
					Properties: &swarming.SwarmingRpcsTaskProperties{},
				},
				result: &swarming.SwarmingRpcsTaskResult{
					BotDimensions: []*swarming.SwarmingRpcsStringListPair{},
				},
			},
			options: []CheckerOptions{
				ExpectRunRequestTagKeys("foo"),
			},
			expectedFindings: []string{`CheckRunTask "task0": missing request tag "foo"`},
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			c := NewChecker(test.options...)
			c.CheckRunTask(test.taskInfo)
			diff := cmp.Diff(c.Findings(), test.expectedFindings)
			if diff != "" {
				t.Errorf("expected %v, got %v\ndiff:%v", test.expectedFindings, c.Findings(), diff)
			}
		})
	}
}

func TestCheckArmComparability(t *testing.T) {
	for i, test := range []struct {
		name                 string
		controls, treatments *processedArmTasks
		expectedFindings     []string
		options              []CheckerOptions
	}{
		{
			name:       "empty",
			controls:   &processedArmTasks{},
			treatments: &processedArmTasks{},
		},
		{
			// See https://screenshot.googleplex.com/9RCYmMmj2N5hQBA for an example of this situation.
			name: "different gpu driver versions",
			controls: &processedArmTasks{
				tasks: []*armTask{
					{
						taskInfo: swarmingTaskInfo{
							request: &swarming.SwarmingRpcsTaskRequest{
								Properties: &swarming.SwarmingRpcsTaskProperties{
									Dimensions: []*swarming.SwarmingRpcsStringPair{
										{
											Key:   "",
											Value: "",
										},
									},
								},
							},
							result: &swarming.SwarmingRpcsTaskResult{
								BotDimensions: []*swarming.SwarmingRpcsStringListPair{
									{
										Key: "gpu",
										Value: []string{
											"10de",
											"10de:1cb3",
											"10de:1cb3-390.116",
										},
									},
								},
							},
						},
					},
					{
						taskInfo: swarmingTaskInfo{
							request: &swarming.SwarmingRpcsTaskRequest{
								Properties: &swarming.SwarmingRpcsTaskProperties{
									Dimensions: []*swarming.SwarmingRpcsStringPair{
										{
											Key:   "",
											Value: "",
										},
									},
								},
							},
							result: &swarming.SwarmingRpcsTaskResult{
								BotDimensions: []*swarming.SwarmingRpcsStringListPair{
									{
										Key: "gpu",
										Value: []string{
											"10de",
											"10de:1cb3",
											"10de:1cb3-440.100",
										},
									},
								},
							},
						},
					},
				},
			},
			treatments: &processedArmTasks{
				tasks: []*armTask{
					{
						taskInfo: swarmingTaskInfo{
							request: &swarming.SwarmingRpcsTaskRequest{
								Properties: &swarming.SwarmingRpcsTaskProperties{
									Dimensions: []*swarming.SwarmingRpcsStringPair{
										{
											Key:   "",
											Value: "",
										},
									},
								},
							},
							result: &swarming.SwarmingRpcsTaskResult{
								BotDimensions: []*swarming.SwarmingRpcsStringListPair{
									{
										Key: "gpu",
										Value: []string{
											"10de",
											"10de:1cb3",
											"10de:1cb3-390.116",
										},
									},
								},
							},
						},
					},
					{
						taskInfo: swarmingTaskInfo{
							request: &swarming.SwarmingRpcsTaskRequest{
								Properties: &swarming.SwarmingRpcsTaskProperties{
									Dimensions: []*swarming.SwarmingRpcsStringPair{
										{
											Key:   "",
											Value: "",
										},
									},
								},
							},
							result: &swarming.SwarmingRpcsTaskResult{
								BotDimensions: []*swarming.SwarmingRpcsStringListPair{
									{
										Key: "gpu",
										Value: []string{
											"10de",
											"10de:1cb3",
											"10de:1cb3-390.116",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedFindings: []string{
				`CheckArmComparability: disjoint result dims within control tasks: [key: "gpu", values: ["10de" "10de:1cb3" "10de:1cb3-390.116"] key: "gpu", values: ["10de" "10de:1cb3" "10de:1cb3-440.100"]]`,
				`CheckArmComparability: disjoint result dims across arms' tasks: [key: "gpu", values: ["10de" "10de:1cb3" "10de:1cb3-440.100"]]`,
			},
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			c := NewChecker(test.options...)

			c.CheckArmComparability(test.controls, test.treatments)

			diff := cmp.Diff(c.Findings(), test.expectedFindings)
			if diff != "" {
				t.Errorf("expected %v, got %v\ndiff:%v", test.expectedFindings, c.Findings(), diff)
			}
		})
	}
}

func TestCheckControlTreatmentSpecMatch(t *testing.T) {
	testRunSpec1 := &specpb.RunSpec{
		Os:                   "os_1",
		SyntheticProductName: "product_1",
	}
	testRunSpec2 := &specpb.RunSpec{
		Os:                   "os_2",
		SyntheticProductName: "product_2",
	}
	testRunSpec3 := &specpb.RunSpec{
		Os:                   "os_1",
		SyntheticProductName: "product_2",
	}
	for _, test := range []struct {
		name          string
		controlSpec   *specpb.ExperimentSpec
		treatmentSpec *specpb.ExperimentSpec
		expectError   bool
		expectErrMsg  string
	}{
		{
			name:          "Control spec has no control",
			controlSpec:   &specpb.ExperimentSpec{},
			treatmentSpec: &specpb.ExperimentSpec{},
			expectError:   true,
			expectErrMsg:  "control",
		}, {
			name: "Treatment spec has no treatment",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{},
			},
			treatmentSpec: &specpb.ExperimentSpec{},
			expectError:   true,
			expectErrMsg:  "treatment",
		}, {
			name: "Control run spec is nil",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{},
			},
			expectError:  true,
			expectErrMsg: "length",
		}, {
			name: "The length of control run spec is greater than 1",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1, testRunSpec2},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{},
			},
			expectError:  true,
			expectErrMsg: "length",
		}, {
			name: "Treatment run spec is nil",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{},
			},
			expectError:  true,
			expectErrMsg: "length",
		}, {
			name: "The length of treatment run spec is greater than 1",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1, testRunSpec2},
				},
			},
			expectError:  true,
			expectErrMsg: "length",
		}, {
			name: "The os of control and treatment run spec doesn't match",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec2},
				},
			},
			expectError:  true,
			expectErrMsg: "are not same",
		}, {
			name: "The synthetic product name of control and treatment run spec doesn't match",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec3},
				},
			},
			expectError:  true,
			expectErrMsg: "are not same",
		}, {
			name: "Control and treatment spec match",
			controlSpec: &specpb.ExperimentSpec{
				Control: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			treatmentSpec: &specpb.ExperimentSpec{
				Treatment: &specpb.ArmSpec{
					RunSpec: []*specpb.RunSpec{testRunSpec1},
				},
			},
			expectError: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewChecker()
			err := c.CheckControlTreatmentSpecMatch(test.controlSpec, test.treatmentSpec)
			t.Logf("Gots for test (%v)", err)

			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}

			if err == nil && test.expectError {
				t.Error("Expected error but not nil")
			}
		})
	}
}
