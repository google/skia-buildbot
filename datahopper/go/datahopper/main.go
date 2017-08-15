/*
	Pulls data from multiple sources and funnels into metrics.
*/

package main

import (
	"context"
	"flag"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/datahopper/go/bot_metrics"
	"go.skia.org/infra/datahopper/go/swarming_metrics"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/perf/go/perfclient"
	"google.golang.org/api/option"
)

const (
	MEASUREMENT_SWARM_BOTS_LAST_SEEN          = "swarming.bots.last-seen"
	MEASUREMENT_SWARM_BOTS_QUARANTINED        = "swarming.bots.quarantined"
	MEASUREMENT_SWARM_TASKS_STATE             = "swarming.tasks.state"
	MEASUREMENT_SWARM_TASKS_DURATION          = "swarming.tasks.duration"
	MEASUREMENT_SWARM_TASKS_OVERHEAD_BOT      = "swarming.tasks.overhead.bot"
	MEASUREMENT_SWARM_TASKS_OVERHEAD_DOWNLOAD = "swarming.tasks.overhead.download"
	MEASUREMENT_SWARM_TASKS_OVERHEAD_UPLOAD   = "swarming.tasks.overhead.upload"
	MEASUREMENT_SWARM_TASKS_PENDING_TIME      = "swarming.tasks.pending-time"
)

// flags
var (
	grpcPort           = flag.String("grpc_port", ":8000", "Port on which to run the buildbot data gRPC server.")
	httpPort           = flag.String("http_port", ":8001", "Port on which to run the HTTP server.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory used by data processors.")

	perfBucket = flag.String("perf_bucket", "skia-perf", "The GCS bucket that should be used for writing into perf")
	perfPrefix = flag.String("perf_duration_prefix", "task-duration", "The folder name in the bucket that task duration metric shoudl be written.")
)

var (
	// Regexp matching non-alphanumeric characters.
	re = regexp.MustCompile("[^A-Za-z0-9]+")

	BUILDSLAVE_OFFLINE_BLACKLIST = []string{
		"build3-a3",
		"build4-a3",
		"vm255-m3",
	}
)

func main() {
	common.InitWithMust(
		"datahopper",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

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
	swarmInternal, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		sklog.Fatal(err)
	}

	authClient, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatal(err)
	}

	gsClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(authClient))
	if err != nil {
		sklog.Fatal(err)
	}
	storageClient := gcs.NewGCSClient(gsClient, *perfBucket)
	pc := perfclient.New(*perfPrefix, storageClient)

	tnp := taskname.DefaultTaskNameParser()

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
	swarmingClients := map[string]swarming.ApiClient{
		swarming.SWARMING_SERVER:         swarm,
		swarming.SWARMING_SERVER_PRIVATE: swarmInternal,
	}
	swarmingPools := map[string][]string{
		swarming.SWARMING_SERVER:         swarming.POOLS_PUBLIC,
		swarming.SWARMING_SERVER_PRIVATE: swarming.POOLS_PRIVATE,
	}
	for swarmingServer, client := range swarmingClients {
		for _, pool := range swarmingPools[swarmingServer] {
			go func(server, pool string, client swarming.ApiClient) {
				oldMetrics := []metrics2.Int64Metric{}
				for range time.Tick(2 * time.Minute) {
					sklog.Infof("Loading Swarming bot data for pool %s", pool)
					bots, err := client.ListBotsForPool(pool)
					if err != nil {
						sklog.Error(err)
						continue
					}

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
							"bot":      bot.BotId,
							"pool":     pool,
							"swarming": server,
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
			}(swarmingServer, pool, client)
		}
	}

	// Swarming tasks.
	if err := swarming_metrics.StartSwarmingTaskMetrics(w, swarm, context.Background(), pc, tnp); err != nil {
		sklog.Fatal(err)
	}

	// Number of commits in the repo.
	go func() {
		skiaGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "skia"})
		infraGauge := metrics2.GetInt64Metric("repo.commits", map[string]string{"repo": "infra"})
		for range time.Tick(5 * time.Minute) {
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
		for range time.Tick(10 * time.Minute) {
			if err := setLastBackupTime(); err != nil {
				sklog.Errorf("Failed to get last DB backup time: %s", err)
			}
		}
	}()

	// Jobs metrics.
	if err := StartJobMetrics(*taskSchedulerDbUrl, context.Background()); err != nil {
		sklog.Fatal(err)
	}

	// Generate "time to X% bot coverage" metrics.
	if err := bot_metrics.Start(*taskSchedulerDbUrl, *workdir, nil, context.Background()); err != nil {
		sklog.Fatal(err)
	}

	// Run a backup server.
	go func() {
		sklog.Fatal(buildbot.RunBackupServer(db, *httpPort))
	}()

	// Wait while the above goroutines generate data.
	select {}
}
