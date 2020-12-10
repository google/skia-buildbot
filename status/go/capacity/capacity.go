package capacity

// This package makes multiple queries to InfluxDB to get metrics that allow
// us to gauge theoretical capacity needs. Presently, the last 3 days worth of
// swarming data is used as the basis for these metrics.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

type CapacityClient struct {
	tcc   *task_cfg_cache.TaskCfgCache
	tasks cache.TaskCache
	repos repograph.Map
	// The cached measurements
	lastMeasurements map[string]BotConfig
	mtx              sync.Mutex
}

// Caller is responsible for periodically updating the arguments.
func New(tcc *task_cfg_cache.TaskCfgCache, tasks cache.TaskCache, repos repograph.Map) *CapacityClient {
	return &CapacityClient{tcc: tcc, tasks: tasks, repos: repos}
}

type taskData struct {
	Duration time.Duration
	BotId    string
}

type TaskDuration struct {
	Name            string        `json:"task_name"`
	AverageDuration time.Duration `json:"task_duration_ns"`
	OnCQ            bool          `json:"on_cq_also"`
}

// BotConfig represents one bot config we test on. I.e. one group of dimensions that execute tasks.
type BotConfig struct {
	Dimensions           []string        `json:"dimensions"`
	Bots                 map[string]bool `json:"bots"` // maps bot id to boolean
	TaskAverageDurations []TaskDuration  `json:"tasks"`
}

// getTaskDurations fetches Tasks from the TaskCache and generates a taskData for each completed
// Swarming Task, grouped by repo and TaskSpec name.
func (c *CapacityClient) getTaskDurations() (map[string]map[string][]taskData, error) {
	// Fetch last 72 hours worth of tasks that TaskScheduler created.
	now := time.Now()
	before := now.Add(-72 * time.Hour)
	tasks, err := c.tasks.GetTasksFromDateRange(before, now)
	if err != nil {
		return nil, fmt.Errorf("Could not fetch tasks between %s and %s: %s", before, now, err)
	}
	sklog.Infof("Found %d tasks in last 72 hours", len(tasks))

	// Go through all the tasks and group the durations and bot ids by task name
	durations := make(map[string]map[string][]taskData)
	count := 0
	for _, task := range tasks {
		// Skip any task that didn't finish or didn't run.  Finished and Started are
		// the same if the task never ran.
		if !task.Done() {
			continue
		}
		if task.Fake() {
			continue
		}
		// TODO(dogben): Need to consider deduplicated tasks also.
		duration := task.Finished.Sub(task.Started)
		if len(durations[task.Repo]) == 0 {
			durations[task.Repo] = map[string][]taskData{}
		}
		if len(durations[task.Repo][task.Name]) == 0 {
			count++
		}
		durations[task.Repo][task.Name] = append(durations[task.Repo][task.Name], taskData{
			Duration: duration,
			BotId:    task.SwarmingBotId,
		})
	}

	sklog.Infof("From %d tasks, we saw %d unique task names over %d repos", len(tasks), count, len(durations))
	return durations, nil
}

// getCQTaskSpecs returns the TaskSpec names of all Jobs on the CQ.
// TODO(benjaminwagner): return a util.StringSet{}
func (c *CapacityClient) getCQTaskSpecs() ([]string, error) {
	cqTasks, err := cq.GetSkiaCQTryBots()
	if err != nil {
		sklog.Warningf("Could not get Skia CQ bots.  Continuing anyway.  %s", err)
		cqTasks = []string{}
	}
	infraCQTasks, err := cq.GetSkiaInfraCQTryBots()
	if err != nil {
		sklog.Warningf("Could not get Infra CQ bots.  Continuing anyway.  %s", err)
		infraCQTasks = []string{}
	}
	cqTasks = append(cqTasks, infraCQTasks...)
	// TODO(benjaminwagner): This is a list of Job names, not Task names.
	return cqTasks, nil
}

// botConfigKey creates a string key from a list of dimensions. dims will be sorted.
func botConfigKey(dims []string) string {
	sort.Strings(dims)
	return strings.Join(dims, "|")
}

// getTasksCfg finds the most recent cached TasksCfg for the main branch of the given repo. Also
// returns the commit hash where the TasksCfg was found.
func (c *CapacityClient) getTasksCfg(ctx context.Context, repo string) (*specs.TasksCfg, string, error) {
	repoGraph, ok := c.repos[repo]
	if !ok {
		return nil, "", skerr.Fmt("Unknown repo %q", repo)
	}
	commit := repoGraph.Get(git.DefaultBranch)
	if commit == nil {
		return nil, "", skerr.Fmt("Unable to find main branch in %q", repo)
	}
	const lookback = 5
	for i := 0; i < lookback; i++ {
		rs := types.RepoState{
			Repo:     repo,
			Revision: commit.Hash,
		}
		tasksCfg, cachedErr, err := c.tcc.Get(ctx, rs)
		if cachedErr != nil {
			err = cachedErr
		}
		if err == nil {
			return tasksCfg, commit.Hash, nil
		}
		sklog.Warningf("Error getting TasksCfg for %s: %s", rs, err)
		if p := commit.GetParents(); len(p) == 0 {
			return nil, "", skerr.Fmt("Unable to find TasksCfg for any revision in %q", repo)
		} else {
			commit = p[0]
		}
	}
	return nil, "", skerr.Fmt("Unable to find any TasksCfg for %s looking back %d commits.", repo, lookback)
}

