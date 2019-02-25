package specs

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/set_if_unset_cache"
	"go.skia.org/infra/go/testutils"
	specs_testutils "go.skia.org/infra/task_scheduler/go/specs/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	buildTask = &TaskSpec{
		CipdPackages: []*CipdPackage{
			&CipdPackage{
				Name:    "android_sdk",
				Path:    "android_sdk",
				Version: "version:0",
			},
		},
		Command:    []string{"ninja", "skia"},
		Dimensions: []string{"pool:Skia", "os:Ubuntu"},
		ExtraTags: map[string]string{
			"is_build_task": "true",
		},
		Isolate:     "compile_skia.isolate",
		MaxAttempts: 5,
		Priority:    0.8,
	}
	testTask = &TaskSpec{
		CipdPackages: []*CipdPackage{
			&CipdPackage{
				Name:    "skimage",
				Path:    "skimage",
				Version: "version:0",
			},
			&CipdPackage{
				Name:    "skp",
				Path:    "skp",
				Version: "version:0",
			},
		},
		Command:      []string{"test", "skia"},
		Dependencies: []string{specs_testutils.BuildTask},
		Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
		EnvPrefixes: map[string][]string{
			"PATH": []string{"curdir"},
		},
		Isolate:  "test_skia.isolate",
		Priority: 0.8,
	}
	perfTask = &TaskSpec{
		CipdPackages: []*CipdPackage{
			&CipdPackage{
				Name:    "skimage",
				Path:    "skimage",
				Version: "version:0",
			},
			&CipdPackage{
				Name:    "skp",
				Path:    "skp",
				Version: "version:0",
			},
		},
		Command:      []string{"perf", "skia"},
		Dependencies: []string{specs_testutils.BuildTask},
		Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
		Isolate:      "perf_skia.isolate",
		Priority:     0.8,
	}

	buildJob = &JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{specs_testutils.BuildTask},
	}
	testJob = &JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{specs_testutils.TestTask},
	}
	perfJob = &JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{specs_testutils.PerfTask},
	}

	baseTasksCfg = &TasksCfg{
		Tasks: map[string]*TaskSpec{
			specs_testutils.BuildTask: buildTask,
			specs_testutils.TestTask:  testTask,
		},
		Jobs: map[string]*JobSpec{
			specs_testutils.BuildTask: buildJob,
			specs_testutils.TestTask:  testJob,
		},
	}
	nextTasksCfg = &TasksCfg{
		Tasks: map[string]*TaskSpec{
			specs_testutils.BuildTask: buildTask,
			specs_testutils.PerfTask:  perfTask,
			specs_testutils.TestTask:  testTask,
		},
		Jobs: map[string]*JobSpec{
			specs_testutils.BuildTask: buildJob,
			specs_testutils.PerfTask:  perfJob,
			specs_testutils.TestTask:  testJob,
		},
	}
)

func TestCopyTaskSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &TaskSpec{
		Caches: []*Cache{
			&Cache{
				Name: "cache-me",
				Path: "if/you/can",
			},
		},
		CipdPackages: []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "23",
			},
		},
		Command:      []string{"do", "something"},
		Dependencies: []string{"coffee", "chocolate"},
		Dimensions:   []string{"width:13", "height:17"},
		Environment: map[string]string{
			"Polluted": "true",
		},
		EnvPrefixes: map[string][]string{
			"PATH": []string{"curdir"},
		},
		ExecutionTimeout: 60 * time.Minute,
		Expiration:       90 * time.Minute,
		ExtraArgs:        []string{"--do-really-awesome-stuff"},
		ExtraTags: map[string]string{
			"dummy_tag": "dummy_val",
		},
		IoTimeout:      10 * time.Minute,
		Isolate:        "abc123",
		MaxAttempts:    5,
		Outputs:        []string{"out"},
		Priority:       19.0,
		ServiceAccount: "fake-account@gmail.com",
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestTaskSpecs(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	assert.NoError(t, repos.Update(ctx))

	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	assert.NoError(t, cache.Set(ctx, rs1, baseTasksCfg, nil))
	assert.NoError(t, cache.Set(ctx, rs2, nextTasksCfg, nil))
	specs, err := cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name := range byName {
			total++
			if rs.Revision == c1 {
				countC1++
			} else if rs.Revision == c2 {
				countC2++
			} else {
				t.Fatalf("Unknown commit: %q", rs.Revision)
			}
			if strings.HasPrefix(name, "Build") {
				countBuild++
			} else if strings.HasPrefix(name, "Test") {
				countTest++
			} else if strings.HasPrefix(name, "Perf") {
				countPerf++
			} else {
				t.Fatalf("Unknown task spec name: %q", name)
			}
		}
	}
	assert.Equal(t, 2, countC1)
	assert.Equal(t, 3, countC2)
	assert.Equal(t, 2, countBuild)
	assert.Equal(t, 2, countTest)
	assert.Equal(t, 1, countPerf)
	assert.Equal(t, 5, total)
}

