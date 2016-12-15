package capacity

// This package makes multiple queries to InfluxDB to get metrics that allow
// us to gauge theoretical capacity needs. Presently, the last 3 days worth of
// swarming data is used as the basis for these metrics.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

type CapacityClient struct {
	tcc   *specs.TaskCfgCache
	tasks db.TaskCache
	repos repograph.Map
	// The cached measurements
	lastMeasurements map[string]BotConfig
}

func New(tcc *specs.TaskCfgCache, tasks db.TaskCache, repos repograph.Map) *CapacityClient {
	return &CapacityClient{tcc: tcc, tasks: tasks, repos: repos}
}

type taskData struct {
	Duration time.Duration
	BotId    string
}

type TaskDuration struct {
	Name            string        `json:"task_name"`
	AverageDuration time.Duration `json:"task_duration_ns"`
}

// BotConfig represents one bot config we test on. I.e. one group of dimensions that execute tasks.
type BotConfig struct {
	Dimensions           []string        `json:"dimensions"`
	Bots                 map[string]bool `json:"bots"` // maps bot id to boolean
	TaskAverageDurations []TaskDuration  `json:"tasks"`
}

// QueryAll updates the capacity metrics.
func (c *CapacityClient) QueryAll() error {
	glog.Infoln("Recounting Capacity Stats")

	// Fetch last 48 hours worth of tasks that TaskScheduler created.
	now := time.Now()
	before := now.Add(-48 * time.Hour)
	tasks, err := c.tasks.GetTasksFromDateRange(before, now)
	if err != nil {
		return fmt.Errorf("Could not fetch tasks between %s and %s: %s", before, now, err)
	}
	glog.Infof("Found %d tasks in last 48 hours", len(tasks))

	// Go through all the tasks and group the durations and bot ids by task name
	durations := make(map[string][]taskData)
	for _, task := range tasks {
		// Skip any task that didn't finish or didn't run.  Finished and Started are
		// the same if the task never ran.
		if !task.Done() {
			continue
		}
		duration := task.Finished.Sub(task.Started)
		durations[task.Name] = append(durations[task.Name], taskData{
			Duration: duration,
			BotId:    task.SwarmingBotId,
		})
	}

	glog.Infof("From %d tasks, we saw %d unique task names", len(tasks), len(durations))

	// The db.Task structs don't have their dimensions, so we pull those off of the master
	// branches of all the repos. If the dimensions were updated recently, this may lead
	// to some inaccuracies. In practice, this probably won't happen because updates
	// tend to update, say, all the Nexus10s to a new OS version, which is effectively no change.
	tips := []db.RepoState{}
	for name, graph := range c.repos {
		master := graph.Get("master")
		tips = append(tips, db.RepoState{
			Repo:     name,
			Revision: master.Hash,
		})
	}

	glog.Infof("About to look up those tasks in %+v", tips)

	// botConfigs coalesces all dimension groups together. For example, all tests
	// that require "device_type:flounder|device_os:N12345" will be grouped together,
	// letting us determine our actual use and theoretical capacity of that config.
	botConfigs := make(map[string]BotConfig)

	for taskName, taskRuns := range durations {
		var taskSpec *specs.TaskSpec
		var err error
		// Look up the TaskSpec for the dimensions.
		for _, rs := range tips {
			taskSpec, err = c.tcc.GetTaskSpec(rs, taskName)
			if err == nil {
				// no err means we found it
				break
			}
		}
		if err != nil {
			glog.Warningf("Could not find taskspec for %s", taskName)
			continue
		}
		dims := taskSpec.Dimensions
		sort.Strings(dims)
		key := strings.Join(dims, "|")
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
		glog.Infof("Over %d runs, task %s took %s", len(taskRuns), taskName, avgDuration)
		config.TaskAverageDurations = append(config.TaskAverageDurations, TaskDuration{
			Name:            taskName,
			AverageDuration: avgDuration,
		})
		botConfigs[key] = config
	}

	c.lastMeasurements = botConfigs
	return err
}

// StartLoading begins an infinite loop to recompute the capacity metrics after a
// given interval of time.  Any errors are logged, but the loop is not broken.
func (c *CapacityClient) StartLoading(interval time.Duration) {
	go func() {
		util.RepeatCtx(interval, context.Background(), func() {
			if err := c.QueryAll(); err != nil {
				glog.Errorf("There was a problem counting capacity stats")
			}
		})
	}()
}

func (c *CapacityClient) CapacityMetrics() map[string]BotConfig {
	return c.lastMeasurements
}
