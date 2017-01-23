/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const (
	MEASUREMENT_SWARM_BOTS_LAST_SEEN   = "swarming.bots.last-seen"
	MEASUREMENT_SWARM_BOTS_QUARANTINED = "swarming.bots.quarantined"
	MEASUREMENT_SWARM_TASKS_RUNNING    = "swarming.tasks.running"
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

	// Absolutify the workdir.
	w, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(w)
	}
	sklog.Infof("Workdir is %s", w)

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(w, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}

	// Swarming API client.
	swarm, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	// Shared repo objects.
	reposDir := path.Join(w, "repos")
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		sklog.Fatal(err)
	}
	repos, err := repograph.NewMap([]string{common.REPO_SKIA, common.REPO_SKIA_INFRA}, reposDir)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := repos.Update(); err != nil {
		sklog.Fatal(err)
	}

	// Data generation goroutines.
	db, err := buildbot.NewLocalDB(path.Join(w, "buildbot.db"))
	if err != nil {
		sklog.Fatal(err)
	}

	// Run a server for the buildbot data.
	if _, err := buildbot.RunBuildServer(*grpcPort, db); err != nil {
		sklog.Fatal(err)
	}

	// Swarming bots.
	go func() {
		oldMetrics := []metrics2.Int64Metric{}
		for _ = range time.Tick(2 * time.Minute) {
			sklog.Info("Loading Skia Swarming bot data.")
			skiaBots, err := swarm.ListSkiaBots()
			if err != nil {
				sklog.Error(err)
				continue
			}
			sklog.Info("Loading CT Swarming bot data.")
			ctBots, err := swarm.ListCTBots()
			if err != nil {
				sklog.Error(err)
				continue
			}
			bots := append(skiaBots, ctBots...)

			// Delete old metrics, replace with new ones. This fixes the case where
			// bots are removed but their metrics hang around, or where dimensions
			// change resulting in duplicate metrics with the same bot ID.
			failedDelete := []metrics2.Int64Metric{}
			for _, m := range oldMetrics {
				if err := m.Delete(); err != nil {
					sklog.Warningf("Failed to delete metric: %s", err)
					failedDelete = append(failedDelete, m)
				}
			}
			oldMetrics = append([]metrics2.Int64Metric{}, failedDelete...)

			now := time.Now()
			for _, bot := range bots {
				last, err := time.Parse("2006-01-02T15:04:05", bot.LastSeenTs)
				if err != nil {
					sklog.Error(err)
					continue
				}

				tags := map[string]string{
					"bot": bot.BotId,
				}
				for _, d := range bot.Dimensions {
					tags[fmt.Sprintf("dimension-%s", d.Key)] = strings.Join(d.Value, ",")
				}

				// Bot last seen <duration> ago.
				m1 := metrics2.GetInt64Metric(MEASUREMENT_SWARM_BOTS_LAST_SEEN, tags)
				m1.Update(int64(now.Sub(last)))
				oldMetrics = append(oldMetrics, m1)

				// Bot quarantined status.
				quarantined := int64(0)
				if bot.Quarantined {
					quarantined = int64(1)
				}
				m2 := metrics2.GetInt64Metric(MEASUREMENT_SWARM_BOTS_QUARANTINED, tags)
				m2.Update(quarantined)
				oldMetrics = append(oldMetrics, m2)
			}
		}
	}()

	// Swarming tasks.
	go func() {
		// Initial query: load data from the past 2 hours.
		lastLoad := time.Now().Add(-2 * time.Hour)
		for _ = range time.Tick(2 * time.Minute) {
			now := time.Now()
			tasks, err := swarm.ListSkiaTasks(lastLoad, now)
			if err != nil {
				sklog.Error(err)
				continue
			}
			lastLoad = now

			// Count the number of tasks in each state.
			counts := map[string]int64{}
			for _, t := range tasks {
				counts[t.TaskResult.State] += 1
			}

			// Report the number of running tasks as a metric.
			for state, count := range counts {
				skiaGauge := metrics2.GetInt64Metric(MEASUREMENT_SWARM_TASKS_RUNNING, map[string]string{"pool": "skia", "state": state})
				skiaGauge.Update(count)
			}
		}
	}()

	// Number of commits in the repo.
	go func() {
		skiaGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "skia"})
		infraGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "infra"})
		for _ = range time.Tick(5 * time.Minute) {
			nSkia, err := repos[common.REPO_SKIA].Repo().NumCommits()
			if err != nil {
				sklog.Errorf("Failed to get number of commits for Skia: %s", err)
			} else {
				skiaGauge.Update(nSkia)
			}
			nInfra, err := repos[common.REPO_SKIA_INFRA].Repo().NumCommits()
			if err != nil {
				sklog.Errorf("Failed to get number of commits for Infra: %s", err)
			} else {
				infraGauge.Update(nInfra)
			}
		}
	}()

	// Time since last successful backup.
	go func() {
		lv := metrics2.NewLiveness("last-buildbot-db-backup", nil)
		authClient, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_ONLY)
		if err != nil {
			sklog.Fatal(err)
		}
		gsClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(authClient))
		if err != nil {
			sklog.Fatal(err)
		}
		setLastBackupTime := func() error {
			last := time.Time{}
			if err := gcs.AllFilesInDir(gsClient, "skia-buildbots", "db_backup", func(item *storage.ObjectAttrs) {
				if item.Updated.After(last) {
					last = item.Updated
				}
			}); err != nil {
				return err
			}
			lv.ManualReset(last)
			sklog.Infof("Last DB backup was %s.", last)
			return nil
		}
		if err := setLastBackupTime(); err != nil {
			sklog.Fatal(err)
		}
		for _ = range time.Tick(10 * time.Minute) {
			if err := setLastBackupTime(); err != nil {
				sklog.Errorf("Failed to get last DB backup time: %s", err)
			}
		}
	}()

	// Run a backup server.
	go func() {
		sklog.Fatal(buildbot.RunBackupServer(db, *httpPort))
	}()

	// Wait while the above goroutines generate data.
	select {}
}
