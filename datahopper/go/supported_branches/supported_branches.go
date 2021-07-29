package supported_branches

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/supported_branches"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// This metric indicates whether or not a given branch has a valid
	// commit queue config; its value is 0 (false) or 1 (true).
	METRIC_BRANCH_EXISTS = "cq_cfg_branch_exists"

	// This metric indicates whether or not a given CQ tryjob for a
	// particular branch exists in tasks.json for that branch; its value
	// is 0 (false) or 1 (true).
	METRIC_TRYJOB_EXISTS = "cq_cfg_tryjob_exists"

	// This metric indicates whether or not bots exist which are able to run
	// a given CQ tryjob for a given branch. Its value is 0 (false) or 1
	// (true).
	METRIC_BOT_EXISTS = "cq_cfg_bot_exists_for_tryjob"
)

// botCanRunTask returns true iff a bot with the given dimensions is able to
// run a task with the given dimensions.
func botCanRunTask(botDims, taskDims map[string][]string) bool {
	for k, vals := range taskDims {
		for _, v := range vals {
			if !util.In(v, botDims[k]) {
				return false
			}
		}
	}
	return true
}

// metricsForRepo collects supported branch metrics for a single repo.
func metricsForRepo(repo *gitiles.Repo, newMetrics map[metrics2.Int64Metric]struct{}, botDimsList []map[string][]string) error {
	sbc, err := supported_branches.ReadConfigFromRepo(repo)
	if err != nil {
		if strings.Contains(err.Error(), "Not Found") || strings.Contains(err.Error(), "404") {
			sklog.Infof("Skipping repo %s; no supported branches file found.", repo.URL())
			return nil
		}
		return fmt.Errorf("Failed to get supported branches for %s: %s", repo.URL(), err)
	}
	for _, branch := range sbc.Branches {
		// Find the CQ trybots for this branch.
		cqJobsToCfg, err := cq.GetCQJobsToConfig(repo, branch.Ref)
		if err != nil {
			return skerr.Wrapf(err, "Failed to get CQ config for %s and %s", repo.URL(), branch.Ref)
		}
		branchExistsMetric := metrics2.GetInt64Metric(METRIC_BRANCH_EXISTS, map[string]string{
			"repo":   repo.URL(),
			"branch": branch.Ref,
		})
		branchExistsMetric.Update(1)
		newMetrics[branchExistsMetric] = struct{}{}
		cqTrybots := []string{}
		for builder := range cqJobsToCfg {
			name := builder
			split := strings.Split(name, "/")
			if len(split) != 3 {
				sklog.Errorf("Invalid builder name %q; skipping.", name)
				continue
			}
			// Skip non-Skia trybots.
			if !strings.Contains(split[0], "skia") && !strings.Contains(split[1], "skia") {
				continue
			}
			cqTrybots = append(cqTrybots, split[2])
		}

		// Obtain the tasks cfg for this branch.
		tasksContents, err := repo.ReadFileAtRef(context.Background(), specs.TASKS_CFG_FILE, branch.Ref)
		if err != nil {
			return fmt.Errorf("Failed to read %s on %s of %s: %s", specs.TASKS_CFG_FILE, branch.Ref, repo.URL(), err)
		}
		tasksCfg, err := specs.ParseTasksCfg(string(tasksContents))
		if err != nil {
			return fmt.Errorf("Failed to parse %s on %s of %s: %s", specs.TASKS_CFG_FILE, branch.Ref, repo.URL(), err)
		}

		// Determine whether each tryjob exists in the tasks cfg.
		for _, job := range cqTrybots {
			jobSpec, ok := tasksCfg.Jobs[job]
			jobExists := int64(0)
			if ok {
				jobExists = 1
			}
			jobExistsMetric := metrics2.GetInt64Metric(METRIC_TRYJOB_EXISTS, map[string]string{
				"repo":   repo.URL(),
				"branch": branch.Ref,
				"tryjob": job,
			})
			jobExistsMetric.Update(jobExists)
			newMetrics[jobExistsMetric] = struct{}{}

			// Determine whether bots exist for this tryjob.
			if ok {
				// First, find all tasks for the job.
				tasks := map[string]*specs.TaskSpec{}
				var add func(string)
				add = func(name string) {
					taskSpec := tasksCfg.Tasks[name]
					tasks[name] = taskSpec
					for _, dep := range taskSpec.Dependencies {
						add(dep)
					}
				}
				for _, task := range jobSpec.TaskSpecs {
					add(task)
				}

				// Now verify that there's at least one bot
				// which can run each task.
				botExists := int64(1)
				for taskName, taskSpec := range tasks {
					taskDims, err := swarming.ParseDimensions(taskSpec.Dimensions)
					if err != nil {
						return fmt.Errorf("Failed to parse dimensions for %s on %s; %s\ndims: %+v", taskName, branch.Ref, err, taskSpec.Dimensions)
					}
					canRunTask := false
					for _, botDims := range botDimsList {
						if botCanRunTask(botDims, taskDims) {
							canRunTask = true
							break
						}
					}
					if !canRunTask {
						botExists = 0
						sklog.Warningf("No bot can run %s on %s in %s", taskName, branch.Ref, repo.URL)
						break
					}
				}
				botExistsMetric := metrics2.GetInt64Metric(METRIC_BOT_EXISTS, map[string]string{
					"repo":   repo.URL(),
					"branch": branch.Ref,
					"tryjob": job,
				})
				botExistsMetric.Update(botExists)
				newMetrics[botExistsMetric] = struct{}{}
			}
		}
	}
	return nil
}