// computeBotConfigs groups TaskSpecs by identical dimensions and returns a BotConfig for each
// dimension set. Arguments are getTaskDurations() and getCQTaskSpecs(). The returned map is keyed
// by botConfigKey(BotConfig.Dimensions).
func (c *CapacityClient) computeBotConfigs(ctx context.Context, durations map[string]map[string][]taskData, cqTasks []string) (map[string]BotConfig, error) {
	// botConfigs coalesces all dimension groups together. For example, all tests
	// that require "device_type:flounder|device_os:N12345" will be grouped together,
	// letting us determine our actual use and theoretical capacity of that config.
	botConfigs := make(map[string]BotConfig)

	for repo, repoDurations := range durations {
		// The db.Task structs don't have their dimensions, so we pull those off of the main
		// branches of the repo. If the dimensions were updated recently, this may lead
		// to some inaccuracies. In practice, this probably won't happen because updates
		// tend to update, say, all the Nexus10s to a new OS version, which is effectively no change.
		tasksCfg, hash, err := c.getTasksCfg(ctx, repo)
		if err != nil {
			sklog.Errorf("Skipping repo %s: %s", repo, err)
			continue
		}

		for taskName, taskRuns := range repoDurations {
			// Look up the TaskSpec to get the dimensions.
			taskSpec, ok := tasksCfg.Tasks[taskName]
			if !ok {
				sklog.Warningf("Could not find taskspec for %s at %s@%s.", taskName, repo, hash)
				continue
			}
			dims := taskSpec.Dimensions
			key := botConfigKey(dims)
			config, ok := botConfigs[key]
			if !ok {
				config = BotConfig{
					Dimensions:           dims,
					Bots:                 make(map[string]bool),
					TaskAverageDurations: make([]TaskDuration, 0),
				}
			}
			// Compute average duration and add all the bots we've seen on this task
			avgDuration := time.Duration(0)
			for _, td := range taskRuns {
				avgDuration += td.Duration
				config.Bots[td.BotId] = true
			}
			if len(taskRuns) != 0 {
				avgDuration /= time.Duration(len(taskRuns))
			}
			config.TaskAverageDurations = append(config.TaskAverageDurations, TaskDuration{
				Name:            taskName,
				AverageDuration: avgDuration,
				OnCQ:            util.In(taskName, cqTasks),
			})
			botConfigs[key] = config
		}
	}
	return botConfigs, nil
}

// mergeBotConfigs replaces overlapping BotConfigs in the given map by a combined BotConfig.
func mergeBotConfigs(botConfigs map[string]BotConfig) {
	// okToMerge returns true if the BotConfigs referenced by configKeys do not have conflicting
	// dimensions.
	okToMerge := func(configKeys []string) bool {
		combinedDims := map[string]string{}
		for _, configKey := range configKeys {
			nextDims, err := swarming.ParseDimensionsSingleValue(botConfigs[configKey].Dimensions)
			if err != nil {
				return false
			}
			for k, v := range nextDims {
				if other, ok := combinedDims[k]; ok && v != other {
					return false
				}
				combinedDims[k] = v
			}
		}
		return true
	}

	botIdToConfigs := map[string][]string{}
	configsToMerge := util.NewStringSet()
	for key, config := range botConfigs {
		for botId := range config.Bots {
			if !util.In(key, botIdToConfigs[botId]) {
				botIdToConfigs[botId] = append(botIdToConfigs[botId], key)
			}
			if len(botIdToConfigs[botId]) > 1 && okToMerge(botIdToConfigs[botId]) {
				configsToMerge[key] = true
			}
		}
	}
	for key := range configsToMerge {
		if _, ok := botConfigs[key]; !ok {
			// Already merged.
			continue
		}
		groupKeys := util.NewStringSet()
		bots := util.NewStringSet()
		var gather func(string)
		gather = func(key string) {
			if _, ok := groupKeys[key]; ok {
				return
			}
			groupKeys[key] = true
			config := botConfigs[key]
			for botId := range config.Bots {
				bots[botId] = true
				for _, other := range botIdToConfigs[botId] {
					gather(other)
				}
			}
		}
		gather(key)
		dimSet := util.NewStringSet()
		durs := []TaskDuration{}
		for key := range groupKeys {
			config := botConfigs[key]
			dimSet.AddLists(config.Dimensions)
			durs = append(durs, config.TaskAverageDurations...)
			delete(botConfigs, key)
		}
		dims := dimSet.Keys()
		newKey := botConfigKey(dims)
		botConfigs[newKey] = BotConfig{
			Dimensions:           dims,
			Bots:                 bots,
			TaskAverageDurations: durs,
		}
	}
}

// QueryAll updates the capacity metrics.
func (c *CapacityClient) QueryAll(ctx context.Context) error {
	sklog.Info("Recounting Capacity Stats")

	durations, err := c.getTaskDurations()
	if err != nil {
		return err
	}

	cqTasks, err := c.getCQTaskSpecs()
	if err != nil {
		return err
	}

	botConfigs, err := c.computeBotConfigs(ctx, durations, cqTasks)
	if err != nil {
		return err
	}

	// Merge BotConfigs with overlapping bots. (This could be the wrong thing to do if a bot's
	// dimensions change, e.g. installing a GPU in a CPU bot or recreating a GCE bot with a different
	// CPU type. I expect that to be a rare case.)
	mergeBotConfigs(botConfigs)

	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.lastMeasurements = botConfigs
	return err
}

// StartLoading begins an infinite loop to recompute the capacity metrics after a
// given interval of time.  Any errors are logged, but the loop is not broken.
func (c *CapacityClient) StartLoading(ctx context.Context, interval time.Duration) {
	go func() {
		util.RepeatCtx(ctx, interval, func(ctx context.Context) {
			if err := c.QueryAll(ctx); err != nil {
				sklog.Errorf("There was a problem counting capacity stats")
			}
		})
	}()
}

func (c *CapacityClient) CapacityMetrics() map[string]BotConfig {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lastMeasurements
}
