package bot_metrics

/*
	This package provides metrics for the average time between a commit
	landing and it being tested on X% of bots, where X is one of several
	percentiles.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
)

const (
	// Cache the last-finished timestamp in this file.
	TIMESTAMP_FILE = "last_finished.gob"
)

var (
	// The Task Scheduler didn't exist before this, so don't bother loading
	// data from before.
	BEGINNING_OF_TIME = time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)

	// This is the assumed maximum lag time between a commit landing and a
	// task being triggered. When looking at tasks for a given commit, we'll
	// look this far ahead of the commit.
	COMMIT_TASK_WINDOW = 4 * 24 * time.Hour

	// For efficiency, or periodic tasks which shouldn't factor in.
	IGNORE = []string{
		"Weekly",
		"Nightly",
		"Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-CT_DM_1m_SKPs",
	}

	PERCENTILES  = []float64{50.0, 75.0, 90.0, 99.0, 100.0}
	TIME_PERIODS = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour}

	MAX_TIME_PERIOD = TIME_PERIODS[len(TIME_PERIODS)-1] + COMMIT_TASK_WINDOW
)

// fmtPercent returns a string representation of the given percentage as float64.
func fmtPercent(v float64) string {
	return fmt.Sprintf("%2.1f", v)
}

// fmtStream returns the name of an event stream given a repo URL.
func fmtStream(repo string) string {
	split := strings.Split(repo, "/")
	repoName := strings.TrimSuffix(split[len(split)-1], ".git")
	return fmt.Sprintf("commits-%s", repoName)
}

// read pulls all events from the EventDB and returns them in a convenient format.
func read(edb events.EventDB, repos repograph.Map, from, to time.Time) (map[string]map[*repograph.Commit]*commitData, error) {
	rv := map[string]map[*repograph.Commit]*commitData{}
	for repoUrl, repo := range repos {
		ev, err := edb.Range(fmtStream(repoUrl), from, to)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to read range from %q to %q in repo %q", from, to, repoUrl)
		}
		rvSub := make(map[*repograph.Commit]*commitData, len(ev))
		for _, e := range ev {
			d := new(commitData)
			if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(d); err != nil {
				return nil, skerr.Wrapf(err, "failed to decode commit data")
			}
			c := repo.Get(d.Hash)
			if c == nil {
				// This commit was lost in the master -> main migration.
				if d.Hash == "33de3938b2440af4f41979a6dacbee967d30aa56" {
					continue
				}
				return nil, skerr.Fmt("No such commit %q in %s", d.Hash, repoUrl)
			}
			rvSub[c] = d
		}
		rv[repoUrl] = rvSub
	}
	return rv, nil
}

// write (re)inserts all events into the EventDB.
func write(edb events.EventDB, data map[string]map[*repograph.Commit]*commitData) error {
	for repoUrl, sub := range data {
		s := fmtStream(repoUrl)
		for _, cData := range sub {
			// TODO(borenet): Only if changed.
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(cData); err != nil {
				return skerr.Wrap(err)
			}
			if err := edb.Insert(&events.Event{
				Stream:    s,
				Timestamp: cData.Timestamp,
				Data:      buf.Bytes(),
			}); err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}

// ignoreTask returns true if the task should be ignored.
func ignoreTask(name string) bool {
	for _, i := range IGNORE {
		if strings.Contains(name, i) {
			return true
		}
	}
	return false
}

// commitData is a struct containing all of the information we need about a
// commit in order to compute "average time to X% bot coverage" metrics,
// including X% coverage metrics about the individual commit.
type commitData struct {
	Hash      string                   `json:"hash"`
	Metrics   map[string]time.Duration `json:"metrics"`
	NumTasks  int                      `json:"num_tasks"`
	Tasks     map[string]time.Duration `json:"tasks"`
	Timestamp time.Time
}

// computeMetrics computes "time to X% coverage" metrics for the individual
// commit.
func (d *commitData) computeMetrics() {
	var durations []int64
	if d.NumTasks > 0 {
		durations = make([]int64, 0, d.NumTasks)
		for _, d := range d.Tasks {
			durations = append(durations, int64(d))
		}
		sort.Sort(util.Int64Slice(durations))
	}
	m := make(map[string]time.Duration, len(PERCENTILES))
	for _, pct := range PERCENTILES {
		// record the time at which the X%th task finished, subtract the commit time
		idx := int(math.Ceil(pct*float64(d.NumTasks)/float64(100.0))) - 1
		if idx < 0 || len(d.Tasks) <= idx {
			// The commit hasn't reached this amount of coverage yet.
			continue
		}
		m[fmtPercent(pct)] = time.Duration(durations[idx])
	}
	d.Metrics = m
}

// addMetric adds the given aggregate metric to the stream.
func addMetric(s *events.EventStream, repoUrl string, pct float64, period time.Duration) error {
	tags := map[string]string{
		"percent": fmtPercent(pct),
		"repo":    repoUrl,
	}
	return s.AggregateMetric(tags, period, func(ev []*events.Event) (float64, error) {
		if len(ev) == 0 {
			return 0.0, nil
		}
		count := 0
		total := time.Duration(0)
		for _, e := range ev {
			var d commitData
			if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&d); err != nil {
				return 0.0, err
			}
			v := d.Metrics[fmtPercent(pct)]
			if v == 0 {
				continue
			}
			count++
			total += v
		}
		if count == 0 {
			return 0.0, nil
		}
		rv := total / time.Duration(count)
		sklog.Infof("%s avg time to %2.1f%% tested (%s): %s", repoUrl, pct, period, rv)
		return float64(rv), nil
	})
}

// cycle runs ingestion of task data, maps each task to the commits it covered
// before any other task, and inserts event data based on the lag time between
// a commit landing and each task finishing for that commit.
func cycle(ctx context.Context, tCache cache.TaskCache, repos repograph.Map, tcc *task_cfg_cache.TaskCfgCache, edb events.EventDB, em *events.EventMetrics, lastFinished, now time.Time) error {
	if err := tCache.Update(ctx); err != nil {
		return skerr.Wrapf(err, "failed to update task cache")
	}

	totalCommits := 0
	for _, r := range repos {
		totalCommits += r.Len()
	}

	// cfgs is a local cache for TaskCfgs.
	cfgs := map[*repograph.Commit]*specs.TasksCfg{}

	// Compute lag times for all commits in range.
	period := 24 * time.Hour
	periodStart := lastFinished
	if util.TimeIsZero(periodStart) {
		periodStart = time.Now().Add(-MAX_TIME_PERIOD)
	} else {
		periodStart = periodStart.Add(-COMMIT_TASK_WINDOW) // In case we backfilled and finished some tasks.
	}

	// Read cached data.
	data, err := read(edb, repos, periodStart, now)
	if err != nil {
		return skerr.Wrapf(err, "failed to read cached data")
	}

	for {
		// Load tasks from the time period.
		periodEnd := periodStart.Add(period)
		if periodEnd.After(now) {
			periodEnd = now
		}

		sklog.Infof("Loading data for %s - %s", periodStart, periodEnd)
		tasks, err := tCache.GetTasksFromDateRange(periodStart, periodEnd)
		if err != nil {
			return skerr.Wrapf(err, "failed to load tasks from %s to %s", periodStart, periodEnd)
		}

		// For each task, find all commits first covered by the task
		// and record the lag time between the commit landing and the
		// task finishing.
		for _, t := range tasks {
			if !t.Done() || t.IsTryJob() || ignoreTask(t.Name) {
				continue
			}
			if _, ok := repos[t.Repo]; !ok {
				continue
			}

			c, repoUrl, _, err := repos.FindCommit(t.Revision)
			if err != nil {
				if strings.Contains(err.Error(), "Unable to find commit") {
					// Assume this means that the git commit
					// history was changed, in which case
					// we'll never be able to process this
					// task. Just drop it.
					sklog.Errorf("Failed to process task %s: %s; did git history change? Ignoring.", t.Id, err)
					continue
				} else {
					return skerr.Wrapf(err, "failed to process task %s", err)
				}
			}
			if repoUrl != t.Repo {
				return skerr.Fmt("Got wrong repo for commit %s in %s (got %s)", t.Revision, t.Repo, repoUrl)
			}

			repoData, ok := data[repoUrl]
			if !ok {
				repoData = map[*repograph.Commit]*commitData{}
				data[repoUrl] = repoData
			}

			// For each commit covered by the task, record the lag time.
			if err := c.Recurse(func(commit *repograph.Commit) error {
				// Prevent us from tracing through the entire commit history.
				if commit.Timestamp.Before(now.Add(-COMMIT_TASK_WINDOW)) {
					return repograph.ErrStopRecursing
				}

				// Get the cached data for this commit.
				cData, ok := repoData[commit]
				if !ok {
					cData = &commitData{
						Hash:      commit.Hash,
						NumTasks:  -1,
						Tasks:     map[string]time.Duration{},
						Timestamp: commit.Timestamp,
					}
					repoData[commit] = cData
				}

				// Only record lag time for this task spec if it was
				// defined for this commit.
				if t.Id != "buildbot-id" {
					cfg, ok := cfgs[commit]
					if !ok {
						c, cachedErr, err := tcc.Get(ctx, types.RepoState{
							Repo:     repoUrl,
							Revision: commit.Hash,
						})
						if cachedErr != nil || err == task_cfg_cache.ErrNoSuchEntry {
							return nil
						} else if err != nil {
							return err
						}
						cfg = c
						cfgs[commit] = cfg
					}

					// Get the cached data for this commit.
					if cData.NumTasks < 0 {
						numTasks := 0
						for name := range cfg.Tasks {
							if !ignoreTask(name) {
								numTasks++
							}
						}
						cData.NumTasks = numTasks
					}

					if _, ok := cfg.Tasks[t.Name]; !ok {
						return repograph.ErrStopRecursing
					}
				}

				d := t.Finished.Sub(commit.Timestamp)

				// If the commit hasn't been covered yet or was
				// covered by another task which ran after this
				// one, record the lag time and keep recursing.
				if best, ok := cData.Tasks[t.Name]; !ok || best > d {
					cData.Tasks[t.Name] = d
					return nil
				}

				// This commit was covered by a previous task,
				// so all previous commits will also be covered.
				return repograph.ErrStopRecursing
			}); err != nil {
				return skerr.Wrap(err)
			}
		}
		if err := write(edb, data); err != nil {
			return skerr.Wrapf(err, "failed to write results")
		}
		if periodEnd.Equal(now) {
			break
		}
		periodStart = periodStart.Add(period)
	}

	// Calculate metrics for each commit.
	// TODO(borenet): Only do the commits in range?
	for _, repoData := range data {
		for _, cData := range repoData {
			cData.computeMetrics()
		}
	}
	if err := write(edb, data); err != nil {
		return skerr.Wrapf(err, "failed to write results (2)")
	}

	if err := em.UpdateMetrics(); err != nil {
		return skerr.Wrapf(err, "failed to update metrics")
	}
	em.LogMetrics()
	return nil
}

// getLastIngestionTs returns the timestamp of the last commit for which we
// successfully ingested events.
func getLastIngestionTs(edb events.EventDB, repos repograph.Map) (time.Time, error) {
	timeEnd := time.Now()
	window := time.Hour
	for {
		timeStart := timeEnd.Add(-window)
		var latest time.Time
		for repo := range repos {
			ev, err := edb.Range(fmtStream(repo), timeStart, timeEnd)
			if err != nil {
				return BEGINNING_OF_TIME, err
			}
			if len(ev) > 0 {
				ts := ev[len(ev)-1].Timestamp
				if ts.After(latest) {
					latest = ts
				}
			}
		}
		if !util.TimeIsZero(latest) {
			return latest, nil
		}
		if timeStart.Before(BEGINNING_OF_TIME) {
			return BEGINNING_OF_TIME, nil
		}
		window *= 2
		timeEnd = timeStart
	}
}

// Start initiates "average time to X% bot coverage" metrics data generation.
// The caller is responsible for updating the passed-in repos and TaskCfgCache.
func Start(ctx context.Context, tCache cache.TaskCache, repos repograph.Map, tcc *task_cfg_cache.TaskCfgCache, btProject, btInstance string, ts oauth2.TokenSource) error {
	// Set up event metrics.
	edb, err := events.NewBTEventDB(ctx, btProject, btInstance, ts)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create EventDB")
	}
	em, err := events.NewEventMetrics(edb, "time_to_bot_coverage")
	if err != nil {
		return fmt.Errorf("Failed to create EventMetrics: %s", err)
	}
	for repoUrl := range repos {
		s := em.GetEventStream(fmtStream(repoUrl))
		for _, p := range TIME_PERIODS {
			for _, pct := range PERCENTILES {
				if err := addMetric(s, repoUrl, pct, p); err != nil {
					return fmt.Errorf("Failed to add metric: %s", err)
				}
			}
		}
	}

	lv := metrics2.NewLiveness("last_successful_bot_coverage_metrics")
	lastFinished, err := getLastIngestionTs(edb, repos)
	if err != nil {
		return fmt.Errorf("Failed to get timestamp of last successful ingestion: %s", err)
	}
	go util.RepeatCtx(ctx, 10*time.Minute, func(ctx context.Context) {
		now := time.Now()
		if err := cycle(ctx, tCache, repos, tcc, edb, em, lastFinished, now); err != nil {
			sklog.Errorf("Failed to obtain avg time to X%% bot coverage metrics: %s", err)
		} else {
			lastFinished = now
			lv.Reset()
		}
	})
	return nil
}
