/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"fmt"
	"path"
	"regexp"
	"time"

	go_metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics"
)

const (
	SKIA_REPO  = "https://skia.googlesource.com/skia.git"
	INFRA_REPO = "https://skia.googlesource.com/buildbot.git"
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	workdir        = flag.String("workdir", ".", "Working directory used by data processors.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	grpcPort       = flag.String("grpc_port", ":8000", "Port on which to run the buildbot data gRPC server.")
	httpPort       = flag.String("http_port", ":8001", "Port on which to run the HTTP server.")

	// Regexp matching non-alphanumeric characters.
	re = regexp.MustCompile("[^A-Za-z0-9]+")
)

// fixName transforms names of builders/buildsteps into strings useable by
// InfluxDB.
func fixName(s string) string {
	return re.ReplaceAllString(s, "_")
}

func main() {
	defer common.LogPanic()

	// Global init to initialize glog and parse arguments.
	common.InitWithMetrics("datahopper", graphiteServer)

	// Shared repo objects.
	skiaRepo, err := gitinfo.CloneOrUpdate(SKIA_REPO, path.Join(*workdir, "datahopper_skia"), true)
	if err != nil {
		glog.Fatal(err)
	}
	infraRepo, err := gitinfo.CloneOrUpdate(INFRA_REPO, path.Join(*workdir, "datahopper_infra"), true)
	if err != nil {
		glog.Fatal(err)
	}
	go func() {
		for _ = range time.Tick(5 * time.Minute) {
			if err := skiaRepo.Update(true, true); err != nil {
				glog.Errorf("Failed to sync Skia repo: %v", err)
			}
			if err := infraRepo.Update(true, true); err != nil {
				glog.Errorf("Failed to sync Infra repo: %v", err)
			}
		}
	}()

	// Data generation goroutines.
	db, err := buildbot.NewLocalDB(path.Join(*workdir, "buildbot.db"))
	if err != nil {
		glog.Fatal(err)
	}

	// Buildbot data ingestion.
	if err := buildbot.IngestNewBuildsLoop(db, *workdir); err != nil {
		glog.Fatal(err)
	}

	// Run a server for the buildbot data.
	if err := buildbot.RunBuildServer(*grpcPort, db); err != nil {
		glog.Fatal(err)
	}

	// Measure buildbot data ingestion progress.
	totalGuage := go_metrics.GetOrRegisterGauge("buildbot.builds.total", go_metrics.DefaultRegistry)
	ingestGuage := go_metrics.GetOrRegisterGauge("buildbot.builds.ingested", go_metrics.DefaultRegistry)
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			totalBuilds, err := buildbot.NumTotalBuilds()
			if err != nil {
				glog.Error(err)
				continue
			}
			ingestedBuilds, err := db.NumIngestedBuilds()
			if err != nil {
				glog.Error(err)
				continue
			}
			totalGuage.Update(int64(totalBuilds))
			ingestGuage.Update(int64(ingestedBuilds))
		}
	}()

	// Average build and step time.
	go func() {
		period := 24 * time.Hour
		for _ = range time.Tick(10 * time.Minute) {
			glog.Info("Loading build and buildstep duration data.")
			end := time.Now().UTC()
			start := end.Add(-period)
			builds, err := db.GetBuildsFromDateRange(start, end)
			if err != nil {
				glog.Errorf("Failed to obtain build and buildstep duration data: %s", err)
				continue
			}
			for _, b := range builds {
				if !b.IsFinished() {
					continue
				}
				// Report build time.
				// app.host.measurement.measurement.builder.measurement*
				d := b.Finished.Sub(b.Started)
				metric := fmt.Sprintf("buildbot.builds.%s.duration", fixName(b.Builder))
				metrics.GetOrRegisterSlidingWindow(metric, metrics.DEFAULT_WINDOW).Update(int64(d))
				for _, s := range b.Steps {
					if !s.IsFinished() {
						continue
					}
					// app.host.measurement.measurement.builder.step.measurement*
					d := s.Finished.Sub(s.Started)
					metric := fmt.Sprintf("buildbot.buildstepsbybuilder.%s.%s.duration", fixName(b.Builder), fixName(s.Name))
					metrics.GetOrRegisterSlidingWindow(metric, metrics.DEFAULT_WINDOW).Update(int64(d))
				}
			}
		}
	}()

	// Number of commits in the repo.
	go func() {
		skiaGauge := go_metrics.GetOrRegisterGauge("repo.skia.commits", go_metrics.DefaultRegistry)
		infraGauge := go_metrics.GetOrRegisterGauge("repo.infra.commits", go_metrics.DefaultRegistry)
		for _ = range time.Tick(5 * time.Minute) {
			skiaGauge.Update(int64(skiaRepo.NumCommits()))
			infraGauge.Update(int64(infraRepo.NumCommits()))
		}
	}()

	// Run a backup server.
	go func() {
		glog.Fatal(buildbot.RunBackupServer(db, *httpPort))
	}()

	// Wait while the above goroutines generate data.
	select {}
}