func TestAddedTaskSpecs(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	assert.NoError(t, repos.Update(ctx))

	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	assert.NoError(t, cache.Set(ctx, rs1, baseTasksCfg, nil))
	assert.NoError(t, cache.Set(ctx, rs2, nextTasksCfg, nil))
	addedTaskSpecs, err := cache.GetAddedTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(addedTaskSpecs[rs1]))
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.BuildTask])
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.TestTask])
	assert.Equal(t, 1, len(addedTaskSpecs[rs2]))
	assert.True(t, addedTaskSpecs[rs2][specs_testutils.PerfTask])

	// c3 adds Beer and Belch (names chosen to avoid merge conficts)
	gb.CreateBranchTrackBranch(ctx, "branchy-mcbranch-face", "master")
	cfg3, err := ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	cfg3.Jobs["Beer"] = &JobSpec{TaskSpecs: []string{"Belch"}}
	cfg3.Tasks["Beer"] = &TaskSpec{
		Dependencies: []string{specs_testutils.BuildTask},
		Isolate:      "swarm_recipe.isolate",
	}
	cfg3.Tasks["Belch"] = &TaskSpec{
		Dependencies: []string{"Beer"},
		Isolate:      "swarm_recipe.isolate",
	}
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg3))
	c3 := gb.Commit(ctx)

	// c4 removes Perf
	gb.CheckoutBranch(ctx, "master")
	cfg4, err := ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	delete(cfg4.Jobs, specs_testutils.PerfTask)
	delete(cfg4.Tasks, specs_testutils.PerfTask)
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg4))
	c4 := gb.Commit(ctx)

	// c5 merges c3 and c4
	c5 := gb.MergeBranch(ctx, "branchy-mcbranch-face")
	cfg5, err := ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)

	// c6 adds back Perf
	cfg6, err := ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	cfg6.Jobs[specs_testutils.PerfTask] = &JobSpec{TaskSpecs: []string{specs_testutils.PerfTask}}
	cfg6.Tasks[specs_testutils.PerfTask] = &TaskSpec{
		Dependencies: []string{specs_testutils.BuildTask},
		Isolate:      "swarm_recipe.isolate",
	}
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg6))
	c6 := gb.Commit(ctx)

	rs3 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c3,
	}
	rs4 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c4,
	}
	rs5 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c5,
	}
	rs6 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c6,
	}

	assert.NoError(t, repos.Update(ctx))
	assert.NoError(t, cache.Set(ctx, rs3, cfg3, nil))
	assert.NoError(t, cache.Set(ctx, rs4, cfg4, nil))
	assert.NoError(t, cache.Set(ctx, rs5, cfg5, nil))
	assert.NoError(t, cache.Set(ctx, rs6, cfg6, nil))
	addedTaskSpecs, err = cache.GetAddedTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2, rs3, rs4, rs5, rs6})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(addedTaskSpecs[rs1]))
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.BuildTask])
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.TestTask])
	assert.Equal(t, 1, len(addedTaskSpecs[rs2]))
	assert.True(t, addedTaskSpecs[rs2][specs_testutils.PerfTask])
	assert.Equal(t, 2, len(addedTaskSpecs[rs3]))
	assert.True(t, addedTaskSpecs[rs3]["Beer"])
	assert.True(t, addedTaskSpecs[rs3]["Belch"])
	assert.Equal(t, 0, len(addedTaskSpecs[rs4]))
	assert.Equal(t, 2, len(addedTaskSpecs[rs5]))
	assert.True(t, addedTaskSpecs[rs5]["Beer"])
	assert.True(t, addedTaskSpecs[rs5]["Belch"])
	assert.Equal(t, 1, len(addedTaskSpecs[rs2]))
	assert.True(t, addedTaskSpecs[rs2][specs_testutils.PerfTask])
}

