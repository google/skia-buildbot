package regression

import (
	"context"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
)

// RegressionsForAlert looks for regressions to the given alert over the last
// 'numContinuous' commits with data and periodically calls
// clusterResponseProcessor with the results of checking each commit.
func RegressionsForAlert(ctx context.Context, cfg *alerts.Config, ps paramtools.ParamSet, clusterResponseProcessor ClusterResponseProcessor, numContinuous int, end time.Time, git *gitinfo.GitInfo, cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder) {
	sklog.Infof("About to cluster for: %#v", *cfg)
	queries, err := cfg.QueriesFromParamset(ps)
	if err != nil {
		sklog.Errorf("Failed to build GroupBy combinations: %s", err)
		return
	}
	for _, q := range queries {
		sklog.Infof("Clustering for query: %q", q)

		// Create ClusterRequest and run.
		req := &ClusterRequest{
			Radius:      cfg.Radius,
			Query:       q,
			Algo:        cfg.Algo,
			Interesting: cfg.Interesting,
			K:           cfg.K,
			TZ:          "UTC",
			Sparse:      cfg.Sparse,
			Type:        CLUSTERING_REQUEST_TYPE_LAST_N,
			N:           int32(numContinuous),
			End:         end,
		}
		_, err := Run(ctx, req, git, cidl, dfBuilder, clusterResponseProcessor)
		if err != nil {
			sklog.Warningf("Failed while clustering %v %s", *req, err)
			continue
		}
	}
}
