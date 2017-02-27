/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/datahopper/go/accum"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"golang.org/x/net/context"
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
	grpcPort = flag.String("grpc_port", ":8000", "Port on which to run the buildbot data gRPC server.")
	httpPort = flag.String("http_port", ":8001", "Port on which to run the HTTP server.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workdir  = flag.String("workdir", ".", "Working directory used by data processors.")
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
	defer common.LogPanic()
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
					"bot":  bot.BotId,
					"pool": "Skia",
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
		taskAccum := accum.New(accum.DefaultReporter)
		// Initial query: load data from the past 2 minutes.
		lastLoad := time.Now().Add(-2 * time.Minute)

		revisitTasks := map[string]bool{}

		for _ = range time.Tick(2 * time.Minute) {
			now := time.Now()
			tasks, err := swarm.ListSkiaTasks(lastLoad, now)
			if err != nil {
				sklog.Error(err)
				continue
			}

			// Count the number of tasks in each state.
			counts := map[string]int64{}
			for _, t := range tasks {
				counts[t.TaskResult.State] += 1
			}

			// Report the number of running tasks as a metric.
			for state, count := range counts {
				skiaGauge := metrics2.GetInt64Metric(MEASUREMENT_SWARM_TASKS_STATE, map[string]string{"pool": "Skia", "state": state})
				skiaGauge.Update(count)
			}

			for id, _ := range revisitTasks {
				task, err := swarm.GetTaskMetadata(id)
				if err != nil {
					sklog.Error(err)
					continue
				}
				tasks = append(tasks, task)
			}
			revisitTasks = map[string]bool{}
			lastLoad = now
			for _, task := range tasks {
				if task.TaskResult.State == "COMPLETED" {
					if task.TaskResult.DedupedFrom != "" {
						continue
					}

					// Get the created time for the task. We'll use that as the
					// timestamp for all data points related to it.
					createdTime, err := swarming.Created(task)
					if err != nil {
						sklog.Errorf("Failed to parse Swarming task created timestamp: %s", err)
						continue
					}

					// Find the tags for the task, including ID, name, dimensions,
					// and components of the builder name.
					var builderName string
					var name string
					user, err := swarming.GetTagValue(task.TaskResult, "user")
					if err != nil || user == "" {
						// This is an old-style task.
						name, err = swarming.GetTagValue(task.TaskResult, "name")
						if err != nil || name == "" {
							sklog.Errorf("Failed to find name for Swarming task: %v", task)
							continue
						}
						builderName, err = swarming.GetTagValue(task.TaskResult, "buildername")
						if err != nil || builderName == "" {
							sklog.Errorf("Failed to find buildername for Swarming task: %v", task)
							continue
						}
					} else if user == "skia-task-scheduler" {
						// This is a new-style task.
						builderName, err = swarming.GetTagValue(task.TaskResult, "sk_name")
						if err != nil || builderName == "" {
							sklog.Errorf("Failed to find sk_name for Swarming task: %v", task)
							continue
						}
						name = builderName
					}

					// Leave 'task-id' in 'tags' so that it gets reported to logs,
					// knowing that Accum will remove it from the metrics.
					tags := map[string]string{
						"bot-id":    task.TaskResult.BotId,
						"task-id":   task.TaskId,
						"task-name": name,
						"pool":      "Skia",
					}

					// Task duration in milliseconds.
					taskAccum.Add(MEASUREMENT_SWARM_TASKS_DURATION, tags, int64(task.TaskResult.Duration*float64(1000.0)))

					if task.TaskResult.PerformanceStats != nil {
						// Overhead stats, in milliseconds.
						taskAccum.Add(MEASUREMENT_SWARM_TASKS_OVERHEAD_BOT, tags, int64(task.TaskResult.PerformanceStats.BotOverhead*float64(1000.0)))
						if task.TaskResult.PerformanceStats.IsolatedDownload != nil {
							taskAccum.Add(MEASUREMENT_SWARM_TASKS_OVERHEAD_DOWNLOAD, tags, int64(task.TaskResult.PerformanceStats.IsolatedDownload.Duration*float64(1000.0)))
						} else {
							sklog.Errorf("Swarming task is missing its IsolatedDownload section: %v", task.TaskResult)
						}
						if task.TaskResult.PerformanceStats.IsolatedUpload != nil {
							taskAccum.Add(MEASUREMENT_SWARM_TASKS_OVERHEAD_UPLOAD, tags, int64(task.TaskResult.PerformanceStats.IsolatedUpload.Duration*float64(1000.0)))
						} else {
							sklog.Errorf("Swarming task is missing its IsolatedUpload section: %v", task.TaskResult)
						}
					}

					// Pending time in milliseconds.
					startTime, err := swarming.Started(task)
					if err != nil {
						sklog.Errorf("Failed to parse Swarming task started timestamp: %s", err)
						continue
					}
					pendingMs := int64(startTime.Sub(createdTime).Seconds() * float64(1000.0))
					taskAccum.Add(MEASUREMENT_SWARM_TASKS_PENDING_TIME, tags, pendingMs)
				} else {
					revisitTasks[task.TaskId] = true
				}
			}
			taskAccum.Report()
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
