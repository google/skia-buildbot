package stats

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/metrics2"
	tracedb "go.skia.org/infra/go/trace/db"
)

// Start calculating and reporting statistics on the repo and tiles.
//
// We presume the git.Update(true) is called somewhere else, usually this is done
// in the trace/db.Builder, so the repo is always as good as the loaded tiles.
func Start(tileBuilder tracedb.MasterTileBuilder, git *gitinfo.GitInfo) {
	coverage := metrics2.GetFloat64Metric("perf.coverage", nil)
	liveness := metrics2.NewLiveness("perf.coverage", nil)
	latency := metrics2.NewTimer("perf.stats.latency", nil)
	commits := metrics2.GetInt64Metric("perf.commits", nil)

	go func() {
		for _ = range time.Tick(2 * time.Minute) {
			latency.Start()
			tile := tileBuilder.GetTile()
			numCommits := tile.LastCommitIndex() + 1
			numTraces := len(tile.Traces)
			total := 0
			for _, tr := range tile.Traces {
				for i := 0; i < numCommits; i++ {
					if !tr.IsMissing(i) {
						total += 1
					}
				}
			}
			cov := float64(total) / float64(numCommits*numTraces)
			glog.Info("Coverage: ", cov)
			coverage.Update(cov)

			commits.Update(int64(git.NumCommits()))
			latency.Stop()
			liveness.Reset()
		}
	}()
}
