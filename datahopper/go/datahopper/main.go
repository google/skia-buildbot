/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics2"
)

const (
	SKIA_REPO  = "https://skia.googlesource.com/skia.git"
	INFRA_REPO = "https://skia.googlesource.com/buildbot.git"
)

// flags
var (
	workdir  = flag.String("workdir", ".", "Working directory used by data processors.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	grpcPort = flag.String("grpc_port", ":8000", "Port on which to run the buildbot data gRPC server.")
	httpPort = flag.String("http_port", ":8001", "Port on which to run the HTTP server.")

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
	common.InitWithMetrics2("datahopper")

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
	totalGauge := metrics2.GetInt64Metric("buildbot.builds.total", nil)
	ingestGuage := metrics2.GetInt64Metric("buildbot.builds.ingested", nil)
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
			totalGauge.Update(int64(totalBuilds))
			ingestGuage.Update(int64(ingestedBuilds))
		}
	}()

	// Average build and step time.
	go func() {
		start := time.Now().Add(-24 * time.Hour)
		for _ = range time.Tick(10 * time.Minute) {
			end := time.Now().UTC()
			glog.Info("Loading build and buildstep duration data from %s to %s", start, end)
			builds, err := db.GetBuildsFromDateRange(start, end)
			if err != nil {
				glog.Errorf("Failed to obtain build and buildstep duration data: %s", err)
				continue
			}
			for _, b := range builds {
				if !b.IsFinished() {
					continue
				}
				tags := map[string]string{
					"builder":    b.Builder,
					"buildslave": b.BuildSlave,
					"master":     b.Master,
					"number":     strconv.Itoa(b.Number),
				}
				builderNameParts, err := buildbot.ParseBuilderName(b.Builder)
				if err != nil {
					glog.Errorf("Failed to parse builder name %q: %s", b.Builder, err)
					continue
				}
				for k, v := range builderNameParts {
					tags[k] = v
				}
				// Report build time.
				d := b.Finished.Sub(b.Started)
				metrics2.RawAddInt64PointAtTime("buildbot.builds.duration", tags, int64(d), b.Finished)
				for _, s := range b.Steps {
					if !s.IsFinished() {
						continue
					}
					d := s.Finished.Sub(s.Started)
					stepTags := make(map[string]string, len(tags)+1)
					for k, v := range tags {
						stepTags[k] = v
					}
					stepTags["step"] = s.Name
					metrics2.RawAddInt64PointAtTime("buildbot.buildsteps.duration", stepTags, int64(d), s.Finished)
				}
			}
			start = end
		}
	}()

	// Number of commits in the repo.
	go func() {
		skiaGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "skia"})
		infraGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "infra"})
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
