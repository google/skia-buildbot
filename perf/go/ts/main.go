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
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dryrun"
	"go.skia.org/infra/perf/go/frontend"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/continuous"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/types"
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

// IgnoreNil is a utility struct that allows specifying which structs should
// have go2ts:"ignorenil" applied.
type IgnoreNil struct {
	ParamSet paramtools.ParamSet `go2ts:"ignorenil"`
}

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddMultiple(generator,
		IgnoreNil{}, // Goes first to ensure the ignorenil version of structs are seen first.
		alerts.Alert{},
		alerts.AlertsStatus{},
		clustering2.ClusterSummary{},
		clustering2.ValuePercent{},
		dataframe.FrameRequest{},
		dataframe.FrameResponse{},
		dryrun.DryRunStatus{},
		dryrun.StartDryRunResponse{},
		frontend.AlertUpdateResponse{},
		frontend.ClusterStartResponse{},
		frontend.ClusterStatus{},
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
		continuous.Current{},
		regression.FullSummary{},
		regression.RegressionDetectionRequest{},
		regression.TriageStatus{},
		results.TryBotRequest{},
		results.TryBotResponse{},
	)

	// TODO(jcgregorio) Switch to generator.AddMultipleUnionToNamespace().
	addMultipleUnions(generator, []unionAndName{
		{alerts.AllConfigState, "ConfigState"},
		{alerts.AllDirections, "Direction"},
		{dataframe.AllRequestType, "RequestType"},
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

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
