package perfresults

import (
	"context"
	"net/http"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
)

type rbeProvider func(ctx context.Context, casInstance string) (*RBEPerfLoader, error)

type loader struct {
	client      *http.Client
	rbeProvider rbeProvider
}

// NewLoader returns a new loader to load perf_results from the buildbucket.
//
// This is a simple lightweight struct used to inject dependencies and for testing purposes.
// One can simply call: NewLoader().LoadPerfResults(...).
func NewLoader() loader {
	return loader{
		rbeProvider: NewRBEPerfLoader,
	}
}

func getNonNilCAS(cases ...*apipb.CASReference) []*apipb.CASReference {
	nonnil := make([]*apipb.CASReference, len(cases))
	ct := 0
	for _, c := range cases {
		if c == nil {
			continue
		}
		nonnil[ct] = c
		ct++
	}
	return nonnil[:ct]
}

// checkCasInstances returns true if all the cas instances are from the same rbe instance.
func checkCasInstances(cases ...*apipb.CASReference) bool {
	// empty slice is considered to be true.
	if len(cases) == 0 {
		return true
	}
	first := cases[0]
	for _, c := range cases[1:] {
		if first.GetCasInstance() != c.GetCasInstance() {
			return false
		}
	}
	return true
}

// LoadPerfResults loads and merges all the perf_results.json from the given buildID.
func (loader loader) LoadPerfResults(ctx context.Context, buildID int64) (BuildInfo, map[string]*PerfResults, error) {
	bc, _ := newBuildsClient(ctx, loader.client)
	bi, err := bc.findBuildInfo(ctx, buildID)
	if err != nil {
		return BuildInfo{}, nil, skerr.Wrap(err)
	}

	sc, err := newSwarmingClient(ctx, bi.SwarmingInstance, loader.client)
	if err != nil {
		return BuildInfo{}, nil, skerr.Wrap(err)
	}

	childIDs, err := sc.findChildTaskIds(ctx, bi.TaskID)
	if err != nil {
		return BuildInfo{}, nil, skerr.Wrap(err)
	}

	cases, err := sc.findTaskCASOutputs(ctx, childIDs...)
	if err != nil {
		return BuildInfo{}, nil, skerr.Wrap(err)
	}

	cases = getNonNilCAS(cases...)

	// If there is no test runs, we return the build info only.
	if len(cases) == 0 {
		return bi, nil, nil
	}

	if !checkCasInstances(cases...) {
		return BuildInfo{}, nil, skerr.Fmt("CAS outputs are not from the same instance (%v)", buildID)
	}

	rbe, err := loader.rbeProvider(ctx, cases[0].GetCasInstance())
	if err != nil {
		return BuildInfo{}, nil, skerr.Wrap(err)
	}

	pr, err := rbe.LoadPerfResults(ctx, cases...)
	return bi, pr, err
}
