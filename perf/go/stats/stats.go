package stats

import (
	"time"

	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/types"

	"github.com/golang/glog"
	metrics "github.com/rcrowley/go-metrics"
)

// Start calculating and reporting statistics on the repo and tiles.
//
// We presume the git.Update(true) is called somewhere else, usually this is done
// in the ingester, so the repo is always as good as the ingested tiles.
func Start(tileStore types.TileStore, git *gitinfo.GitInfo) {
	coverage := metrics.NewRegisteredGaugeFloat64("stats.tests.bench_runs_per_changelist", metrics.DefaultRegistry)
	skpLatency := metrics.NewRegisteredTimer("stats.skp.update_latency", metrics.DefaultRegistry)
	commits := metrics.NewRegisteredGauge("stats.commits.total", metrics.DefaultRegistry)

	go func() {
		for _ = range time.Tick(2 * time.Minute) {
			tile, err := tileStore.Get(0, -1)
			if err != nil {
				glog.Warning("Failed to get tile: %s", err)
				continue
			}
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

			last, err := git.LastSkpCommit()
			if err != nil {
				glog.Warning("Failed to read last SKP commit: %s", err)
				continue
			}
			skpLatency.Update(time.Since(last))
			commits.Update(int64(git.NumCommits()))
		}
	}()
}
