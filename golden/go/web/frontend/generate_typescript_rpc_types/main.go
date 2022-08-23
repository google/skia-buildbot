//go:generate go run . -o ../../../../modules/rpc_types.ts

package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/web/frontend"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	addTypes(generator)

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}

func addTypes(generator *go2ts.Go2TS) {
	generator.AddIgnoreNil(paramtools.ParamSet{})

	// Response for the /json/v1/changelist/{system}/{id} RPC endpoint.
	generator.AddWithName(frontend.ChangelistSummary{}, "ChangelistSummaryResponse")

	// Response for the /json/v1/paramset RPC endpoint.
	generator.AddWithName(paramtools.ReadOnlyParamSet{}, "ParamSetResponse")

	// Response for the /json/v1/search RPC endpoint.
	//
	// We add frontend.SearchResult first because we want to recursively preserve its nil types. If
	// we don't add frontend.SearchResult explicitly, it will be discovered by go2ts as a field in
	// frontend.SearchResponse tagged with `go2ts:"ignorenil"`, which recursively ignores all nils.
	//
	// The frontend.SearchResponse struct has a frontend.TriageRequestDataV2 field. We want to add
	// frontend.TriageRequestDataV2 without the "V2" suffix, so we must add
	// frontend.TriageRequestDataV2 before we add frontend.SearchResponse. This ensures that go2ts
	// won't discover frontend.TriageRequestDataV2 as a field in frontend.SearchResponse and add it
	// with the default type name ("TriageRequestDataV2").
	generator.Add(frontend.SearchResult{})
	generator.AddWithName(frontend.TriageRequestDataV2{}, "TriageRequestData")
	generator.AddWithName(frontend.SearchResponse{}, "SearchResponse")

	// Request for the /json/v2/triage RPC endpoint.
	generator.AddWithName(frontend.TriageRequestV2{}, "TriageRequest")

	// Request for the /json/v3/triage RPC endpoint.
	generator.AddWithName(frontend.TriageRequestV3{}, "TriageRequestV3")

	// Resoibse for the /json/v3/triage RPC endpoint.
	generator.Add(frontend.TriageResponse{})

	// Response for the /json/v1/trstatus RPC endpoint.
	generator.AddWithName(frontend.GUIStatus{}, "StatusResponse")

	// Response for the /json/v1/groupings RPC endpoint.
	generator.Add(frontend.GroupingsResponse{})

	// Response for the /json/v1/byblame RPC endpoint.
	generator.Add(frontend.ByBlameResponse{})

	// Response for the /json/v2/triagelog RPC endpoint.
	generator.Add(frontend.TriageLogResponse{})

	// Response for the /json/v1/changelists RPC endpoint.
	generator.Add(frontend.ChangelistsResponse{})

	// Payload for the /json/v1/ignores/add and /json/v1/ignores/save RPC endpoints.
	generator.Add(frontend.IgnoreRuleBody{})

	// Response for the /json/v1/ignores RPC endpoint.
	generator.Add(frontend.IgnoresResponse{})

	// Response for the /json/v1/list RPC endpoint.
	generator.Add(frontend.ListTestsResponse{})

	// Response for the /json/v1/diff RPC endpoint.
	generator.Add(frontend.DigestComparison{})

	// Response for the /json/v1/details RPC endpoint.
	generator.Add(frontend.DigestDetails{})

	// Response for the /json/v1/clusterdiff RPC endpoint.
	generator.AddWithName(frontend.Node{}, "ClusterDiffNode")
	generator.AddWithName(frontend.Link{}, "ClusterDiffLink")
	generator.Add(frontend.ClusterDiffResult{})

	generator.AddUnionWithName(expectations.AllLabel, "Label")
	generator.AddUnionWithName([]frontend.RefClosest{frontend.PositiveRef, frontend.NegativeRef, frontend.NoRef}, "RefClosest")
	generator.AddUnionWithName(frontend.AllTriageResponseStatus, "TriageResponseStatus")
	generator.AddUnionWithName(frontend.AllClosestDiffLabels, "ClosestDiffLabel")
}
