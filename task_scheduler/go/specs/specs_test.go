package specs

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	specs_testutils "go.skia.org/infra/task_scheduler/go/specs/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	// Use this as an expected error when you don't care about the actual
	// error which is returned.
	ERR_DONT_CARE = fmt.Errorf("DONTCARE")
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
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	specs, err := cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name := range byName {
			sklog.Infof("%s %s", rs, name)
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
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)

	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}

	addedTaskSpecs, err := cache.GetAddedTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(addedTaskSpecs[rs1]))
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.BuildTask])
	assert.True(t, addedTaskSpecs[rs1][specs_testutils.TestTask])
	assert.Equal(t, 1, len(addedTaskSpecs[rs2]))
	assert.True(t, addedTaskSpecs[rs2][specs_testutils.PerfTask])

	// c3 adds Beer and Belch (names chosen to avoid merge conficts)
	gb.CreateBranchTrackBranch(ctx, "branchy-mcbranch-face", "master")
	cfg, err := ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	cfg.Jobs["Beer"] = &JobSpec{TaskSpecs: []string{"Belch"}}
	cfg.Tasks["Beer"] = &TaskSpec{
		Dependencies: []string{specs_testutils.BuildTask},
		Isolate:      "swarm_recipe.isolate",
	}
	cfg.Tasks["Belch"] = &TaskSpec{
		Dependencies: []string{"Beer"},
		Isolate:      "swarm_recipe.isolate",
	}
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg))
	c3 := gb.Commit(ctx)

	// c4 removes Perf
	gb.CheckoutBranch(ctx, "master")
	cfg, err = ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	delete(cfg.Jobs, specs_testutils.PerfTask)
	delete(cfg.Tasks, specs_testutils.PerfTask)
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg))
	c4 := gb.Commit(ctx)

	// c5 merges c3 and c4
	c5 := gb.MergeBranch(ctx, "branchy-mcbranch-face")

	// c6 adds back Perf
	cfg, err = ReadTasksCfg(gb.Dir())
	assert.NoError(t, err)
	cfg.Jobs[specs_testutils.PerfTask] = &JobSpec{TaskSpecs: []string{specs_testutils.PerfTask}}
	cfg.Tasks[specs_testutils.PerfTask] = &TaskSpec{
		Dependencies: []string{specs_testutils.BuildTask},
		Isolate:      "swarm_recipe.isolate",
	}
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, cfg))
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
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), path.Join(tmp, "cache"), DEFAULT_NUM_WORKERS, project, instance, nil)
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
	_, err = cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))
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
	assert.NoError(t, cache.Cleanup(period))
	assert.Equal(t, 1, len(cache.cache))
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
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), path.Join(tmp, "cache"), DEFAULT_NUM_WORKERS, project, instance, nil)
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
	_, err = cache.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))

	botUpdateCount := 0
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(cmd *exec.Command) error {
		for _, arg := range cmd.Args {
			if strings.Contains(arg, "bot_update") {
				botUpdateCount++
				return errors.New("error: Failed to merge in the changes.")
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx = exec.NewContext(ctx, mock.Run)
	repoStates := []types.RepoState{
		types.RepoState{
			Repo:     rs1.Repo,
			Revision: rs1.Revision,
			Patch: types.Patch{
				Server:   "my-server",
				Issue:    "12345",
				Patchset: "1",
			},
		},
	}
	_, err = cache.GetTaskSpecsForRepoStates(ctx, repoStates)
	assert.EqualError(t, err, "Errors loading task cfgs: [error: Failed to merge in the changes.; Stdout+Stderr:\n]")
	assert.Equal(t, 1, botUpdateCount)

	// Try again, assert that we didn't run bot_update again.
	_, err = cache.GetTaskSpecsForRepoStates(ctx, repoStates)
	assert.EqualError(t, err, "Errors loading task cfgs: [error: Failed to merge in the changes.; Stdout+Stderr:\n]")
	assert.Equal(t, 1, botUpdateCount)

	// Create a new cache, assert that it doesn't run bot_update.
	cache2, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), path.Join(tmp, "cache2"), DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)
	_, err = cache2.GetTaskSpecsForRepoStates(ctx, repoStates)
	assert.EqualError(t, err, "Errors loading task cfgs: [error: Failed to merge in the changes.; Stdout+Stderr:\n]")
	assert.Equal(t, 1, botUpdateCount)
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
	assert.EqualError(t, err, "Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{}, map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"b\" and \"a\"")

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
	assert.EqualError(t, err, "Found a circular dependency involving \"j\" and \"a\"")

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
	assert.EqualError(t, err, "Task \"d\" is not reachable by any Job!")

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
	assert.EqualError(t, err, "Job \"j\" has unknown task \"q\" as a dependency.")
}

func tempGitRepoSetup(t *testing.T) (context.Context, *git_testutils.GitBuilder, string, string) {
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	gb.Add(ctx, "codereview.settings", `CODE_REVIEW_SERVER: codereview.chromium.org
PROJECT: skia`)
	c1 := gb.CommitMsg(ctx, "initial commit")
	c2 := gb.CommitGen(ctx, "somefile")
	return ctx, gb, c1, c2
}

