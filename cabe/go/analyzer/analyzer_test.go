package analyzer

import (
	"context"
	"path/filepath"
	"testing"

	"go.opencensus.io/trace"

	"go.skia.org/infra/bazel/go/bazel"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/cabe/go/replaybackends"
	cabe_stats "go.skia.org/infra/cabe/go/stats"
	"go.skia.org/infra/go/tracing/tracingtest"

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
				Estimate: -66.42026581108367,
				LowerCi:  -67.81744007796827,
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
				Estimate: -16.543955983162405,
				LowerCi:  -21.556758153745736,
				UpperCi:  -11.235135761402926,
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
				Estimate: -16.543955983162405,
				LowerCi:  -21.556758153745736,
				UpperCi:  -11.235135761402926,
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
				Estimate: -4.958510387907966,
				LowerCi:  -10.688376235186182,
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
				Estimate: -4.958510387907966,
				LowerCi:  -10.688376235186182,
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
				Estimate: -4.132084178180195,
				LowerCi:  -5.139127442879166,
				UpperCi:  -3.174103971744557,
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
				Estimate: 0.7015611255400733,
				LowerCi:  -0.5240466122385157,
				UpperCi:  1.7282707804197495,
				PValue:   0.25149635438751106,
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
				Estimate: 31.15739918803897,
				LowerCi:  6.414886823364019,
				UpperCi:  56.170944789716515,
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
				Estimate: 51.174468247411006,
				LowerCi:  20.75168162096137,
				UpperCi:  81.03505946711454,
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
				Estimate: 13.106570392527917,
				LowerCi:  -10.220424833044683,
				UpperCi:  32.37324271232311,
				PValue:   0.2123169782118901,
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
				Estimate: 0.5584261246817324,
				LowerCi:  -0.5081247375108067,
				UpperCi:  1.5631045526682597,
				PValue:   0.33855552600718997,
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
				Estimate: 0.6888594953676552,
				LowerCi:  -0.5104040328350878,
				UpperCi:  1.7512139378455638,
				PValue:   0.25699089113595686,
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
				UpperCi:  9.548884656603306,
				PValue:   0.5347774276405586,
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
	diag := a.Diagnostics()
	assert.NotNil(t, diag)
	assert.Equal(t, 0, len(diag.ExcludedSwarmingTasks))
	assert.Equal(t, 0, len(diag.ExcludedReplicas))
	assert.Equal(t, 128, len(diag.IncludedSwarmingTasks))
	assert.Equal(t, 64, len(diag.IncludedReplicas))
}

func TestRun_withReplayBackends_tracing(t *testing.T) {
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
	exporter := &tracingtest.Exporter{}
	trace.RegisterExporter(exporter)
	defer trace.UnregisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	res, err := a.Run(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(res), 13)
	assert.NotEmpty(t, exporter.SpanData())
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
		Upper:           54.461149,
		Lower:           29.047617,
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
	diag := a.Diagnostics()
	assert.NotNil(t, diag)
	assert.Equal(t, 4, len(diag.ExcludedSwarmingTasks))
	assert.Equal(t, 2, len(diag.ExcludedReplicas))
	assert.Equal(t, 16, len(diag.IncludedSwarmingTasks))
	assert.Equal(t, 8, len(diag.IncludedReplicas))
}