func TestTaskCfgCacheCleanup(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	assert.NoError(t, repos.Update(ctx))
	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)

	// Load configs into the cache.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	assert.NoError(t, cache.Set(ctx, rs1, baseTasksCfg, nil))
	assert.NoError(t, cache.Set(ctx, rs2, nextTasksCfg, nil))
	_, err = cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, cache.cache.Len())
	_, err = cache.GetAddedTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.addedTasksCache))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := git.NewRepo(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	d1, err := r.Details(ctx, c1)
	assert.NoError(t, err)
	d2, err := r.Details(ctx, c2)
	diff := d2.Timestamp.Sub(d1.Timestamp)
	now := time.Now()
	period := now.Sub(d2.Timestamp) + (diff / 2)
	assert.NoError(t, cache.Cleanup(ctx, period))
	assert.Equal(t, 1, cache.cache.Len())
	assert.Equal(t, 1, len(cache.addedTasksCache))
}

func TestTaskCfgCacheError(t *testing.T) {
	testutils.LargeTest(t)

	// Verify that we properly cache merge errors.
	ctx, gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(ctx, gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	assert.NoError(t, repos.Update(ctx))
	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)

	// Load configs into the cache.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	assert.NoError(t, cache.Set(ctx, rs1, baseTasksCfg, nil))
	assert.NoError(t, cache.Set(ctx, rs2, nextTasksCfg, nil))
	_, err = cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, cache.cache.Len())

	rs3 := types.RepoState{
		Repo:     rs1.Repo,
		Revision: rs1.Revision,
		Patch: types.Patch{
			Server:   "my-server",
			Issue:    "12345",
			Patchset: "1",
		},
	}
	repoStates := []types.RepoState{rs3}

	// This is a permanent error. It shouldn't be returned from
	// GetTaskSpecsForRepoStates, since that would block scheduling
	// permanently.
	storedErr := errors.New("error: Failed to merge in the changes.; Stdout+Stderr:\n")
	assert.NoError(t, cache.Set(ctx, rs3, nil, storedErr))
	_, err = cache.GetTaskSpecsForRepoStates(ctx, repoStates)
	assert.NoError(t, err)
	_, err = cache.Get(ctx, rs3)
	assert.EqualError(t, err, storedErr.Error())

	// Create a new cache, assert that we get the same error.
	cache2, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)
	_, err = cache2.GetTaskSpecsForRepoStates(ctx, repoStates)
	assert.NoError(t, err)
	_, err = cache2.Get(ctx, rs3)
	assert.EqualError(t, err, storedErr.Error())
}

// makeTasksCfg generates a JSON representation of a TasksCfg based on the given
// tasks and jobs.
func makeTasksCfg(t *testing.T, tasks, jobs map[string][]string) string {
	taskSpecs := make(map[string]*TaskSpec, len(tasks))
	for name, deps := range tasks {
		taskSpecs[name] = &TaskSpec{
			CipdPackages: []*CipdPackage{},
			Dependencies: deps,
			Dimensions:   []string{},
			Isolate:      "abc123",
			Priority:     0.0,
		}
	}
	jobSpecs := make(map[string]*JobSpec, len(jobs))
	for name, deps := range jobs {
		jobSpecs[name] = &JobSpec{
			TaskSpecs: deps,
		}
	}
	cfg := TasksCfg{
		Tasks: taskSpecs,
		Jobs:  jobSpecs,
	}
	return testutils.MarshalIndentJSON(t, &cfg)
}

func TestTasksCircularDependency(t *testing.T) {
	testutils.SmallTest(t)
	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{}, map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"b\" and \"a\"")

	// Longer cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"d"},
		"d": {"e"},
		"e": {"f"},
		"f": {"g"},
		"g": {"h"},
		"h": {"i"},
		"i": {"j"},
		"j": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"j\" and \"a\"")

	// No false positive on a complex-ish graph.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"d", "e", "f"},
	}, map[string][]string{
		"j": {"a", "g"},
	}))
	assert.NoError(t, err)

	// Unreachable task (d)
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"e", "f"},
	}, map[string][]string{
		"j": {"g"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Task \"d\" is not reachable by any Job!")

	// Dependency on unknown task.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"e", "f"},
	}, map[string][]string{
		"j": {"q"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Job \"j\" has unknown task \"q\" as a dependency.")
}

