package time_to_bot_coverage

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/specs"
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

	PERCENTILES = []float64{50.0, 75.0, 90.0, 99.0, 100.0}
)

type percentData struct {
	RepoData map[string]map[*repograph.Commit]*commitData
}

type gobPercentData struct {
	RepoData map[string]map[string]*commitData
}

func fmtPercent(v float64) string {
	return fmt.Sprintf("%2.1f", v)
}

func fmtStream(repo string) string {
	split := strings.Split(repo, "/")
	repoName := strings.TrimSuffix(split[len(split)-1], ".git")
	return fmt.Sprintf("commits-%s", repoName)
}

// read pulls all events from the EventDB and returns them in a convenient format.
func read(edb events.EventDB, repos repograph.Map, now time.Time) (*percentData, error) {
	rv := &percentData{
		RepoData: map[string]map[*repograph.Commit]*commitData{},
	}
	for repoUrl, repo := range repos {
		ev, err := edb.Range(fmtStream(repoUrl), BEGINNING_OF_TIME, now)
		if err != nil {
			return nil, err
		}
		rvSub := make(map[*repograph.Commit]*commitData, len(ev))
		for _, e := range ev {
			d := new(commitData)
			if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(d); err != nil {
				return nil, err
			}
			c := repo.Get(d.Hash)
			if c == nil {
				return nil, fmt.Errorf("No such commit %q in %s", d.Hash, repoUrl)
			}
			rvSub[c] = d
		}
		rv.RepoData[repoUrl] = rvSub
	}
	return rv, nil
}

