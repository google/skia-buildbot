package analyzer

import (
	"context"
	"path/filepath"
	"testing"

	"go.skia.org/infra/bazel/go/bazel"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/cabe/go/replaybackends"
	cabe_stats "go.skia.org/infra/cabe/go/stats"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/stretchr/testify/assert"
)

const fakeBenchmarkName = "fake benchmark name"

var (
	cabeSpec = &cpb.ExperimentSpec{
		Analysis: &cpb.AnalysisSpec{
			Benchmark: []*cpb.Benchmark{{
				Name: "fake benchmark name",
				Workload: []string{
					"Compositing.Display.DrawToSwapUs",
					"Graphics.Smoothness.Jank.AllAnimations",
					"Graphics.Smoothness.Jank.AllSequences",
					"Graphics.Smoothness.PercentDroppedFrames3.AllAnimations",
					"Graphics.Smoothness.PercentDroppedFrames3.AllSequences",
					"Memory.GPU.PeakMemoryUsage2.PageLoad",
					"metrics_duration",
					"motionmark",
					"motionmarkLower",
					"motionmarkUpper",
					"renderingMetric_duration",
					"trace_import_duration",
					"umaMetric_duration",
				},
			}},
		},
		Common: &cpb.ArmSpec{
			BuildSpec: []*cpb.BuildSpec{{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "b692c01",
				},
			}},
			RunSpec: []*cpb.RunSpec{{
				Os:                   "Mac",
				SyntheticProductName: "Macmini9,1_arm64-64-Apple_M1_apple m1_16384_1_5693005.2",
			}},
		},
		Control: &cpb.ArmSpec{},
		Treatment: &cpb.ArmSpec{
			BuildSpec: []*cpb.BuildSpec{{
				GerritChanges: []*cpb.GerritChange{{
					PatchsetHash: "d4565f9",
				}},
			}},
		},
	}
	cabeResults = []Results{
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Compositing.Display.DrawToSwapUs",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -66.30783735699579,
				LowerCi:  -67.77167618955136,
				UpperCi:  -64.67566905094428,
				PValue:   3.610001186871159e-12,
				XMedian:  4220.841672500003,
				YMedian:  12861.869927307689,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Graphics.Smoothness.Jank.AllAnimations",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -16.73062483206894,
				LowerCi:  -21.120670031839595,
				UpperCi:  -13.138801973339953,
				PValue:   1.3489114306741e-07,
				XMedian:  2.8,
				YMedian:  3.2,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Graphics.Smoothness.Jank.AllSequences",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -16.73062483206894,
				LowerCi:  -21.120670031839595,
				UpperCi:  -13.138801973339953,
				PValue:   1.3489114306741e-07,
				XMedian:  2.8,
				YMedian:  3.2,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Graphics.Smoothness.PercentDroppedFrames3.AllAnimations",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -4.621028211793865,
				LowerCi:  -10.557585442069328,
				UpperCi:  1.7101263441554604,
				PValue:   0.12098409484052819,
				XMedian:  16.6,
				YMedian:  18.2,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Graphics.Smoothness.PercentDroppedFrames3.AllSequences",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -4.621028211793865,
				LowerCi:  -10.557585442069328,
				UpperCi:  1.7101263441554604,
				PValue:   0.12098409484052819,
				XMedian:  16.6,
				YMedian:  18.2,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "Memory.GPU.PeakMemoryUsage2.PageLoad",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: -4.247016655834912,
				LowerCi:  -5.278978221402397,
				UpperCi:  -3.2108781224263527,
				PValue:   1.8427925750551541e-09,
				XMedian:  72.66666666666667,
				YMedian:  75.33333333333333,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "metrics_duration",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 0.8482862740345709,
				LowerCi:  -0.2859781113841775,
				UpperCi:  1.858087165306177,
				PValue:   0.1592140742909307,
				XMedian:  6786.875,
				YMedian:  6709.8125,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "motionmark",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 32.379285235910714,
				LowerCi:  8.806785244533177,
				UpperCi:  54.97965702347043,
				PValue:   0.008840583696913873,
				XMedian:  420.8447767851758,
				YMedian:  393.5,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "motionmarkLower",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 51.31135620023197,
				LowerCi:  24.235675882547667,
				UpperCi:  80.72348755186569,
				PValue:   7.632395510914769e-05,
				XMedian:  397.89662355293524,
				YMedian:  318,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "motionmarkUpper",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 13.704720839623374,
				LowerCi:  -6.729178442198935,
				UpperCi:  31.114821028841778,
				PValue:   0.18215234689722948,
				XMedian:  443.39272609936535,
				YMedian:  497,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "renderingMetric_duration",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 0.68934637669682,
				LowerCi:  -0.2584134232097246,
				UpperCi:  1.6487635502799636,
				PValue:   0.12043215769345683,
				XMedian:  454,
				YMedian:  449,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "trace_import_duration",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 0.8448450266459018,
				LowerCi:  -0.36873794758962575,
				UpperCi:  1.9245630078938092,
				PValue:   0.1612044338739682,
				XMedian:  6316.666499999999,
				YMedian:  6247.375,
			},
		},
		{
			Benchmark:   fakeBenchmarkName,
			WorkLoad:    "umaMetric_duration",
			BuildConfig: "",
			RunConfig:   "Mac,arm",
			Statistics: &cabe_stats.BerfWilcoxonSignedRankedTestResult{
				Estimate: 0.0022244196845155884,
				LowerCi:  -0.0018183524108428273,
				UpperCi:  15.464824031447932,
				PValue:   0.3329511135593646,
				XMedian:  6,
				YMedian:  6,
			},
		},
	}
	commonArmSpec = &cpb.ArmSpec{
		BuildSpec: []*cpb.BuildSpec{
			{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "b692c01",
				},
			},
		},
		RunSpec: []*cpb.RunSpec{
			{
				Os:                   "Mac",
				SyntheticProductName: "Macmini9,1_arm64-64-Apple_M1_apple m1_16384_1_5693005.2",
			},
		},
	}

	analysisResults = []*cpb.AnalysisResult{
		{
			ExperimentSpec: &cpb.ExperimentSpec{
				Common:    cabeSpec.Common,
				Control:   cabeSpec.Control,
				Treatment: cabeSpec.Treatment,
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     fakeBenchmarkName,
							Workload: []string{cabeResults[0].WorkLoad},
						},
					},
				},
			},
			Statistic: &cpb.Statistic{
				Upper:           cabeResults[0].Statistics.UpperCi,
				Lower:           cabeResults[0].Statistics.LowerCi,
				PValue:          cabeResults[0].Statistics.PValue,
				ControlMedian:   cabeResults[0].Statistics.YMedian,
				TreatmentMedian: cabeResults[0].Statistics.XMedian,
			},
		},
	}
)