// Perform one iteration of supported branch metrics.
func cycle(repos []*gitiles.Repo, oldMetrics map[metrics2.Int64Metric]struct{}, swarm swarming.ApiClient, pools []string) (map[metrics2.Int64Metric]struct{}, error) {
	// Get all of the Swarming bots.
	bots := []*swarming_api.SwarmingRpcsBotInfo{}
	for _, pool := range pools {
		call := swarm.SwarmingService().Bots.List()
		// Copied from apiClient.ListFreeBots. We don't want to count dead or
		// quarantined bots, but it doesn't matter if they are busy or free at the
		// moment.
		call.Dimensions(fmt.Sprintf("%s:%s", swarming.DIMENSION_POOL_KEY, pool))
		call.IsDead("FALSE")
		call.Quarantined("FALSE")
		b, err := swarming.ProcessBotsListCall(call)
		if err != nil {
			return nil, err
		}
		bots = append(bots, b...)
	}

	// Collect all dimensions for all bots.
	// TODO(borenet): Can we exclude duplicates?
	botDimsList := make([]map[string][]string, 0, len(bots))
	for _, bot := range bots {
		botDimsList = append(botDimsList, swarming.BotDimensionsToStringMap(bot.Dimensions))
	}

	// Calculate metrics for each repo.
	newMetrics := map[metrics2.Int64Metric]struct{}{}
	for _, repo := range repos {
		if err := metricsForRepo(repo, newMetrics, botDimsList); err != nil {
			return nil, err
		}
	}

	// Delete unused old metrics.
	for m := range oldMetrics {
		if _, ok := newMetrics[m]; !ok {
			if err := m.Delete(); err != nil {
				sklog.Errorf("Failed to delete metric: %s", err)
				// Add the metric to newMetrics so that we'll
				// have the chance to delete it again on the
				// next cycle.
				newMetrics[m] = struct{}{}
			}
		}
	}
	return newMetrics, nil
}

// Start collecting metrics for supported branches.
func Start(ctx context.Context, repoUrls []string, client *http.Client, swarm swarming.ApiClient, pools []string) {
	repos := make([]*gitiles.Repo, 0, len(repoUrls))
	for _, repo := range repoUrls {
		repos = append(repos, gitiles.NewRepo(repo, client))
	}
	lv := metrics2.NewLiveness("last_successful_supported_branches_update")
	oldMetrics := map[metrics2.Int64Metric]struct{}{}
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		newMetrics, err := cycle(repos, oldMetrics, swarm, pools)
		if err == nil {
			lv.Reset()
			oldMetrics = newMetrics
		} else {
			sklog.Errorf("Failed to update supported branches metrics: %s", err)
		}
	})
}