// write (re)inserts all events into the EventDB.
func write(edb events.EventDB, data *percentData) error {
	for repoUrl, sub := range data.RepoData {
		s := fmtStream(repoUrl)
		for _, cData := range sub {
			// TODO(borenet): Only if changed.
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(cData); err != nil {
				return err
			}
			if err := edb.Insert(&events.Event{
				Stream:    s,
				Timestamp: cData.Timestamp,
				Data:      buf.Bytes(),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func ignoreTask(name string) bool {
	for _, i := range IGNORE {
		if strings.Contains(name, i) {
			return true
		}
	}
	return false
}

type commitData struct {
	Hash      string                   `json:"hash"`
	Metrics   map[string]time.Duration `json:"metrics"`
	NumTasks  int                      `json:"num_tasks"`
	Tasks     map[string]time.Duration `json:"tasks"`
	Timestamp time.Time
}

func (d *commitData) computeMetrics() {
	durations := make([]int64, 0, d.NumTasks)
	for _, d := range d.Tasks {
		durations = append(durations, int64(d))
	}
	sort.Sort(util.Int64Slice(durations))
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

func (d *commitData) done() bool {
	return d.NumTasks == len(d.Tasks)
}

func addMetric(s *events.EventStream, repoUrl string, pct float64, period time.Duration) {
	tags := map[string]string{
		"percent": fmtPercent(pct),
		"repo":    repoUrl,
	}
	s.AggregateMetric(tags, period, func(ev []*events.Event) (float64, error) {
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

func cycle(taskDb db.RemoteDB, repos repograph.Map, tcc *specs.TaskCfgCache, edb events.EventDB, em *events.EventMetrics, lastFinished, now time.Time, workdir string) error {
	if err := repos.Update(); err != nil {
		return err
	}

	totalCommits := 0
	for _, r := range repos {
		totalCommits += r.Len()
	}

	// cfgs is a local cache for TaskCfgs.
	cfgs := map[*repograph.Commit]*specs.TasksCfg{}

	// Read cached data.
	data, err := read(edb, repos, now)
	if err != nil {
		return err
	}

	period := 24 * time.Hour
	periodStart := lastFinished
	if util.TimeIsZero(periodStart) {
		periodStart = BEGINNING_OF_TIME
	} else {
		periodStart = periodStart.Add(-COMMIT_TASK_WINDOW) // In case we backfilled and finished some tasks.
	}
	for {
		periodEnd := periodStart.Add(period)
		if periodEnd.After(now) {
			periodEnd = now
		}

		sklog.Infof("Loading data for %s - %s", periodStart, periodEnd)
		tasks, err := taskDb.GetTasksFromDateRange(periodStart, periodEnd)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if !t.Done() || t.IsTryJob() || ignoreTask(t.Name) {
				continue
			}

			c, repoUrl, _, err := repos.FindCommit(t.Revision)
			if err != nil {
				return err
			}
			if repoUrl != t.Repo {
				sklog.Fatal("Got wrong repo for commit %s in %s (got %s)", t.Revision, t.Repo, repoUrl)
			}

			repoData, ok := data.RepoData[repoUrl]
			if !ok {
				repoData = map[*repograph.Commit]*commitData{}
				data.RepoData[repoUrl] = repoData
			}

			// For each commit covered by the task, record the lag time.
			if err := c.Recurse(func(commit *repograph.Commit) (bool, error) {
				// Only record lag time for this task spec if it was
				// defined for this commit.
				cfg, ok := cfgs[commit]
				if !ok {
					c, err := tcc.ReadTasksCfg(db.RepoState{
						Repo:     repoUrl,
						Revision: commit.Hash,
					})
					if err != nil {
						// Some old commits only have tasks without jobs. Skip them.
						if strings.Contains(err.Error(), "is not reachable by any Job") {
							cfgs[commit] = &specs.TasksCfg{
								Tasks: map[string]*specs.TaskSpec{},
								Jobs:  map[string]*specs.JobSpec{},
							}
							return false, nil
						}
						return false, err
					}
					cfg = c
					cfgs[commit] = cfg
				}

				// Get the cached data for this commit.
				cData, ok := repoData[commit]
				if !ok {
					numTasks := 0
					for name, _ := range cfg.Tasks {
						if !ignoreTask(name) {
							numTasks++
						}
					}
					cData = &commitData{
						Hash:      commit.Hash,
						NumTasks:  numTasks,
						Tasks:     map[string]time.Duration{},
						Timestamp: commit.Timestamp,
					}
					repoData[commit] = cData
				}

				if _, ok := cfg.Tasks[t.Name]; !ok {
					return false, nil
				}

				d := t.Finished.Sub(commit.Timestamp)

				// If the commit hasn't been covered yet or was
				// covered by another task which ran after this
				// one, record the lag time and keep recursing.
				if best, ok := cData.Tasks[t.Name]; !ok || best > d {
					cData.Tasks[t.Name] = d
					return true, nil
				}

				// This commit was covered by a previous task,
				// so all previous commits will also be covered.
				return false, nil
			}); err != nil {
				return err
			}
		}
		if err := write(edb, data); err != nil {
			return err
		}
		if periodEnd.Equal(now) {
			break
		}
		periodStart = periodStart.Add(period)
	}

	// Calculate metrics for each commit.
	// TODO(borenet): Only do the commits in range?
	for _, repoData := range data.RepoData {
		for _, cData := range repoData {
			cData.computeMetrics()
		}
	}
	if err := write(edb, data); err != nil {
		return err
	}

	if err := em.UpdateMetrics(); err != nil {
		return err
	}
	em.LogMetrics()
	if err := writeTs(workdir, now); err != nil {
		return fmt.Errorf("Failed to write timestamp file: %s", err)
	}
	return nil
}

func readTs(workdir string) (time.Time, error) {
	var rv time.Time
	b, err := ioutil.ReadFile(path.Join(workdir, TIMESTAMP_FILE))
	if err != nil {
		if os.IsNotExist(err) {
			return BEGINNING_OF_TIME, nil
		}
		return rv, err
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&rv); err != nil {
		return rv, err
	}
	return rv, nil
}

func writeTs(workdir string, ts time.Time) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ts); err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(workdir, TIMESTAMP_FILE), buf.Bytes(), os.ModePerm)
}

func Start(dbUrl, workdir string, ctx context.Context) error {
	// Setup.
	wd, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(wd, os.ModePerm); err != nil {
		return err
	}

	taskDb, err := remote_db.NewClient(dbUrl)
	if err != nil {
		return err
	}

	repos, err := repograph.NewMap(common.PUBLIC_REPOS, wd)
	if err != nil {
		return err
	}

	tcc, err := specs.NewTaskCfgCache(repos, path.Join(wd, "depot_tools"), path.Join(wd, "taskCfgCache"), 1)
	if err != nil {
		return err
	}

	// Set up event metrics.
	edb, err := events.NewEventDB(path.Join(workdir, "percent-metrics.bdb"))
	if err != nil {
		return err
	}
	em, err := events.NewEventMetrics(edb)
	if err != nil {
		return err
	}
	for repoUrl, _ := range repos {
		for _, p := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
			s := em.GetEventStream(fmtStream(repoUrl))
			for _, pct := range PERCENTILES {
				addMetric(s, repoUrl, pct, p)
			}
		}
	}

	lv := metrics2.NewLiveness("last-successful-bot-coverage-metrics")
	lastFinished, err := readTs(wd)
	if err != nil {
		return err
	}
	go util.RepeatCtx(10*time.Minute, ctx, func() {
		now := time.Now()
		if err := cycle(taskDb, repos, tcc, edb, em, lastFinished, now, wd); err != nil {
			sklog.Errorf("Failed to obtain avg time to X%% bot coverage metrics: %s", err)
		} else {
			lastFinished = now
			lv.Reset()
		}
	})
	return nil
}