func TestRun_withReplayBackends(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(
		bazel.RunfilesDir(),
		"external/cabe_replay_data",
		// https://pinpoint-dot-chromeperf.appspot.com/job/16f46f1c260000
		"pinpoint_16f46f1c260000.zip")
	replayer := replaybackends.FromZipFile(
		path,
		fakeBenchmarkName,
	)
	a := New(
		"16f46f1c260000",
		WithCASResultReader(replayer.CASResultReader),
		WithSwarmingTaskReader(replayer.SwarmingTaskReader),
	)

	res, err := a.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(res), 13)

	// Check that the analyzer correctly inferred the ExperimentSpec from the
	// swarming task metadata and PerfResults json from RBE-CAS.
	gotExpSpec := a.ExperimentSpec()
	expectedExpSpec := cabeSpec
	diff := cmp.Diff(expectedExpSpec, gotExpSpec,
		cmpopts.EquateEmpty(),
		cmpopts.EquateApprox(0, 0.03),
		protocmp.Transform())
	assert.Equal(t, "", diff)

	// check CABE statistical results
	byWorkload := func(a, b Results) bool {
		return a.WorkLoad < b.WorkLoad
	}

	diff = cmp.Diff(cabeResults, res,
		cmpopts.SortSlices(byWorkload),
		cmpopts.EquateEmpty(),
		cmpopts.EquateApprox(0, 0.03),
		protocmp.Transform())

	assert.Equal(t, "", diff)

	gotAnalysisResults := a.AnalysisResults()
	assert.Equal(t, len(res), len(gotAnalysisResults), "Analyzer should generate the same number of Result structs as it does AnalysisResult protos")
	// Just check the first *cpb.AnalysisResult proto to make sure
	// it contains the same data as the first Result struct.
	diff = cmp.Diff(analysisResults[:1], gotAnalysisResults[:1],
		cmpopts.EquateEmpty(),
		cmpopts.EquateApprox(0, 0.03),
		protocmp.Transform())

	assert.Equal(t, "", diff)
}

func TestRun_withReplayBackends_tasksNotComplete(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(
		bazel.RunfilesDir(),
		"external/cabe_replay_data",
		// https://pinpoint-dot-chromeperf.appspot.com/job/17e73187160000
		"pinpoint_17e73187160000.zip")

	replayer := replaybackends.FromZipFile(
		path,
		fakeBenchmarkName,
	)
	a := New(
		"17e73187160000",
		WithExperimentSpec(
			&cpb.ExperimentSpec{
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     fakeBenchmarkName,
							Workload: []string{"Compile:duration"},
						},
					},
				},
			},
		),
		WithCASResultReader(replayer.CASResultReader),
		WithSwarmingTaskReader(replayer.SwarmingTaskReader),
	)

	res, err := a.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(res), 1)

	expectedResults := &cpb.Statistic{
		Upper:           54.891271,
		Lower:           27.297663,
		PValue:          0.007812,
		ControlMedian:   5502.464000,
		TreatmentMedian: 7824.926500,
	}
	gotAnalysisResults := a.AnalysisResults()
	assert.Equal(t, len(gotAnalysisResults), 1)

	diff := cmp.Diff(expectedResults, gotAnalysisResults[0].Statistic,
		cmpopts.EquateEmpty(),
		cmpopts.EquateApprox(0, 0.03),
		protocmp.Transform())

	assert.Equal(t, "", diff)
}
