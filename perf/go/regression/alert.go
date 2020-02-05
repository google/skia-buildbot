package regression

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
)

// RegressionsForAlert looks for regressions to the given alert over the last
// 'numContinuous' commits with data and periodically calls
// clusterResponseProcessor with the results of checking each commit.
func RegressionsForAlert(ctx context.Context, cfg *alerts.Alert, ps paramtools.ParamSet, clusterResponseProcessor ClusterResponseProcessor, numContinuous int, end time.Time, vcs vcsinfo.VCS, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder, stepProvider StepProvider) {
	queriesCounter := metrics2.GetCounter("perf_clustering_queries", nil)
	sklog.Infof("About to cluster for: %#v", *cfg)

	// This set of queries is restricted by the incoming set of trace ids, if
	// that's the kind of loop we're doing, by restricting 'ps' to just the
	// trace ids.
	queries, err := cfg.QueriesFromParamset(ps)
	if err != nil {
		sklog.Errorf("Failed to build GroupBy combinations: %s", err)
		return
	}
	sklog.Infof("Config expanded into %d queries.", len(queries))
	for step, q := range queries {
		if stepProvider != nil {
			stepProvider(step, len(queries))
		}
		sklog.Infof("Clustering for query: %q", q)

		// Create ClusterRequest and run.
		req := &ClusterRequest{
			Radius:        cfg.Radius,
			Query:         q,
			Algo:          cfg.Algo,
			StepDetection: cfg.Step,
			Interesting:   cfg.Interesting,
			K:             cfg.K,
			TZ:            "UTC",
			Sparse:        cfg.Sparse,
			Type:          CLUSTERING_REQUEST_TYPE_LAST_N,
			N:             int32(numContinuous),
			End:           end,
		}
		_, err := Run(ctx, req, vcs, cidl, dfBuilder, clusterResponseProcessor)
		if err != nil {
			sklog.Warningf("Failed while clustering %v %s", *req, err)
			continue
		}
		queriesCounter.Inc(1)
	}
	sklog.Infof("Finished clustering for: %#v", *cfg)
}
