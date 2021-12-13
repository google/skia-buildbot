// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate go run . -o ../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dryrun"
	"go.skia.org/infra/perf/go/frontend"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/pivot"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/continuous"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

type unionAndName struct {
	v        interface{}
	typeName string
}

func addMultipleUnions(generator *go2ts.Go2TS, unions []unionAndName) {
	for _, u := range unions {
		generator.AddUnionWithName(u.v, u.typeName)
	}
}

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddIgnoreNil(paramtools.Params{})
	generator.AddIgnoreNil(paramtools.ParamSet{})
	generator.AddIgnoreNil(paramtools.ReadOnlyParamSet{})
	generator.AddIgnoreNil(types.TraceSet{})

	generator.AddUnionToNamespace(pivot.AllOperations, "pivot")
	generator.AddToNamespace(pivot.Request{}, "pivot")

	generator.AddMultiple(generator,
		alerts.Alert{},
		alerts.AlertsStatus{},
		clustering2.ClusterSummary{},
		clustering2.ValuePercent{},
		continuous.Current{},
		dryrun.RegressionAtCommit{},
		frame.FrameRequest{},
		frame.FrameResponse{},
		frontend.AlertUpdateResponse{},
		frontend.ClusterStartResponse{},
		frontend.CommitDetailsRequest{},
		frontend.CountHandlerRequest{},
		frontend.CountHandlerResponse{},
		frontend.RangeRequest{},
		frontend.RegressionRangeRequest{},
		frontend.RegressionRangeResponse{},
		frontend.ShiftRequest{},
		frontend.ShiftResponse{},
		frontend.SkPerfConfig{},
		frontend.TriageRequest{},
		frontend.TriageResponse{},
		frontend.TryBugRequest{},
		frontend.TryBugResponse{},
		perfgit.Commit{},
		regression.FullSummary{},
		regression.RegressionDetectionRequest{},
		regression.RegressionDetectionResponse{},
		regression.TriageStatus{},
		results.TryBotRequest{},
		results.TryBotResponse{},
	)

	// TODO(jcgregorio) Switch to generator.AddMultipleUnionToNamespace().
	addMultipleUnions(generator, []unionAndName{
		{alerts.AllConfigState, "ConfigState"},
		{alerts.AllDirections, "Direction"},
		{frame.AllRequestType, "RequestType"},
		{frontend.AllRegressionSubset, "Subset"},
		{regression.AllProcessState, "ProcessState"},
		{regression.AllStatus, "Status"},
		{stepfit.AllStepFitStatus, "StepFitStatus"},
		{types.AllClusterAlgos, "ClusterAlgo"},
		{types.AllStepDetections, "StepDetection"},
		{results.AllRequestKind, "TryBotRequestKind"},
	})

	generator.AddUnionToNamespace(progress.AllStatus, "progress")
	generator.AddToNamespace(progress.SerializedProgress{}, "progress")

	generator.AddToNamespace(format.Format{}, "ingest")

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
