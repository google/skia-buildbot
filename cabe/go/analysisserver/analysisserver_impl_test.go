package analysisserver

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/cabe/go/replaybackends"

	"go.skia.org/infra/bazel/go/bazel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fakeBenchmarkName = "fake benchmark name"
)

func startTestServer(t *testing.T, casResultReader backends.CASResultReader, swarmingTaskReader backends.SwarmingTaskReader) (cpb.AnalysisClient, func()) {
	serverListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()

	cpb.RegisterAnalysisServer(server, New(casResultReader, swarmingTaskReader))

	go func() {
		require.NoError(t, server.Serve(serverListener))
	}()

	clientConn, err := grpc.Dial(
		serverListener.Addr().String(),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second))

	require.NoError(t, err)

	closer := func() {
		// server.Stop will close the listener for us:
		// https://pkg.go.dev/google.golang.org/grpc#Server.Stop
		// Explicitly closing the listener causes the server.Serve
		// call to return an error, which causes this test to fail
		// even when the code under test behaves as expected.
		server.Stop()
	}

	client := cpb.NewAnalysisClient(clientConn)
	return client, closer
}

func TestAnalysisServiceServer_GetAnalysis(t *testing.T) {
	test := func(name string, request *cpb.GetAnalysisRequest, wantFirstResult *cpb.AnalysisResult, wantError bool) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(
				bazel.RunfilesDir(),
				"external/cabe_replay_data",
				// https://pinpoint-dot-chromeperf.appspot.com/job/16f46f1c260000
				"pinpoint_16f46f1c260000.zip")
			benchmarkName := "fake benchmark name"
			replayer := replaybackends.FromZipFile(
				path,
				benchmarkName,
			)
			client, closer := startTestServer(t, replayer.CASResultReader, replayer.SwarmingTaskReader)
			defer closer()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := client.GetAnalysis(ctx, request)

			if wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if request.ExperimentSpec == nil {
				assert.NotNil(t, r.InferredExperimentSpec)
			} else {
				assert.Nil(t, r.InferredExperimentSpec)
			}
			diff := cmp.Diff(wantFirstResult, r.Results[0], cmpopts.EquateEmpty(), cmpopts.EquateApprox(0, 0.03), protocmp.Transform())

			assert.Equal(t, diff, "", "diff should be empty")
		})
	}
	cabeSpec := &cpb.ExperimentSpec{
		Analysis: &cpb.AnalysisSpec{
			Benchmark: []*cpb.Benchmark{{
				Name: fakeBenchmarkName,
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
	analysisResults := []*cpb.AnalysisResult{
		{
			ExperimentSpec: &cpb.ExperimentSpec{
				Common:    cabeSpec.Common,
				Control:   cabeSpec.Control,
				Treatment: cabeSpec.Treatment,
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     fakeBenchmarkName,
							Workload: []string{"Compositing.Display.DrawToSwapUs"},
						},
					},
				},
			},
			Statistic: &cpb.Statistic{
				Upper:           -64.67566905094428,
				Lower:           -67.77167618955136,
				PValue:          3.610001186871159e-12,
				ControlMedian:   12861.869927307689,
				TreatmentMedian: 4220.841672500003,
			},
		},
	}
	test("basic request, no experiment spec", &cpb.GetAnalysisRequest{PinpointJobId: "123"}, analysisResults[0], false)
	test("basic request, including experiment spec", &cpb.GetAnalysisRequest{
		PinpointJobId: "123",
		ExperimentSpec: &cpb.ExperimentSpec{
			Common:    cabeSpec.Common,
			Control:   cabeSpec.Control,
			Treatment: cabeSpec.Treatment,
			Analysis: &cpb.AnalysisSpec{
				Benchmark: []*cpb.Benchmark{
					{
						Name:     fakeBenchmarkName,
						Workload: []string{"Compositing.Display.DrawToSwapUs"},
					},
				},
			},
		},
	}, analysisResults[0], false)

}
