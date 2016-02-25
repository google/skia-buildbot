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
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

const (
	SKIA_REPO  = "https://skia.googlesource.com/skia.git"
	INFRA_REPO = "https://skia.googlesource.com/buildbot.git"

	BUILDSLAVES_CONNECTED_MEASUREMENT = "buildbot.buildslaves.connected"
)

// flags
var (
	workdir  = flag.String("workdir", ".", "Working directory used by data processors.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	grpcPort = flag.String("grpc_port", ":8000", "Port on which to run the buildbot data gRPC server.")
	httpPort = flag.String("http_port", ":8001", "Port on which to run the HTTP server.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")

	// Regexp matching non-alphanumeric characters.
	re = regexp.MustCompile("[^A-Za-z0-9]+")

	BUILDSLAVE_OFFLINE_BLACKLIST = []string{
		"build3-a3",
		"build4-a3",
		"vm255-m3",
	}
)

// fixName transforms names of builders/buildsteps into strings useable by
// InfluxDB.
func fixName(s string) string {
	return re.ReplaceAllString(s, "_")
}

func main() {
	defer common.LogPanic()

	// Global init to initialize glog and parse arguments.
	common.InitWithMetrics2("datahopper", influxHost, influxUser, influxPassword, influxDatabase, local)

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
	if _, err := buildbot.RunBuildServer(*grpcPort, db); err != nil {
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

	// Average build and step duration, failure rate.
	go func() {
		start := time.Now().Add(-1 * time.Hour)
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
					glog.Warningf("Failed to parse builder name %q: %s", b.Builder, err)
					builderNameParts = map[string]string{}
				}
				for k, v := range builderNameParts {
					tags[k] = v
				}
				// Report build duration.
				d := b.Finished.Sub(b.Started)
				metrics2.RawAddInt64PointAtTime("buildbot.builds.duration", tags, int64(d), b.Finished)

				// Report build failure rate.
				failureStatus := 0
				if b.Results != buildbot.BUILDBOT_SUCCESS {
					failureStatus = 1
				}
				metrics2.RawAddInt64PointAtTime("buildbot.builds.failure-status", tags, int64(failureStatus), b.Finished)

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
					// Report step duration.
					metrics2.RawAddInt64PointAtTime("buildbot.buildsteps.duration", stepTags, int64(d), s.Finished)

					// Report step failure rate.
					stepFailStatus := 0
					if s.Results != buildbot.BUILDBOT_SUCCESS {
						stepFailStatus = 1
					}
					metrics2.RawAddInt64PointAtTime("buildbot.buildsteps.failure-status", stepTags, int64(stepFailStatus), s.Finished)
				}
			}
			start = end
		}
	}()

	// Offline buildslaves.
	go func() {
		for _ = range time.Tick(time.Minute) {
			glog.Info("Loading buildslave data.")
			slaves, err := buildbot.GetBuildSlaves()
			if err != nil {
				glog.Error(err)
				continue
			}
			for masterName, m := range slaves {
				for _, s := range m {
					if util.In(s.Name, BUILDSLAVE_OFFLINE_BLACKLIST) {
						continue
					}
					v := int64(0)
					if s.Connected {
						v = int64(1)
					}
					metrics2.GetInt64Metric(BUILDSLAVES_CONNECTED_MEASUREMENT, map[string]string{
						"buildslave": s.Name,
						"master":     masterName,
					}).Update(v)
				}
			}
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

	// Time since last successful backup.
	go func() {
		lv := metrics2.NewLiveness("last-buildbot-db-backup", nil)
		authClient, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_ONLY)
		if err != nil {
			glog.Fatal(err)
		}
		gsClient, err := storage.NewClient(context.Background(), cloud.WithBaseHTTP(authClient))
		if err != nil {
			glog.Fatal(err)
		}
		setLastBackupTime := func() error {
			last := time.Time{}
			if err := gs.AllFilesInDir(gsClient, "skia-buildbots", "db_backup", func(item *storage.ObjectAttrs) {
				if item.Updated.After(last) {
					last = item.Updated
				}
			}); err != nil {
				return err
			}
			lv.ManualReset(last)
			glog.Infof("Last DB backup was %s.", last)
			return nil
		}
		if err := setLastBackupTime(); err != nil {
			glog.Fatal(err)
		}
		for _ = range time.Tick(10 * time.Minute) {
			if err := setLastBackupTime(); err != nil {
				glog.Errorf("Failed to get last DB backup time: %s", err)
			}
		}
	}()

	// Run a backup server.
	go func() {
		glog.Fatal(buildbot.RunBackupServer(db, *httpPort))
	}()

	// Wait while the above goroutines generate data.
	select {}
}