func TestGetTaskSpecDAG(t *testing.T) {
	testutils.SmallTest(t)
	test := func(dag map[string][]string, jobDeps []string) {
		cfg, err := ParseTasksCfg(makeTasksCfg(t, dag, map[string][]string{
			"j": jobDeps,
		}))
		assert.NoError(t, err)
		j, ok := cfg.Jobs["j"]
		assert.True(t, ok)
		res, err := j.GetTaskSpecDAG(cfg)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, res, dag)
	}

	test(map[string][]string{"a": {}}, []string{"a"})

	test(map[string][]string{
		"a": {"b"},
		"b": {},
	}, []string{"a"})

	test(map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"d", "e", "f"},
	}, []string{"a", "g"})
}

func TestTaskCfgCacheStorage(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, r1, _ := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)
	assert.NoError(t, repos.Update(ctx))

	botUpdateCount := 0
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(cmd *exec.Command) error {
		for _, arg := range cmd.Args {
			if strings.Contains(arg, "bot_update") {
				botUpdateCount++
				break
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx = exec.NewContext(ctx, mock.Run)

	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	c, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
	assert.NoError(t, err)

	check := func(rs ...types.RepoState) {
		c2, err := NewTaskCfgCache(ctx, repos, project, instance, nil)
		assert.NoError(t, err)
		expectBotUpdateCount := botUpdateCount
		for _, r := range rs {
			cfg, err := c2.Get(ctx, r)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
		}
		// Assert that we obtained the TasksCfg from BigTable and not by
		// running bot_update.
		assert.Equal(t, expectBotUpdateCount, botUpdateCount)

		// Verify that the caches are updated as expected.
		c.mtx.Lock()
		defer c.mtx.Unlock()
		c2.mtx.Lock()
		defer c2.mtx.Unlock()
		assert.Equal(t, c.cache.Len(), c2.cache.Len())
		c.cache.ForEach(ctx, func(ctx context.Context, key string, value1 set_if_unset_cache.Value) {
			value2, err := c2.cache.Get(ctx, key)
			assert.NoError(t, err)
			v1 := value1.(*CachedValue)
			v2 := value2.(*CachedValue)
			assert.Equal(t, v1.Err, v2.Err)
			deepequal.AssertDeepEqual(t, v1.Cfg, v2.Cfg)
			deepequal.AssertDeepEqual(t, v1.RepoState, v2.RepoState)
		})
		deepequal.AssertDeepEqual(t, c.addedTasksCache, c2.addedTasksCache)
		deepequal.AssertDeepEqual(t, c.recentCommits, c2.recentCommits)
		deepequal.AssertDeepEqual(t, c.recentJobSpecs, c2.recentJobSpecs)
		deepequal.AssertDeepEqual(t, c.recentTaskSpecs, c2.recentTaskSpecs)
	}

	// Empty cache.
	check()

	// No entries.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r1,
	}
	cfg, err := c.Get(ctx, rs1)
	assert.Equal(t, set_if_unset_cache.ErrNoSuchEntry, err)
	assert.Nil(t, cfg)
	assert.Equal(t, 0, c.cache.Len())
	taskSpecs, err := c.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(taskSpecs))

	// One entry.
	assert.NoError(t, c.Set(ctx, rs1, baseTasksCfg, nil))
	check(rs1)

	// Cleanup() the cache to remove the entries.
	assert.NoError(t, c.Cleanup(ctx, time.Duration(0)))
	assert.Equal(t, 0, c.cache.Len())
	check()

	// Add two commits with identical tasks.json hash and check serialization.
	r3 := gb.CommitGen(ctx, "otherfile.txt")
	rs3 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r3,
	}
	r4 := gb.CommitGen(ctx, "otherfile.txt")
	rs4 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r4,
	}
	assert.NoError(t, repos.Update(ctx))
	assert.NoError(t, c.Set(ctx, rs3, nextTasksCfg, nil))
	assert.NoError(t, c.Set(ctx, rs4, nextTasksCfg, nil))
	_, err = c.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs3, rs4})
	assert.NoError(t, err)
	assert.Equal(t, 2, c.cache.Len())
	check(rs3, rs4)
}
