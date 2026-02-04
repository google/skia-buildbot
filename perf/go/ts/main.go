// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate bazelisk run --config=mayberemote //:go -- run . -o ../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"go.skia.org/infra/go/go2ts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dryrun"
	"go.skia.org/infra/perf/go/frontend"
	frontendApi "go.skia.org/infra/perf/go/frontend/api"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/pinpoint"
	"go.skia.org/infra/perf/go/pivot"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	subProto "go.skia.org/infra/perf/go/subscription/proto/v1"
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
	generator.GenerateNominalTypes = true
	generator.AddIgnoreNil(paramtools.Params{})
	generator.AddIgnoreNil(paramtools.ParamSet{})
	generator.AddIgnoreNil(paramtools.ReadOnlyParamSet{})
	generator.AddIgnoreNil(types.TraceSet{})

	generator.AddUnionToNamespace(pivot.AllOperations, "pivot")
	generator.AddToNamespace(pivot.Request{}, "pivot")

	generator.AddMultiple(generator,
		alerts.Alert{},
		alerts.AlertsStatus{},
		chromeperf.RevisionInfo{},
		clustering2.ClusterSummary{},
		clustering2.ValuePercent{},
		config.Favorites{},
		config.QueryConfig{},
		dryrun.RegressionAtCommit{},
		frame.FrameRequest{},
		frame.FrameResponse{},
		frontendApi.AlertUpdateResponse{},
		frontendApi.CIDHandlerResponse{},
		frontendApi.ClusterStartResponse{},
		frontendApi.CommitDetailsRequest{},
		frontendApi.CountHandlerRequest{},
		frontendApi.CountHandlerResponse{},
		frontendApi.GetAnomaliesResponse{},
		frontendApi.GetGroupReportResponse{},
		frontendApi.GetUserIssuesForTraceKeysRequest{},
		frontendApi.GetUserIssuesForTraceKeysResponse{},
		frontendApi.GetGraphsShortcutRequest{},
		frontendApi.GetSheriffListResponse{},
		frontendApi.NextParamListHandlerRequest{},
		frontendApi.NextParamListHandlerResponse{},
		frontendApi.RangeRequest{},
		frontendApi.RegressionRangeRequest{},
		frontendApi.RegressionRangeResponse{},
		frontendApi.ShiftRequest{},
		frontendApi.ShiftResponse{},
		frontend.SkPerfConfig{},
		frontendApi.TriageRequest{},
		frontendApi.TriageResponse{},
		frontendApi.TryBugRequest{},
		frontendApi.TryBugResponse{},
		frontendApi.ListIssuesResponse{},
		graphsshortcut.GraphsShortcut{},
		pinpoint.CreateBisectRequest{},
		pinpoint.CreateLegacyTryRequest{},
		pinpoint.CreatePinpointResponse{},
		provider.Commit{},
		regression.Regression{},
		regression.FullSummary{},
		regression.RegressionDetectionRequest{},
		regression.RegressionDetectionResponse{},
		regression.TriageStatus{},
		subProto.Subscription{},
		types.TraceMetadata{},
	)

	// TODO(jcgregorio) Switch to generator.AddMultipleUnionToNamespace().
	addMultipleUnions(generator, []unionAndName{
		{alerts.AllConfigState, "ConfigState"},
		{alerts.AllDirections, "Direction"},
		{frame.AllRequestType, "RequestType"},
		{frontendApi.AllRegressionSubset, "Subset"},
		{regression.AllProcessState, "ProcessState"},
		{regression.AllStatus, "Status"},
		{stepfit.AllStepFitStatus, "StepFitStatus"},
		{types.AllClusterAlgos, "ClusterAlgo"},
		{types.AllStepDetections, "StepDetection"},
		{frame.AllResponseDisplayModes, "FrameResponseDisplayMode"},
		{notifytypes.AllNotifierTypes, "NotifierTypes"},
		{config.AllTraceFormats, "TraceFormat"},
		{types.AllAlertActions, "AlertAction"},
		{types.AllProjectIds, "ProjectId"},
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
