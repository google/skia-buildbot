package main

/*
	Tool for computing average time to X% testedness for commits.
*/

import (
	"flag"
	"math"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory.")
)

type TaskFinishedSlice []*db.Task

func (s TaskFinishedSlice) Len() int { return len(s) }

func (s TaskFinishedSlice) Less(i, j int) bool {
	return s[i].Finished.Before(s[j].Finished)
}

func (s TaskFinishedSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func main() {
	common.Init()
	defer common.LogPanic()

	wd, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Setup.
	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	repos, err := repograph.NewMap(common.PUBLIC_REPOS, wd)
	if err != nil {
		sklog.Fatal(err)
	}

	taskCfgCache, err := specs.NewTaskCfgCache(repos, path.Join(wd, "depot_tools"), path.Join(wd, "taskCfgCache"), 1)
	if err != nil {
		sklog.Fatal(err)
	}

	timeWindow := 24 * time.Hour
	end := time.Now().Add(-72 * time.Hour)
	start := end.Add(-timeWindow)

	tasks, err := taskDb.GetTasksFromDateRange(start, end)
	if err != nil {
		sklog.Fatal(err)
	}

	// Organize by repo and task spec.
	sklog.Infof("Organizing task specs.")
	byTaskSpec := make(map[string]map[string][]*db.Task, len(repos))
	for _, t := range tasks {
		// Filter by repo, since the DB might contain tasks from repos
		// we don't care about.
		if _, ok := repos[t.Repo]; !ok {
			continue
		}

		if !t.Done() {
			continue
		}

		m, ok := byTaskSpec[t.Repo]
		if !ok {
			m = map[string][]*db.Task{}
			byTaskSpec[t.Repo] = m
		}
		m[t.Name] = append(m[t.Name], t)
	}

	// Collect the commits we care about.
	sklog.Infof("Collecting commits.")
	commits := make(map[string][]*repograph.Commit, len(repos))
	for repoUrl, repo := range repos {
		commits[repoUrl] = []*repograph.Commit{}
		if err := repo.RecurseAllBranches(func(c *repograph.Commit) (bool, error) {
			if start.After(c.Timestamp) {
				return false, nil
			}
			if c.Timestamp.After(end) {
				return true, nil
			}
			commits[repoUrl] = append(commits[repoUrl], c)
			return true, nil
		}); err != nil {
			sklog.Fatal(err)
		}
	}

	percentiles := []float64{50.0, 75.0, 90.0, 99.0, 100.0}
	metrics := make(map[string]map[string]map[float64]time.Duration, len(repos))

	// for each commit:
	for repoUrl, c := range commits {
		sklog.Infof("Repo: %s", repoUrl)
		hasAncestor := make(map[string]map[string]bool, len(commits))
		repoMetrics := make(map[string]map[float64]time.Duration, len(commits))
		for _, commit := range c {
			sklog.Infof("Commit: %s", commit.Hash)
			sklog.Infof("  Reading tasks cfg")
			cfg, err := taskCfgCache.ReadTasksCfg(db.RepoState{
				Repo:     repoUrl,
				Revision: commit.Hash,
			})
			if err != nil {
				sklog.Fatal(err)
			}
			// for each task spec:
			sklog.Infof("  Finding task specs.")
			commitTasks := make([]*db.Task, 0, len(cfg.Tasks))
			totalImportantTasks := 0
			missing := []string{}
			for spec, _ := range cfg.Tasks {
				// Filter out periodic tasks.
				// TODO(borenet): Use the Job.Trigger field for this?
				if strings.Contains(spec, "Nightly") || strings.Contains(spec, "Weekly") || spec == "Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-CT_DM_1m_SKPs" {
					//sklog.Infof("Filtering out %s", spec)
					continue
				}
				totalImportantTasks++

				// find the first completed task which ran at a commit at or after the given commit.
				var first *db.Task
				for _, task := range byTaskSpec[repoUrl][spec] {
					c := repos[repoUrl].Get(task.Revision)
					ha, ok := hasAncestor[task.Revision]
					if !ok {
						ha = map[string]bool{}
						hasAncestor[task.Revision] = ha
					}
					found, ok := ha[commit.Hash]
					if !ok {
						found = c.HasAncestor(commit.Hash)
						ha[commit.Hash] = found
					}
					if found {
						if first == nil || first.Finished.After(task.Finished) {
							first = task
						}
					}
				}
				if first != nil {
					commitTasks = append(commitTasks, first)
				}
			}
			// sort tasks by completed time
			sort.Sort(TaskFinishedSlice(commitTasks))

			// for each percentile:
			sklog.Infof("  Calculating percentiles.")
			m := make(map[float64]time.Duration, len(percentiles))
			for _, pct := range percentiles {
				// record the time at which the X%th task finished, subtract the commit time
				idx := int(math.Ceil(pct*float64(totalImportantTasks)/float64(100.0))) - 1
				if len(commitTasks) <= idx {
					// The commit hasn't reached this amount of coverage yet.
					continue
				}
				d := commitTasks[idx].Finished.Sub(commit.Timestamp)
				m[pct] = d
				sklog.Infof("    %2f: %s", pct, d)
			}
			repoMetrics[commit.Hash] = m
		}
		metrics[repoUrl] = repoMetrics
	}

	// average the percentiles
	for repoUrl, _ := range repos {
		sklog.Infof("Repo: %s", repoUrl)
		results := make(map[float64]time.Duration, len(percentiles))
		counts := make(map[float64]int64, len(percentiles))
		for _, m := range metrics[repoUrl] {
			for d, v := range m {
				results[d] += v
				counts[d]++
			}
		}
		for d, res := range results {
			results[d] = time.Duration(int64(res) / counts[d])
		}

		// Print results.
		sklog.Infof("Metrics:")
		for d, res := range results {
			sklog.Infof("%2f%%: %s", d, res)
		}
	}
}