func tempGitRepoBotUpdateTests(t *testing.T, cases map[types.RepoState]error) {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	ctx := context.Background()
	cacheDir := path.Join(tmp, "cache")
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	for rs, expectErr := range cases {
		c, err := tempGitRepoBotUpdate(ctx, rs, depotTools, cacheDir, tmp)
		if expectErr != nil {
			assert.Error(t, err)
			if expectErr != ERR_DONT_CARE {
				assert.EqualError(t, err, expectErr.Error())
			}
		} else {
			defer c.Delete()
			assert.NoError(t, err)
			output, err := c.Git(ctx, "remote", "-v")
			gotRepo := "COULD NOT FIND REPO"
			for _, s := range strings.Split(output, "\n") {
				if strings.HasPrefix(s, "origin") {
					split := strings.Fields(s)
					assert.Equal(t, 3, len(split))
					gotRepo = split[1]
					break
				}
			}
			assert.Equal(t, rs.Repo, gotRepo)
			gotRevision, err := c.RevParse(ctx, "HEAD")
			assert.NoError(t, err)
			assert.Equal(t, rs.Revision, gotRevision)
			// If not a try job, we expect a clean checkout,
			// otherwise we expect a dirty checkout, from the
			// applied patch.
			_, err = c.Git(ctx, "diff", "--exit-code", "--no-patch", rs.Revision)
			if rs.IsTryJob() {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestTempGitRepo(t *testing.T) {
	testutils.LargeTest(t)
	_, gb, c1, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	cases := map[types.RepoState]error{
		{
			Repo:     gb.RepoUrl(),
			Revision: c1,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: "bogusRev",
		}: ERR_DONT_CARE,
	}
	tempGitRepoBotUpdateTests(t, cases)
}

func TestTempGitRepoPatch(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, _, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	issue := "12345"
	patchset := "3"
	gb.CreateFakeGerritCLGen(ctx, issue, patchset)

	cases := map[types.RepoState]error{
		{
			Patch: types.Patch{
				Server:   gb.RepoUrl(),
				Issue:    issue,
				Patchset: patchset,
			},
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
	}
	tempGitRepoBotUpdateTests(t, cases)
}

func TestTempGitRepoParallel(t *testing.T) {
	testutils.LargeTest(t)

	ctx, gb, c1, _ := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)

	rs := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, cache.TempGitRepo(ctx, rs, true, func(g *git.TempCheckout) error {
				return nil
			}))
		}()
	}
	wg.Wait()
}

func TestTempGitRepoErr(t *testing.T) {
	// bot_update uses lots of retries with exponential backoff, which makes
	// this really slow.
	t.Skip("TestTempGitRepoErr is very slow.")
	testutils.LargeTest(t)

	ctx, gb, c1, _ := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewMap(ctx, []string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	project, instance, cleanup := specs_testutils.SetupBigTable(t)
	defer cleanup()
	cache, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)

	// bot_update will fail to apply the issue if we don't fake it in Git.
	rs := types.RepoState{
		Patch: types.Patch{
			Issue:    "my-issue",
			Patchset: "my-patchset",
			Server:   "my-server",
		},
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	assert.Error(t, cache.TempGitRepo(ctx, rs, true, func(c *git.TempCheckout) error {
		// This may fail with a nil pointer dereference due to a nil
		// git.TempCheckout.
		assert.FailNow(t, "We should not have gotten here.")
		return nil
	}))
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
	c, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
	assert.NoError(t, err)

	check := func(rs ...types.RepoState) {
		c2, err := NewTaskCfgCache(ctx, repos, depot_tools_testutils.GetDepotTools(t, ctx), tmp, DEFAULT_NUM_WORKERS, project, instance, nil)
		assert.NoError(t, err)
		expectBotUpdateCount := botUpdateCount
		for _, r := range rs {
			_, err := c2.ReadTasksCfg(ctx, r)
			assert.NoError(t, err)
		}
		// Assert that we obtained the TasksCfg from BigTable and not by
		// running bot_update.
		assert.Equal(t, expectBotUpdateCount, botUpdateCount)

		// Verify that the caches are updated as expected.
		c.mtx.Lock()
		defer c.mtx.Unlock()
		c2.mtx.Lock()
		defer c2.mtx.Unlock()
		assert.Equal(t, len(c.cache), len(c2.cache))
		for k, v := range c.cache {
			v2, ok := c2.cache[k]
			assert.True(t, ok)
			assert.Equal(t, v.err, v2.err)
			deepequal.AssertDeepEqual(t, v.cfg, v2.cfg)
			deepequal.AssertDeepEqual(t, v.rs, v2.rs)
		}
		deepequal.AssertDeepEqual(t, c.addedTasksCache, c2.addedTasksCache)
		deepequal.AssertDeepEqual(t, c.recentCommits, c2.recentCommits)
		deepequal.AssertDeepEqual(t, c.recentJobSpecs, c2.recentJobSpecs)
		deepequal.AssertDeepEqual(t, c.recentTaskSpecs, c2.recentTaskSpecs)
	}

	// Empty cache.
	check()

	// Insert one commit's worth of specs into the cache.
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: r1,
	}
	_, err = c.ReadTasksCfg(ctx, rs1)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c.cache))
	check(rs1)

	// Cleanup() the cache to remove the entries.
	assert.NoError(t, c.Cleanup(time.Duration(0)))
	assert.Equal(t, 0, len(c.cache))
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
	_, err = c.GetTaskSpecsForRepoStates(ctx, []types.RepoState{rs3, rs4})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c.cache))
	check(rs3, rs4)
}
